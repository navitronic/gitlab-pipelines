package glab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/navitronic/gitlab-pipelines/internal/gitlab"
)

// FetchPipelinesByUser fetches recent pipelines for a project belonging to a user,
// ordered by most recently created. Filters both server-side (via the username query
// parameter) and client-side (in case the server ignores the parameter).
func (c *Client) FetchPipelinesByUser(ctx context.Context, projectID int, username string, updatedAfter time.Time) ([]gitlab.Pipeline, error) {
	endpoint := fmt.Sprintf("projects/%d/pipelines?order_by=id&sort=desc&per_page=20&updated_after=%s",
		projectID, url.QueryEscape(updatedAfter.Format(time.RFC3339)))
	if username != "" {
		endpoint += "&username=" + url.QueryEscape(username)
	}
	pipelines, err := c.fetchPipelinesPaginated(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	if username == "" {
		return pipelines, nil
	}
	filtered := pipelines[:0]
	for _, p := range pipelines {
		if p.User != nil && p.User.Username == username {
			filtered = append(filtered, p)
		}
	}
	return filtered, nil
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

// FetchUserMergeRequests fetches merge requests authored by the user that are
// either currently open (any age) or merged after the given time.
func (c *Client) FetchUserMergeRequests(ctx context.Context, mergedAfter time.Time) ([]gitlab.MergeRequest, error) {
	openEndpoint := "merge_requests?scope=created_by_me&state=opened&per_page=100&order_by=updated_at&sort=desc"
	openMRs, err := c.fetchMergeRequestsPaginated(ctx, openEndpoint)
	if err != nil {
		return nil, fmt.Errorf("fetching opened merge requests: %w", err)
	}

	mergedEndpoint := fmt.Sprintf("merge_requests?scope=created_by_me&state=merged&updated_after=%s&per_page=100&order_by=updated_at&sort=desc",
		url.QueryEscape(mergedAfter.Format(time.RFC3339)))
	mergedMRs, err := c.fetchMergeRequestsPaginated(ctx, mergedEndpoint)
	if err != nil {
		return nil, fmt.Errorf("fetching merged merge requests: %w", err)
	}

	return append(openMRs, mergedMRs...), nil
}

func (c *Client) fetchMergeRequestsPaginated(ctx context.Context, baseEndpoint string) ([]gitlab.MergeRequest, error) {
	const perPage = 100
	var all []gitlab.MergeRequest
	for page := 1; ; page++ {
		endpoint := baseEndpoint + "&page=" + strconv.Itoa(page)
		out, err := c.Run(ctx, "api", endpoint)
		if err != nil {
			return nil, fmt.Errorf("fetching merge requests (page %d): %w", page, err)
		}

		var mrs []gitlab.MergeRequest
		if err := json.Unmarshal(out, &mrs); err != nil {
			return nil, fmt.Errorf("parsing merge requests response (page %d): %w", page, err)
		}

		all = append(all, mrs...)
		if len(mrs) < perPage {
			break
		}
	}
	return all, nil
}

func (c *Client) FetchMergeRequestByBranch(ctx context.Context, projectID int, branch string) (gitlab.MergeRequest, error) {
	endpoint := fmt.Sprintf("projects/%d/merge_requests?source_branch=%s&state=all&order_by=updated_at&sort=desc&per_page=1", projectID, url.QueryEscape(branch))
	out, err := c.Run(ctx, "api", endpoint)
	if err != nil {
		return gitlab.MergeRequest{}, fmt.Errorf("fetching merge requests: %w", err)
	}
	var mrs []gitlab.MergeRequest
	if err := json.Unmarshal(out, &mrs); err != nil {
		return gitlab.MergeRequest{}, fmt.Errorf("parsing merge request response: %w", err)
	}
	if len(mrs) == 0 {
		return gitlab.MergeRequest{}, nil
	}
	return mrs[0], nil
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
