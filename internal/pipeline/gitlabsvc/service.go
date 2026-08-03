package gitlabsvc

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/navitronic/gitlab-pipelines/internal/activity"
	"github.com/navitronic/gitlab-pipelines/internal/discovery"
	"github.com/navitronic/gitlab-pipelines/internal/gitlab"
	"github.com/navitronic/gitlab-pipelines/internal/glab"
	"github.com/navitronic/gitlab-pipelines/internal/pipeline"
)

type GitLabClient interface {
	CurrentUser(ctx context.Context) (*gitlab.User, error)
	FetchUserEventsSince(ctx context.Context, userID int, after time.Time) ([]gitlab.Event, error)
	FetchProject(ctx context.Context, projectID int) (*gitlab.Project, error)
	FetchProjectByPath(ctx context.Context, projectPath string) (*gitlab.Project, error)
	FetchPipelines(ctx context.Context, projectID int, limit int) ([]gitlab.Pipeline, error)
	FetchPipelinesByUser(ctx context.Context, projectID int, username string, updatedAfter time.Time) ([]gitlab.Pipeline, error)
	FetchPipeline(ctx context.Context, projectID int, pipelineID int) (gitlab.Pipeline, error)
	FetchPipelineJobs(ctx context.Context, projectID int, pipelineID int) ([]gitlab.Job, error)
	FetchProjectJobs(ctx context.Context, projectID int, cutoff time.Time, limit int) ([]gitlab.Job, error)
	FetchMergeRequestByBranch(ctx context.Context, projectID int, branch string) (gitlab.MergeRequest, error)
	FetchUserMergeRequests(ctx context.Context, updatedAfter time.Time) ([]gitlab.MergeRequest, error)
}

type Service struct {
	client GitLabClient
}

func New(client *glab.Client) *Service {
	return &Service{client: client}
}

func NewWithClient(client GitLabClient) *Service {
	return &Service{client: client}
}

func (s *Service) ListPipelines(ctx context.Context, progress func(string)) ([]pipeline.Pipeline, error) {
	progress("fetching user...")
	user, err := s.client.CurrentUser(ctx)
	if err != nil {
		return nil, wrapErr(err)
	}

	store, err := activity.Load()
	if err != nil {
		store = &activity.Store{}
	}

	progress("fetching activity...")
	since := store.SinceTime()
	events, err := s.client.FetchUserEventsSince(ctx, user.ID, since)
	if err != nil {
		return nil, wrapErr(fmt.Errorf("discovering activity: %w", err))
	}

	repos := discovery.ExtractActiveRepos(events)

	store.Merge(events)
	_ = store.Save()

	repos = discovery.ExtractActiveRepos(store.Events)

	progress("fetching merge requests...")
	mrs, err := s.client.FetchUserMergeRequests(ctx, time.Now().Add(-24*time.Hour))
	if err == nil {
		repos = mergeReposFromMRs(repos, mrs)
	}

	pathCache := make(map[int]string)
	for _, r := range repos {
		if r.ProjectPath != "" {
			pathCache[r.ProjectID] = r.ProjectPath
		}
	}
	for i, r := range repos {
		if r.ProjectPath != "" {
			continue
		}
		if path, ok := pathCache[r.ProjectID]; ok {
			repos[i].ProjectPath = path
			continue
		}
		project, err := s.client.FetchProject(ctx, r.ProjectID)
		if err == nil {
			repos[i].ProjectPath = project.PathWithNamespace
			pathCache[r.ProjectID] = project.PathWithNamespace
		}
	}

	if len(repos) == 0 {
		return nil, nil
	}

	progress(fmt.Sprintf("fetching pipelines (%d projects)...", len(repos)))

	updatedAfter := time.Now().Add(-24 * time.Hour)

	type result struct {
		pipelines []pipeline.Pipeline
		err       error
	}
	ch := make(chan result, len(repos))

	for _, r := range repos {
		go func(r discovery.ActiveRepo) {
			pipelines, err := s.client.FetchPipelinesByUser(ctx, r.ProjectID, user.Username, updatedAfter)
			if err != nil {
				ch <- result{err: wrapErr(err)}
				return
			}
			var out []pipeline.Pipeline
			for _, p := range pipelines {
				out = append(out, convertPipeline(p, r.ProjectID, r.ProjectPath))
			}
			ch <- result{pipelines: out}
		}(r)
	}

	var all []pipeline.Pipeline
	var lastErr error
	for range repos {
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
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})
	return all, nil
}

func (s *Service) ListProjectPipelines(ctx context.Context, projectPath string, limit int, progress func(string)) ([]pipeline.Pipeline, error) {
	progress("fetching project...")
	project, err := s.client.FetchProjectByPath(ctx, projectPath)
	if err != nil {
		return nil, wrapErr(err)
	}

	progress("fetching pipelines...")
	pipelines, err := s.client.FetchPipelines(ctx, project.ID, limit)
	if err != nil {
		return nil, wrapErr(err)
	}

	out := make([]pipeline.Pipeline, len(pipelines))
	for i, p := range pipelines {
		out[i] = convertPipeline(p, project.ID, project.PathWithNamespace)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

// ListProjectJobsToday returns jobs created so far today (local calendar day)
// for the given project. If stages is non-empty, only jobs in those stages
// are returned; the GitLab jobs API has no server-side stage filter, so this
// is applied after fetching (and after the limit cutoff).
func (s *Service) ListProjectJobsToday(ctx context.Context, projectPath string, limit int, stages []string, progress func(string)) ([]pipeline.Job, error) {
	progress("fetching project...")
	project, err := s.client.FetchProjectByPath(ctx, projectPath)
	if err != nil {
		return nil, wrapErr(err)
	}

	progress("fetching jobs...")
	jobs, err := s.client.FetchProjectJobs(ctx, project.ID, startOfDay(time.Now()), limit)
	if err != nil {
		return nil, wrapErr(err)
	}

	stageSet := make(map[string]struct{}, len(stages))
	for _, s := range stages {
		stageSet[s] = struct{}{}
	}

	out := make([]pipeline.Job, 0, len(jobs))
	for _, j := range jobs {
		if len(stageSet) > 0 {
			if _, ok := stageSet[j.Stage]; !ok {
				continue
			}
		}
		out = append(out, convertJob(j))
	}
	return out, nil
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
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
		ID:           strconv.Itoa(j.ID),
		Name:         j.Name,
		Stage:        j.Stage,
		Status:       convertStatus(j.Status),
		AllowFailure: j.AllowFailure,
		WebURL:       j.WebURL,
		CreatedAt:    j.CreatedAt,
		StartedAt:    j.StartedAt,
		Duration:     time.Duration(j.Duration * float64(time.Second)),
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

func mergeReposFromMRs(repos []discovery.ActiveRepo, mrs []gitlab.MergeRequest) []discovery.ActiveRepo {
	seen := make(map[int]struct{}, len(repos))
	for _, r := range repos {
		seen[r.ProjectID] = struct{}{}
	}
	for _, mr := range mrs {
		if mr.ProjectID == 0 {
			continue
		}
		if _, ok := seen[mr.ProjectID]; ok {
			continue
		}
		seen[mr.ProjectID] = struct{}{}
		repos = append(repos, discovery.ActiveRepo{
			ProjectID:  mr.ProjectID,
			LastActive: mr.UpdatedAt,
		})
	}
	return repos
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
