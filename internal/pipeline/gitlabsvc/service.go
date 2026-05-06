package gitlabsvc

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/navitronic/gitlab-builds/internal/discovery"
	"github.com/navitronic/gitlab-builds/internal/gitlab"
	"github.com/navitronic/gitlab-builds/internal/glab"
	"github.com/navitronic/gitlab-builds/internal/pipeline"
)

type Service struct {
	client *glab.Client
}

func New(client *glab.Client) *Service {
	return &Service{client: client}
}

func (s *Service) ListPipelines(ctx context.Context, progress func(string)) ([]pipeline.Pipeline, error) {
	progress("fetching user...")
	user, err := s.client.CurrentUser(ctx)
	if err != nil {
		return nil, wrapErr(err)
	}

	progress("discovering activity...")
	disc := discovery.New(s.client)
	candidates, err := disc.Discover(ctx, user.ID, 3, 10)
	if err != nil {
		return nil, wrapErr(err)
	}

	progress(fmt.Sprintf("fetching pipelines (%d projects)...", len(candidates)))

	type result struct {
		pipelines []pipeline.Pipeline
		err       error
	}
	ch := make(chan result, len(candidates))

	for _, c := range candidates {
		go func(c gitlab.PipelineCandidate) {
			pipelines, err := s.client.FetchPipelinesBySHA(ctx, c.ProjectID, user.ID, c.SHA)
			if err != nil {
				ch <- result{err: wrapErr(err)}
				return
			}
			if len(pipelines) == 0 {
				pipelines, err = s.client.FetchPipelinesByRef(ctx, c.ProjectID, user.ID, c.Ref)
				if err != nil {
					ch <- result{err: wrapErr(err)}
					return
				}
			}
			if len(pipelines) == 0 {
				pipelines, err = s.client.FetchPipelinesFallback(ctx, c.ProjectID, user.ID)
				if err != nil {
					ch <- result{err: wrapErr(err)}
					return
				}
			}
			var out []pipeline.Pipeline
			for _, p := range pipelines {
				out = append(out, convertPipeline(p, c.ProjectID, c.ProjectPath))
			}
			ch <- result{pipelines: out}
		}(c)
	}

	var all []pipeline.Pipeline
	var lastErr error
	for range candidates {
		r := <-ch
		if r.err != nil {
			lastErr = r.err
			continue
		}
		all = append(all, r.pipelines...)
	}
	if len(all) == 0 && lastErr != nil {
		return nil, fmt.Errorf("fetching pipelines: %w", lastErr)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].UpdatedAt.After(all[j].UpdatedAt)
	})
	return all, nil
}

func (s *Service) GetPipeline(ctx context.Context, project string, id string) (pipeline.Pipeline, error) {
	projectID, err := strconv.Atoi(project)
	if err != nil {
		return pipeline.Pipeline{}, fmt.Errorf("invalid project ID: %w", err)
	}
	pipelineID, err := strconv.Atoi(id)
	if err != nil {
		return pipeline.Pipeline{}, fmt.Errorf("invalid pipeline ID: %w", err)
	}
	p, err := s.client.FetchPipeline(ctx, projectID, pipelineID)
	if err != nil {
		return pipeline.Pipeline{}, wrapErr(err)
	}
	return convertPipeline(p, projectID, ""), nil
}

func (s *Service) ListJobs(ctx context.Context, project string, pipelineID string) ([]pipeline.Job, error) {
	projID, err := strconv.Atoi(project)
	if err != nil {
		return nil, fmt.Errorf("invalid project ID: %w", err)
	}
	pipID, err := strconv.Atoi(pipelineID)
	if err != nil {
		return nil, fmt.Errorf("invalid pipeline ID: %w", err)
	}
	jobs, err := s.client.FetchPipelineJobs(ctx, projID, pipID)
	if err != nil {
		return nil, wrapErr(err)
	}
	out := make([]pipeline.Job, len(jobs))
	for i, j := range jobs {
		out[i] = convertJob(j)
	}
	return out, nil
}

func (s *Service) GetMergeRequestURL(ctx context.Context, project string, ref string) (string, error) {
	projectID, err := strconv.Atoi(project)
	if err != nil {
		return "", fmt.Errorf("invalid project ID: %w", err)
	}
	mr, err := s.client.FetchMergeRequestByBranch(ctx, projectID, ref)
	if err != nil {
		return "", wrapErr(err)
	}
	return mr.WebURL, nil
}

func convertPipeline(p gitlab.Pipeline, projectID int, project string) pipeline.Pipeline {
	return pipeline.Pipeline{
		ID:        strconv.Itoa(p.ID),
		ProjectID: strconv.Itoa(projectID),
		Project:   project,
		Ref:       p.Ref,
		SHA:       p.SHA,
		Status:    convertStatus(p.Status),
		Source:    p.Source,
		WebURL:    p.WebURL,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
		Duration:  time.Duration(p.Duration * float64(time.Second)),
	}
}

func convertJob(j gitlab.Job) pipeline.Job {
	return pipeline.Job{
		ID:        strconv.Itoa(j.ID),
		Name:      j.Name,
		Stage:     j.Stage,
		Status:    convertStatus(j.Status),
		WebURL:    j.WebURL,
		CreatedAt: j.CreatedAt,
		StartedAt: j.StartedAt,
		Duration:  time.Duration(j.Duration * float64(time.Second)),
	}
}

func convertStatus(s string) pipeline.Status {
	switch s {
	case "success":
		return pipeline.StatusPassed
	case "failed":
		return pipeline.StatusFailed
	case "running":
		return pipeline.StatusRunning
	case "pending":
		return pipeline.StatusPending
	case "canceled":
		return pipeline.StatusCanceled
	case "skipped":
		return pipeline.StatusSkipped
	default:
		return pipeline.StatusPending
	}
}

func wrapErr(err error) error {
	if errors.Is(err, glab.ErrGlabNotFound) {
		return fmt.Errorf("%w: %w", pipeline.ErrClientNotFound, err)
	}
	if errors.Is(err, glab.ErrAuthRequired) {
		return fmt.Errorf("%w: %w", pipeline.ErrAuthRequired, err)
	}
	return err
}
