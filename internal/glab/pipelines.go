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
// ordered by most recently created.
func (c *Client) FetchPipelinesByUser(ctx context.Context, projectID int, username string, updatedAfter time.Time) ([]gitlab.Pipeline, error) {
	endpoint := fmt.Sprintf("projects/%d/pipelines?order_by=id&sort=desc&per_page=20&updated_after=%s",
		projectID, url.QueryEscape(updatedAfter.Format(time.RFC3339)))
	if username != "" {
		endpoint += "&username=" + url.QueryEscape(username)
	}
	return c.fetchPipelinesPaginated(ctx, endpoint)
}

// FetchPipelines fetches pipelines for a project, ordered by most recently created.
func (c *Client) FetchPipelines(ctx context.Context, projectID int, limit int) ([]gitlab.Pipeline, error) {
	if limit <= 0 {
		limit = 100
	}
	perPage := limit
	if perPage > 100 {
		perPage = 100
	}
	endpoint := fmt.Sprintf("projects/%d/pipelines?order_by=id&sort=desc&per_page=%d", projectID, perPage)
	return c.fetchPipelinesLimited(ctx, endpoint, limit, perPage)
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

// FetchProjectJobs fetches jobs for a project, ordered by most recently created,
// stopping once a job older than cutoff is seen. Fetching is capped at limit as
// a safety net against unbounded pagination.
func (c *Client) FetchProjectJobs(ctx context.Context, projectID int, cutoff time.Time, limit int) ([]gitlab.Job, error) {
	return c.fetchJobsUntil(ctx, projectID, limit, func(j gitlab.Job) bool {
		return j.CreatedAt.Before(cutoff)
	})
}

// FetchProjectJobsSince fetches jobs for a project created after sinceID,
// ordered by most recently created, stopping once a job with ID <= sinceID
// is seen. Fetching is capped at limit as a safety net against unbounded
// pagination. Pass sinceID 0 to fetch the most recent limit jobs.
func (c *Client) FetchProjectJobsSince(ctx context.Context, projectID int, sinceID int, limit int) ([]gitlab.Job, error) {
	return c.fetchJobsUntil(ctx, projectID, limit, func(j gitlab.Job) bool {
		return j.ID <= sinceID
	})
}

// FetchJob fetches a single job by ID.
func (c *Client) FetchJob(ctx context.Context, projectID int, jobID int) (gitlab.Job, error) {
	endpoint := fmt.Sprintf("projects/%d/jobs/%d", projectID, jobID)
	out, err := c.Run(ctx, "api", endpoint)
	if err != nil {
		return gitlab.Job{}, fmt.Errorf("fetching job: %w", err)
	}
	var j gitlab.Job
	if err := json.Unmarshal(out, &j); err != nil {
		return gitlab.Job{}, fmt.Errorf("parsing job response: %w", err)
	}
	return j, nil
}

// fetchJobsUntil paginates a project's jobs newest-first, stopping once stop
// returns true for a job (that job and any after it are excluded), or a page
// comes back short. Fetching is capped at limit.
func (c *Client) fetchJobsUntil(ctx context.Context, projectID int, limit int, stop func(gitlab.Job) bool) ([]gitlab.Job, error) {
	if limit <= 0 {
		limit = 100
	}
	perPage := limit
	if perPage > 100 {
		perPage = 100
	}
	endpoint := fmt.Sprintf("projects/%d/jobs?order_by=id&sort=desc&per_page=%d", projectID, perPage)

	var all []gitlab.Job
	for page := 1; len(all) < limit; page++ {
		jobs, err := c.fetchJobsPage(ctx, endpoint, page)
		if err != nil {
			return nil, err
		}
		if len(jobs) == 0 {
			break
		}

		reachedBoundary := false
		for _, j := range jobs {
			if stop(j) {
				reachedBoundary = true
				break
			}
			all = append(all, j)
			if len(all) >= limit {
				break
			}
		}
		if reachedBoundary || len(jobs) < perPage {
			break
		}
	}
	if len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

func (c *Client) fetchJobsPage(ctx context.Context, baseEndpoint string, page int) ([]gitlab.Job, error) {
	endpoint := baseEndpoint + "&page=" + strconv.Itoa(page)
	out, err := c.Run(ctx, "api", endpoint)
	if err != nil {
		return nil, fmt.Errorf("fetching jobs (page %d): %w", page, err)
	}

	var jobs []gitlab.Job
	if err := json.Unmarshal(out, &jobs); err != nil {
		return nil, fmt.Errorf("parsing jobs response (page %d): %w", page, err)
	}
	return jobs, nil
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
		pipelines, err := c.fetchPipelinesPage(ctx, baseEndpoint, page)
		if err != nil {
			return nil, err
		}

		all = append(all, pipelines...)
		if len(pipelines) < perPage {
			break
		}
	}
	return all, nil
}

func (c *Client) fetchPipelinesLimited(ctx context.Context, baseEndpoint string, limit int, perPage int) ([]gitlab.Pipeline, error) {
	var all []gitlab.Pipeline
	for page := 1; len(all) < limit; page++ {
		pipelines, err := c.fetchPipelinesPage(ctx, baseEndpoint, page)
		if err != nil {
			return nil, err
		}

		all = append(all, pipelines...)
		if len(pipelines) < perPage {
			break
		}
	}
	if len(all) > limit {
		return all[:limit], nil
	}
	return all, nil
}

func (c *Client) fetchPipelinesPage(ctx context.Context, baseEndpoint string, page int) ([]gitlab.Pipeline, error) {
	endpoint := baseEndpoint + "&page=" + strconv.Itoa(page)
	out, err := c.Run(ctx, "api", endpoint)
	if err != nil {
		return nil, fmt.Errorf("fetching pipelines (page %d): %w", page, err)
	}

	var pipelines []gitlab.Pipeline
	if err := json.Unmarshal(out, &pipelines); err != nil {
		return nil, fmt.Errorf("parsing pipelines response (page %d): %w", page, err)
	}
	return pipelines, nil
}
