package glab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/navitronic/gitlab-builds/internal/gitlab"
)

// FetchPipelinesBySHA fetches pipelines for a project matching a specific commit SHA.
func (c *Client) FetchPipelinesBySHA(ctx context.Context, projectID int, userID int, sha string) ([]gitlab.Pipeline, error) {
	endpoint := fmt.Sprintf("projects/%d/pipelines?sha=%s&per_page=100", projectID, url.QueryEscape(sha))
	if userID > 0 {
		endpoint += "&user_id=" + strconv.Itoa(userID)
	}
	return c.fetchPipelinesPaginated(ctx, endpoint)
}

// FetchPipelinesByRef fetches pipelines for a project matching a specific ref.
func (c *Client) FetchPipelinesByRef(ctx context.Context, projectID int, userID int, ref string) ([]gitlab.Pipeline, error) {
	endpoint := fmt.Sprintf("projects/%d/pipelines?ref=%s&per_page=100", projectID, url.QueryEscape(ref))
	if userID > 0 {
		endpoint += "&user_id=" + strconv.Itoa(userID)
	}
	return c.fetchPipelinesPaginated(ctx, endpoint)
}

// FetchPipelinesFallback fetches recent pipelines for a project ordered by update time.
func (c *Client) FetchPipelinesFallback(ctx context.Context, projectID int, userID int) ([]gitlab.Pipeline, error) {
	endpoint := fmt.Sprintf("projects/%d/pipelines?order_by=updated_at&sort=desc&per_page=100", projectID)
	if userID > 0 {
		endpoint += "&user_id=" + strconv.Itoa(userID)
	}
	return c.fetchPipelinesPaginated(ctx, endpoint)
}

// FetchPipeline fetches a single pipeline by ID.
func (c *Client) FetchPipeline(ctx context.Context, projectID int, pipelineID int) (gitlab.Pipeline, error) {
	endpoint := fmt.Sprintf("projects/%d/pipelines/%d", projectID, pipelineID)
	out, err := c.Run(ctx, "api", endpoint)
	if err != nil {
		return gitlab.Pipeline{}, fmt.Errorf("fetching pipeline: %w", err)
	}
	var p gitlab.Pipeline
	if err := json.Unmarshal(out, &p); err != nil {
		return gitlab.Pipeline{}, fmt.Errorf("parsing pipeline response: %w", err)
	}
	return p, nil
}

// FetchPipelineJobs fetches jobs for a specific pipeline.
func (c *Client) FetchPipelineJobs(ctx context.Context, projectID int, pipelineID int) ([]gitlab.Job, error) {
	const perPage = 100
	var allJobs []gitlab.Job
	for page := 1; ; page++ {
		endpoint := fmt.Sprintf("projects/%d/pipelines/%d/jobs?per_page=%d&page=%d", projectID, pipelineID, perPage, page)
		out, err := c.Run(ctx, "api", endpoint)
		if err != nil {
			return nil, fmt.Errorf("fetching pipeline jobs (page %d): %w", page, err)
		}

		var jobs []gitlab.Job
		if err := json.Unmarshal(out, &jobs); err != nil {
			return nil, fmt.Errorf("parsing jobs response (page %d): %w", page, err)
		}

		allJobs = append(allJobs, jobs...)
		if len(jobs) < perPage {
			break
		}
	}
	return allJobs, nil
}

func (c *Client) fetchPipelinesPaginated(ctx context.Context, baseEndpoint string) ([]gitlab.Pipeline, error) {
	const perPage = 100
	var all []gitlab.Pipeline
	for page := 1; ; page++ {
		endpoint := baseEndpoint + "&page=" + strconv.Itoa(page)
		out, err := c.Run(ctx, "api", endpoint)
		if err != nil {
			return nil, fmt.Errorf("fetching pipelines (page %d): %w", page, err)
		}

		var pipelines []gitlab.Pipeline
		if err := json.Unmarshal(out, &pipelines); err != nil {
			return nil, fmt.Errorf("parsing pipelines response (page %d): %w", page, err)
		}

		all = append(all, pipelines...)
		if len(pipelines) < perPage {
			break
		}
	}
	return all, nil
}
