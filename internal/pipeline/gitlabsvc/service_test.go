package gitlabsvc

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/navitronic/gitlab-builds/internal/discovery"
	"github.com/navitronic/gitlab-builds/internal/gitlab"
	"github.com/navitronic/gitlab-builds/internal/glab"
	"github.com/navitronic/gitlab-builds/internal/pipeline"
)

type mockClient struct {
	currentUser              func(ctx context.Context) (*gitlab.User, error)
	fetchUserEventsSince     func(ctx context.Context, userID int, after time.Time) ([]gitlab.Event, error)
	fetchProject             func(ctx context.Context, projectID int) (*gitlab.Project, error)
	fetchPipelinesByUser     func(ctx context.Context, projectID int, userID int, updatedAfter time.Time) ([]gitlab.Pipeline, error)
	fetchPipeline            func(ctx context.Context, projectID int, pipelineID int) (gitlab.Pipeline, error)
	fetchPipelineJobs        func(ctx context.Context, projectID int, pipelineID int) ([]gitlab.Job, error)
	fetchMergeRequestByBranch func(ctx context.Context, projectID int, branch string) (gitlab.MergeRequest, error)
	fetchUserMergeRequests   func(ctx context.Context, updatedAfter time.Time) ([]gitlab.MergeRequest, error)
}

func (m *mockClient) CurrentUser(ctx context.Context) (*gitlab.User, error) {
	return m.currentUser(ctx)
}
func (m *mockClient) FetchUserEventsSince(ctx context.Context, userID int, after time.Time) ([]gitlab.Event, error) {
	return m.fetchUserEventsSince(ctx, userID, after)
}
func (m *mockClient) FetchProject(ctx context.Context, projectID int) (*gitlab.Project, error) {
	return m.fetchProject(ctx, projectID)
}
func (m *mockClient) FetchPipelinesByUser(ctx context.Context, projectID int, userID int, updatedAfter time.Time) ([]gitlab.Pipeline, error) {
	return m.fetchPipelinesByUser(ctx, projectID, userID, updatedAfter)
}
func (m *mockClient) FetchPipeline(ctx context.Context, projectID int, pipelineID int) (gitlab.Pipeline, error) {
	return m.fetchPipeline(ctx, projectID, pipelineID)
}
func (m *mockClient) FetchPipelineJobs(ctx context.Context, projectID int, pipelineID int) ([]gitlab.Job, error) {
	return m.fetchPipelineJobs(ctx, projectID, pipelineID)
}
func (m *mockClient) FetchMergeRequestByBranch(ctx context.Context, projectID int, branch string) (gitlab.MergeRequest, error) {
	return m.fetchMergeRequestByBranch(ctx, projectID, branch)
}
func (m *mockClient) FetchUserMergeRequests(ctx context.Context, updatedAfter time.Time) ([]gitlab.MergeRequest, error) {
	if m.fetchUserMergeRequests != nil {
		return m.fetchUserMergeRequests(ctx, updatedAfter)
	}
	return nil, nil
}

func TestGetPipeline(t *testing.T) {
	mc := &mockClient{
		fetchPipeline: func(_ context.Context, projectID, pipelineID int) (gitlab.Pipeline, error) {
			return gitlab.Pipeline{ID: pipelineID, Status: "success", Ref: "main", SHA: "abc"}, nil
		},
	}
	svc := NewWithClient(mc)

	p, err := svc.GetPipeline(context.Background(), "42", "99")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.ID != "99" {
		t.Errorf("ID = %q, want \"99\"", p.ID)
	}
	if p.Status != pipeline.StatusPassed {
		t.Errorf("Status = %v, want StatusPassed", p.Status)
	}
}

func TestGetPipeline_InvalidProjectID(t *testing.T) {
	svc := NewWithClient(&mockClient{})
	_, err := svc.GetPipeline(context.Background(), "not-a-number", "99")
	if err == nil {
		t.Fatal("expected error for invalid project ID")
	}
}

func TestGetPipeline_InvalidPipelineID(t *testing.T) {
	svc := NewWithClient(&mockClient{})
	_, err := svc.GetPipeline(context.Background(), "42", "not-a-number")
	if err == nil {
		t.Fatal("expected error for invalid pipeline ID")
	}
}

func TestGetPipeline_FetchError(t *testing.T) {
	mc := &mockClient{
		fetchPipeline: func(_ context.Context, _, _ int) (gitlab.Pipeline, error) {
			return gitlab.Pipeline{}, glab.ErrGlabNotFound
		},
	}
	svc := NewWithClient(mc)
	_, err := svc.GetPipeline(context.Background(), "42", "99")
	if !errors.Is(err, pipeline.ErrClientNotFound) {
		t.Errorf("expected ErrClientNotFound, got %v", err)
	}
}

func TestListJobs(t *testing.T) {
	mc := &mockClient{
		fetchPipelineJobs: func(_ context.Context, _, _ int) ([]gitlab.Job, error) {
			return []gitlab.Job{
				{ID: 1, Name: "build", Stage: "build", Status: "success", Duration: 30},
				{ID: 2, Name: "test", Stage: "test", Status: "running", Duration: 10},
			}, nil
		},
	}
	svc := NewWithClient(mc)

	jobs, err := svc.ListJobs(context.Background(), "42", "99")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
	if jobs[0].Name != "build" {
		t.Errorf("jobs[0].Name = %q, want \"build\"", jobs[0].Name)
	}
}

func TestListJobs_InvalidProjectID(t *testing.T) {
	svc := NewWithClient(&mockClient{})
	_, err := svc.ListJobs(context.Background(), "abc", "99")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListJobs_InvalidPipelineID(t *testing.T) {
	svc := NewWithClient(&mockClient{})
	_, err := svc.ListJobs(context.Background(), "42", "abc")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListJobs_FetchError(t *testing.T) {
	mc := &mockClient{
		fetchPipelineJobs: func(_ context.Context, _, _ int) ([]gitlab.Job, error) {
			return nil, glab.ErrAuthRequired
		},
	}
	svc := NewWithClient(mc)
	_, err := svc.ListJobs(context.Background(), "42", "99")
	if !errors.Is(err, pipeline.ErrAuthRequired) {
		t.Errorf("expected ErrAuthRequired, got %v", err)
	}
}

func TestGetMergeRequestURL(t *testing.T) {
	mc := &mockClient{
		fetchMergeRequestByBranch: func(_ context.Context, _ int, branch string) (gitlab.MergeRequest, error) {
			return gitlab.MergeRequest{IID: 5, WebURL: "https://gitlab.com/mr/5"}, nil
		},
	}
	svc := NewWithClient(mc)

	url, err := svc.GetMergeRequestURL(context.Background(), "42", "feature")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://gitlab.com/mr/5" {
		t.Errorf("URL = %q", url)
	}
}

func TestGetMergeRequestURL_InvalidProjectID(t *testing.T) {
	svc := NewWithClient(&mockClient{})
	_, err := svc.GetMergeRequestURL(context.Background(), "abc", "main")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetMergeRequestURL_FetchError(t *testing.T) {
	mc := &mockClient{
		fetchMergeRequestByBranch: func(_ context.Context, _ int, _ string) (gitlab.MergeRequest, error) {
			return gitlab.MergeRequest{}, fmt.Errorf("network error")
		},
	}
	svc := NewWithClient(mc)
	_, err := svc.GetMergeRequestURL(context.Background(), "42", "main")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListPipelines_CurrentUserError(t *testing.T) {
	mc := &mockClient{
		currentUser: func(_ context.Context) (*gitlab.User, error) {
			return nil, glab.ErrGlabNotFound
		},
	}
	svc := NewWithClient(mc)
	_, err := svc.ListPipelines(context.Background(), func(string) {})
	if !errors.Is(err, pipeline.ErrClientNotFound) {
		t.Errorf("expected ErrClientNotFound, got %v", err)
	}
}

func TestListPipelines_EventsError(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "cache"))

	mc := &mockClient{
		currentUser: func(_ context.Context) (*gitlab.User, error) {
			return &gitlab.User{ID: 1, Username: "test"}, nil
		},
		fetchUserEventsSince: func(_ context.Context, _ int, _ time.Time) ([]gitlab.Event, error) {
			return nil, fmt.Errorf("network error")
		},
	}
	svc := NewWithClient(mc)
	_, err := svc.ListPipelines(context.Background(), func(string) {})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListPipelines_NoRepos(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "cache"))

	mc := &mockClient{
		currentUser: func(_ context.Context) (*gitlab.User, error) {
			return &gitlab.User{ID: 1, Username: "test"}, nil
		},
		fetchUserEventsSince: func(_ context.Context, _ int, _ time.Time) ([]gitlab.Event, error) {
			return []gitlab.Event{}, nil
		},
	}
	svc := NewWithClient(mc)
	pipelines, err := svc.ListPipelines(context.Background(), func(string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pipelines != nil {
		t.Errorf("expected nil pipelines, got %v", pipelines)
	}
}

func TestListPipelines_Success(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "cache"))

	now := time.Now()
	mc := &mockClient{
		currentUser: func(_ context.Context) (*gitlab.User, error) {
			return &gitlab.User{ID: 1, Username: "test"}, nil
		},
		fetchUserEventsSince: func(_ context.Context, _ int, _ time.Time) ([]gitlab.Event, error) {
			return []gitlab.Event{
				{ID: 1, ActionName: "pushed to", ProjectID: 42, CreatedAt: now.Add(-1 * time.Hour), PushData: &gitlab.PushData{CommitCount: 1, Ref: "main", RefType: "branch", CommitTo: "abc"}},
			}, nil
		},
		fetchProject: func(_ context.Context, projectID int) (*gitlab.Project, error) {
			return &gitlab.Project{ID: projectID, PathWithNamespace: "group/project"}, nil
		},
		fetchPipelinesByUser: func(_ context.Context, _ int, _ int, _ time.Time) ([]gitlab.Pipeline, error) {
			return []gitlab.Pipeline{
				{ID: 100, Status: "success", Ref: "main", SHA: "abc", UpdatedAt: now},
				{ID: 101, Status: "running", Ref: "feat", SHA: "def", UpdatedAt: now.Add(-5 * time.Minute)},
			}, nil
		},
	}
	svc := NewWithClient(mc)

	pipelines, err := svc.ListPipelines(context.Background(), func(string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pipelines) != 2 {
		t.Fatalf("expected 2 pipelines, got %d", len(pipelines))
	}
	if pipelines[0].ID != "100" {
		t.Errorf("first pipeline ID = %q, want \"100\" (sorted by UpdatedAt)", pipelines[0].ID)
	}
}

func TestListPipelines_PipelineFetchError(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "cache"))

	now := time.Now()
	mc := &mockClient{
		currentUser: func(_ context.Context) (*gitlab.User, error) {
			return &gitlab.User{ID: 1, Username: "test"}, nil
		},
		fetchUserEventsSince: func(_ context.Context, _ int, _ time.Time) ([]gitlab.Event, error) {
			return []gitlab.Event{
				{ID: 1, ActionName: "pushed to", ProjectID: 42, CreatedAt: now.Add(-1 * time.Hour), PushData: &gitlab.PushData{CommitCount: 1, Ref: "main", RefType: "branch", CommitTo: "abc"}},
			}, nil
		},
		fetchProject: func(_ context.Context, projectID int) (*gitlab.Project, error) {
			return &gitlab.Project{ID: projectID, PathWithNamespace: "group/project"}, nil
		},
		fetchPipelinesByUser: func(_ context.Context, _ int, _ int, _ time.Time) ([]gitlab.Pipeline, error) {
			return nil, fmt.Errorf("network error")
		},
	}
	svc := NewWithClient(mc)
	_, err := svc.ListPipelines(context.Background(), func(string) {})
	if err == nil {
		t.Fatal("expected error when all pipeline fetches fail")
	}
}

func TestListPipelines_ProjectPathFromCache(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "cache"))

	now := time.Now()
	fetchProjectCalls := 0
	mc := &mockClient{
		currentUser: func(_ context.Context) (*gitlab.User, error) {
			return &gitlab.User{ID: 1, Username: "test"}, nil
		},
		fetchUserEventsSince: func(_ context.Context, _ int, _ time.Time) ([]gitlab.Event, error) {
			return []gitlab.Event{
				{ID: 1, ActionName: "pushed to", ProjectID: 42, CreatedAt: now.Add(-1 * time.Hour), PushData: &gitlab.PushData{CommitCount: 1, Ref: "main", RefType: "branch", CommitTo: "abc"}},
				{ID: 2, ActionName: "pushed to", ProjectID: 42, CreatedAt: now.Add(-2 * time.Hour), PushData: &gitlab.PushData{CommitCount: 1, Ref: "feat", RefType: "branch", CommitTo: "def"}},
			}, nil
		},
		fetchProject: func(_ context.Context, projectID int) (*gitlab.Project, error) {
			fetchProjectCalls++
			return &gitlab.Project{ID: projectID, PathWithNamespace: "group/project"}, nil
		},
		fetchPipelinesByUser: func(_ context.Context, _ int, _ int, _ time.Time) ([]gitlab.Pipeline, error) {
			return []gitlab.Pipeline{{ID: 100, Status: "success", UpdatedAt: now}}, nil
		},
	}
	svc := NewWithClient(mc)

	_, err := svc.ListPipelines(context.Background(), func(string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fetchProjectCalls > 1 {
		t.Errorf("FetchProject called %d times, expected at most 1", fetchProjectCalls)
	}
}

func TestListPipelines_FetchProjectError(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "cache"))

	now := time.Now()
	mc := &mockClient{
		currentUser: func(_ context.Context) (*gitlab.User, error) {
			return &gitlab.User{ID: 1, Username: "test"}, nil
		},
		fetchUserEventsSince: func(_ context.Context, _ int, _ time.Time) ([]gitlab.Event, error) {
			return []gitlab.Event{
				{ID: 1, ActionName: "pushed to", ProjectID: 42, CreatedAt: now.Add(-1 * time.Hour), PushData: &gitlab.PushData{CommitCount: 1, Ref: "main", RefType: "branch", CommitTo: "abc"}},
			}, nil
		},
		fetchProject: func(_ context.Context, _ int) (*gitlab.Project, error) {
			return nil, fmt.Errorf("not found")
		},
		fetchPipelinesByUser: func(_ context.Context, _ int, _ int, _ time.Time) ([]gitlab.Pipeline, error) {
			return []gitlab.Pipeline{{ID: 100, Status: "success", UpdatedAt: now}}, nil
		},
	}
	svc := NewWithClient(mc)

	pipelines, err := svc.ListPipelines(context.Background(), func(string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pipelines) != 1 {
		t.Fatalf("expected 1 pipeline, got %d", len(pipelines))
	}
}



func TestListPipelines_IncludesMRProjects(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "cache"))

	now := time.Now()
	mc := &mockClient{
		currentUser: func(_ context.Context) (*gitlab.User, error) {
			return &gitlab.User{ID: 1, Username: "test"}, nil
		},
		fetchUserEventsSince: func(_ context.Context, _ int, _ time.Time) ([]gitlab.Event, error) {
			return []gitlab.Event{
				{ID: 1, ActionName: "pushed to", ProjectID: 42, CreatedAt: now.Add(-1 * time.Hour), PushData: &gitlab.PushData{CommitCount: 1, Ref: "main", RefType: "branch", CommitTo: "abc"}},
			}, nil
		},
		fetchUserMergeRequests: func(_ context.Context, _ time.Time) ([]gitlab.MergeRequest, error) {
			return []gitlab.MergeRequest{
				{IID: 10, ProjectID: 99, State: "opened", Title: "New feature", UpdatedAt: now},
			}, nil
		},
		fetchProject: func(_ context.Context, projectID int) (*gitlab.Project, error) {
			return &gitlab.Project{ID: projectID, PathWithNamespace: fmt.Sprintf("group/project-%d", projectID)}, nil
		},
		fetchPipelinesByUser: func(_ context.Context, projectID int, _ int, _ time.Time) ([]gitlab.Pipeline, error) {
			return []gitlab.Pipeline{
				{ID: projectID * 10, Status: "success", UpdatedAt: now},
			}, nil
		},
	}
	svc := NewWithClient(mc)

	pipelines, err := svc.ListPipelines(context.Background(), func(string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pipelines) != 2 {
		t.Fatalf("expected 2 pipelines, got %d", len(pipelines))
	}
	projectIDs := map[string]bool{}
	for _, p := range pipelines {
		projectIDs[p.ProjectID] = true
	}
	if !projectIDs["42"] {
		t.Error("expected pipelines from event-discovered project 42")
	}
	if !projectIDs["99"] {
		t.Error("expected pipelines from MR-discovered project 99")
	}
}

func TestListPipelines_MRProjectDeduped(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "cache"))

	now := time.Now()
	pipelineFetchCount := 0
	mc := &mockClient{
		currentUser: func(_ context.Context) (*gitlab.User, error) {
			return &gitlab.User{ID: 1, Username: "test"}, nil
		},
		fetchUserEventsSince: func(_ context.Context, _ int, _ time.Time) ([]gitlab.Event, error) {
			return []gitlab.Event{
				{ID: 1, ActionName: "pushed to", ProjectID: 42, CreatedAt: now.Add(-1 * time.Hour), PushData: &gitlab.PushData{CommitCount: 1, Ref: "main", RefType: "branch", CommitTo: "abc"}},
			}, nil
		},
		fetchUserMergeRequests: func(_ context.Context, _ time.Time) ([]gitlab.MergeRequest, error) {
			return []gitlab.MergeRequest{
				{IID: 10, ProjectID: 42, State: "opened", Title: "Same project", UpdatedAt: now},
			}, nil
		},
		fetchProject: func(_ context.Context, projectID int) (*gitlab.Project, error) {
			return &gitlab.Project{ID: projectID, PathWithNamespace: "group/project"}, nil
		},
		fetchPipelinesByUser: func(_ context.Context, _ int, _ int, _ time.Time) ([]gitlab.Pipeline, error) {
			pipelineFetchCount++
			return []gitlab.Pipeline{{ID: 100, Status: "success", UpdatedAt: now}}, nil
		},
	}
	svc := NewWithClient(mc)

	pipelines, err := svc.ListPipelines(context.Background(), func(string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pipelines) != 1 {
		t.Fatalf("expected 1 pipeline (deduped), got %d", len(pipelines))
	}
	if pipelineFetchCount != 1 {
		t.Errorf("expected 1 pipeline fetch (deduped), got %d", pipelineFetchCount)
	}
}

func TestListPipelines_MRFetchErrorNonFatal(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "cache"))

	now := time.Now()
	mc := &mockClient{
		currentUser: func(_ context.Context) (*gitlab.User, error) {
			return &gitlab.User{ID: 1, Username: "test"}, nil
		},
		fetchUserEventsSince: func(_ context.Context, _ int, _ time.Time) ([]gitlab.Event, error) {
			return []gitlab.Event{
				{ID: 1, ActionName: "pushed to", ProjectID: 42, CreatedAt: now.Add(-1 * time.Hour), PushData: &gitlab.PushData{CommitCount: 1, Ref: "main", RefType: "branch", CommitTo: "abc"}},
			}, nil
		},
		fetchUserMergeRequests: func(_ context.Context, _ time.Time) ([]gitlab.MergeRequest, error) {
			return nil, fmt.Errorf("network error")
		},
		fetchProject: func(_ context.Context, projectID int) (*gitlab.Project, error) {
			return &gitlab.Project{ID: projectID, PathWithNamespace: "group/project"}, nil
		},
		fetchPipelinesByUser: func(_ context.Context, _ int, _ int, _ time.Time) ([]gitlab.Pipeline, error) {
			return []gitlab.Pipeline{{ID: 100, Status: "success", UpdatedAt: now}}, nil
		},
	}
	svc := NewWithClient(mc)

	pipelines, err := svc.ListPipelines(context.Background(), func(string) {})
	if err != nil {
		t.Fatalf("MR fetch error should be non-fatal, got: %v", err)
	}
	if len(pipelines) != 1 {
		t.Fatalf("expected 1 pipeline from events, got %d", len(pipelines))
	}
}

func TestMergeReposFromMRs(t *testing.T) {
	now := time.Now()
	repos := []discovery.ActiveRepo{
		{ProjectID: 10, LastActive: now},
	}
	mrs := []gitlab.MergeRequest{
		{IID: 1, ProjectID: 10, State: "opened", UpdatedAt: now},
		{IID: 2, ProjectID: 20, State: "merged", UpdatedAt: now},
		{IID: 3, ProjectID: 0, State: "opened", UpdatedAt: now},
		{IID: 4, ProjectID: 30, State: "opened", UpdatedAt: now},
	}

	result := mergeReposFromMRs(repos, mrs)
	if len(result) != 3 {
		t.Fatalf("expected 3 repos, got %d", len(result))
	}
	ids := map[int]bool{}
	for _, r := range result {
		ids[r.ProjectID] = true
	}
	if !ids[10] || !ids[20] || !ids[30] {
		t.Errorf("expected projects 10, 20, 30; got %v", ids)
	}
}

func TestConvertStatus(t *testing.T) {
	tests := []struct {
		input string
		want  pipeline.Status
	}{
		{"success", pipeline.StatusPassed},
		{"failed", pipeline.StatusFailed},
		{"running", pipeline.StatusRunning},
		{"pending", pipeline.StatusPending},
		{"canceled", pipeline.StatusCanceled},
		{"skipped", pipeline.StatusSkipped},
		{"unknown", pipeline.StatusPending},
		{"", pipeline.StatusPending},
	}
	for _, tt := range tests {
		if got := convertStatus(tt.input); got != tt.want {
			t.Errorf("convertStatus(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestConvertPipeline(t *testing.T) {
	now := time.Now()
	input := gitlab.Pipeline{
		ID:        42,
		SHA:       "abc123",
		Ref:       "main",
		Status:    "success",
		Source:    "push",
		WebURL:    "https://gitlab.com/p/42",
		CreatedAt: now.Add(-10 * time.Minute),
		UpdatedAt: now,
		Duration:  125.5,
	}

	got := convertPipeline(input, 99, "group/project")

	if got.ID != "42" {
		t.Errorf("ID = %q, want \"42\"", got.ID)
	}
	if got.ProjectID != "99" {
		t.Errorf("ProjectID = %q, want \"99\"", got.ProjectID)
	}
	if got.Project != "group/project" {
		t.Errorf("Project = %q, want \"group/project\"", got.Project)
	}
	if got.SHA != "abc123" {
		t.Errorf("SHA = %q, want \"abc123\"", got.SHA)
	}
	if got.Ref != "main" {
		t.Errorf("Ref = %q, want \"main\"", got.Ref)
	}
	if got.Status != pipeline.StatusPassed {
		t.Errorf("Status = %v, want StatusPassed", got.Status)
	}
	if got.Source != "push" {
		t.Errorf("Source = %q, want \"push\"", got.Source)
	}
	if got.WebURL != "https://gitlab.com/p/42" {
		t.Errorf("WebURL = %q", got.WebURL)
	}
	expectedDur := time.Duration(125.5 * float64(time.Second))
	if got.Duration != expectedDur {
		t.Errorf("Duration = %v, want %v", got.Duration, expectedDur)
	}
}

func TestConvertJob(t *testing.T) {
	now := time.Now()
	input := gitlab.Job{
		ID:        7,
		Name:      "test",
		Stage:     "test",
		Status:    "running",
		WebURL:    "https://gitlab.com/j/7",
		CreatedAt: now.Add(-5 * time.Minute),
		StartedAt: now.Add(-2 * time.Minute),
		Duration:  60.0,
	}

	got := convertJob(input)

	if got.ID != "7" {
		t.Errorf("ID = %q, want \"7\"", got.ID)
	}
	if got.Name != "test" {
		t.Errorf("Name = %q, want \"test\"", got.Name)
	}
	if got.Stage != "test" {
		t.Errorf("Stage = %q, want \"test\"", got.Stage)
	}
	if got.Status != pipeline.StatusRunning {
		t.Errorf("Status = %v, want StatusRunning", got.Status)
	}
	if got.Duration != 60*time.Second {
		t.Errorf("Duration = %v, want 60s", got.Duration)
	}
}

func TestWrapErr(t *testing.T) {
	err := wrapErr(glab.ErrGlabNotFound)
	if !errors.Is(err, pipeline.ErrClientNotFound) {
		t.Errorf("wrapErr(ErrGlabNotFound) should wrap ErrClientNotFound, got: %v", err)
	}

	err = wrapErr(glab.ErrAuthRequired)
	if !errors.Is(err, pipeline.ErrAuthRequired) {
		t.Errorf("wrapErr(ErrAuthRequired) should wrap ErrAuthRequired, got: %v", err)
	}

	other := fmt.Errorf("something else")
	err = wrapErr(other)
	if err != other {
		t.Errorf("wrapErr(other) should pass through, got: %v", err)
	}
}
