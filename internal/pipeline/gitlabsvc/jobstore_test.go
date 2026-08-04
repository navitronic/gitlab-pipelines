package gitlabsvc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/navitronic/gitlab-pipelines/internal/gitlab"
	"github.com/navitronic/gitlab-pipelines/internal/glab"
	"github.com/navitronic/gitlab-pipelines/internal/pipeline"
)

func TestJobStore_Refresh_FirstCallDoesFullFetch(t *testing.T) {
	now := time.Now()
	var sinceCalls int
	mc := &mockClient{
		fetchProjectByPath: func(_ context.Context, projectPath string) (*gitlab.Project, error) {
			if projectPath != "group/project" {
				t.Fatalf("projectPath = %q, want group/project", projectPath)
			}
			return &gitlab.Project{ID: 42, PathWithNamespace: projectPath}, nil
		},
		fetchProjectJobs: func(_ context.Context, projectID int, cutoff time.Time, limit int) ([]gitlab.Job, error) {
			if projectID != 42 {
				t.Fatalf("projectID = %d, want 42", projectID)
			}
			if limit != 25 {
				t.Fatalf("limit = %d, want 25", limit)
			}
			if !cutoff.Equal(startOfDay(now)) {
				t.Errorf("cutoff = %v, want start of today", cutoff)
			}
			return []gitlab.Job{
				{ID: 1, Name: "build", Stage: "build", Status: "success", CreatedAt: now},
				{ID: 2, Name: "test", Stage: "test", Status: "running", CreatedAt: now},
			}, nil
		},
		fetchProjectJobsSince: func(context.Context, int, int, int) ([]gitlab.Job, error) {
			sinceCalls++
			return nil, nil
		},
	}
	store := NewJobStore(mc, "group/project", 25, nil)

	jobs, err := store.Refresh(context.Background(), func(string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
	if sinceCalls != 0 {
		t.Errorf("expected FetchProjectJobsSince not to be called on first refresh, got %d calls", sinceCalls)
	}
}

func TestJobStore_Refresh_SecondCallOnlyFetchesNewAndInProgress(t *testing.T) {
	now := time.Now()
	var fullFetchCalls, sinceCalls int
	var fetchedJobIDs []int

	mc := &mockClient{
		fetchProjectByPath: func(_ context.Context, projectPath string) (*gitlab.Project, error) {
			return &gitlab.Project{ID: 42, PathWithNamespace: projectPath}, nil
		},
		fetchProjectJobs: func(context.Context, int, time.Time, int) ([]gitlab.Job, error) {
			fullFetchCalls++
			return []gitlab.Job{
				{ID: 1, Name: "build", Stage: "build", Status: "success", CreatedAt: now},
				{ID: 2, Name: "test", Stage: "test", Status: "running", CreatedAt: now},
			}, nil
		},
		fetchProjectJobsSince: func(_ context.Context, projectID int, sinceID int, limit int) ([]gitlab.Job, error) {
			sinceCalls++
			if sinceID != 2 {
				t.Errorf("sinceID = %d, want 2 (the max ID from the first fetch)", sinceID)
			}
			return []gitlab.Job{
				{ID: 3, Name: "deploy", Stage: "deploy", Status: "pending", CreatedAt: now},
			}, nil
		},
		fetchJob: func(_ context.Context, projectID int, jobID int) (gitlab.Job, error) {
			fetchedJobIDs = append(fetchedJobIDs, jobID)
			// job 2 ("test") finishes as failed on this refresh.
			return gitlab.Job{ID: 2, Name: "test", Stage: "test", Status: "failed", CreatedAt: now}, nil
		},
	}
	store := NewJobStore(mc, "group/project", 25, nil)

	// First refresh: full fetch, seeds job 1 (done) and job 2 (running).
	if _, err := store.Refresh(context.Background(), func(string) {}); err != nil {
		t.Fatalf("unexpected error on first refresh: %v", err)
	}

	// Second refresh: should only look for jobs newer than ID 2, and
	// re-fetch job 2 since it was still running.
	jobs, err := store.Refresh(context.Background(), func(string) {})
	if err != nil {
		t.Fatalf("unexpected error on second refresh: %v", err)
	}

	if fullFetchCalls != 1 {
		t.Errorf("expected full fetch to happen exactly once, got %d", fullFetchCalls)
	}
	if sinceCalls != 1 {
		t.Errorf("expected exactly 1 incremental fetch, got %d", sinceCalls)
	}
	if len(fetchedJobIDs) != 1 || fetchedJobIDs[0] != 2 {
		t.Errorf("expected only job 2 (in progress) to be re-fetched individually, got %v", fetchedJobIDs)
	}

	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs total, got %d", len(jobs))
	}
	statuses := make(map[string]pipeline.Status)
	for _, j := range jobs {
		statuses[j.Name] = j.Status
	}
	if statuses["test"] != pipeline.StatusFailed {
		t.Errorf("job 'test' status = %v, want StatusFailed (updated by re-fetch)", statuses["test"])
	}
	if statuses["deploy"] != pipeline.StatusPending {
		t.Errorf("job 'deploy' status = %v, want StatusPending (newly discovered)", statuses["deploy"])
	}
	if statuses["build"] != pipeline.StatusPassed {
		t.Errorf("job 'build' status = %v, want StatusPassed (untouched, was already done)", statuses["build"])
	}
}

func TestJobStore_Refresh_InProgressFetchErrorIsNonFatal(t *testing.T) {
	now := time.Now()
	mc := &mockClient{
		fetchProjectByPath: func(_ context.Context, projectPath string) (*gitlab.Project, error) {
			return &gitlab.Project{ID: 42, PathWithNamespace: projectPath}, nil
		},
		fetchProjectJobs: func(context.Context, int, time.Time, int) ([]gitlab.Job, error) {
			return []gitlab.Job{
				{ID: 1, Name: "test", Stage: "test", Status: "running", CreatedAt: now},
			}, nil
		},
		fetchProjectJobsSince: func(context.Context, int, int, int) ([]gitlab.Job, error) {
			return nil, nil
		},
		fetchJob: func(context.Context, int, int) (gitlab.Job, error) {
			return gitlab.Job{}, errors.New("transient error")
		},
	}
	store := NewJobStore(mc, "group/project", 25, nil)

	if _, err := store.Refresh(context.Background(), func(string) {}); err != nil {
		t.Fatalf("unexpected error on first refresh: %v", err)
	}

	jobs, err := store.Refresh(context.Background(), func(string) {})
	if err != nil {
		t.Fatalf("expected in-progress fetch errors to be swallowed, got: %v", err)
	}
	if len(jobs) != 1 || jobs[0].Status != pipeline.StatusRunning {
		t.Errorf("expected stale job to remain with its last known status, got %+v", jobs)
	}
}

func TestJobStore_Refresh_StageFilter(t *testing.T) {
	now := time.Now()
	mc := &mockClient{
		fetchProjectByPath: func(_ context.Context, projectPath string) (*gitlab.Project, error) {
			return &gitlab.Project{ID: 42, PathWithNamespace: projectPath}, nil
		},
		fetchProjectJobs: func(context.Context, int, time.Time, int) ([]gitlab.Job, error) {
			return []gitlab.Job{
				{ID: 1, Name: "build", Stage: "build", Status: "success", CreatedAt: now},
				{ID: 2, Name: "unit", Stage: "test", Status: "running", CreatedAt: now},
			}, nil
		},
	}
	store := NewJobStore(mc, "group/project", 25, []string{"test"})

	jobs, err := store.Refresh(context.Background(), func(string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 1 || jobs[0].Name != "unit" {
		t.Fatalf("expected only the 'test' stage job, got %+v", jobs)
	}
}

func TestJobStore_Refresh_ExcludesJobsFromPreviousDay(t *testing.T) {
	yesterday := time.Now().Add(-25 * time.Hour)
	mc := &mockClient{
		fetchProjectByPath: func(_ context.Context, projectPath string) (*gitlab.Project, error) {
			return &gitlab.Project{ID: 42, PathWithNamespace: projectPath}, nil
		},
		fetchProjectJobs: func(context.Context, int, time.Time, int) ([]gitlab.Job, error) {
			// A job that slipped in from before the cutoff (defensive: the
			// store should still exclude it from its filtered output).
			return []gitlab.Job{
				{ID: 1, Name: "old", Stage: "build", Status: "success", CreatedAt: yesterday},
			}, nil
		},
	}
	store := NewJobStore(mc, "group/project", 25, nil)

	jobs, err := store.Refresh(context.Background(), func(string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("expected no jobs (all before today's cutoff), got %d", len(jobs))
	}
}

func TestJobStore_Refresh_ProjectError(t *testing.T) {
	mc := &mockClient{
		fetchProjectByPath: func(_ context.Context, _ string) (*gitlab.Project, error) {
			return nil, glab.ErrAuthRequired
		},
	}
	store := NewJobStore(mc, "group/project", 25, nil)

	_, err := store.Refresh(context.Background(), func(string) {})
	if !errors.Is(err, pipeline.ErrAuthRequired) {
		t.Errorf("expected ErrAuthRequired, got %v", err)
	}
}

func TestJobStore_Refresh_FullFetchError(t *testing.T) {
	mc := &mockClient{
		fetchProjectByPath: func(_ context.Context, projectPath string) (*gitlab.Project, error) {
			return &gitlab.Project{ID: 42, PathWithNamespace: projectPath}, nil
		},
		fetchProjectJobs: func(context.Context, int, time.Time, int) ([]gitlab.Job, error) {
			return nil, glab.ErrGlabNotFound
		},
	}
	store := NewJobStore(mc, "group/project", 25, nil)

	_, err := store.Refresh(context.Background(), func(string) {})
	if !errors.Is(err, pipeline.ErrClientNotFound) {
		t.Errorf("expected ErrClientNotFound, got %v", err)
	}
}

func TestJobStore_Refresh_SinceFetchError(t *testing.T) {
	now := time.Now()
	mc := &mockClient{
		fetchProjectByPath: func(_ context.Context, projectPath string) (*gitlab.Project, error) {
			return &gitlab.Project{ID: 42, PathWithNamespace: projectPath}, nil
		},
		fetchProjectJobs: func(context.Context, int, time.Time, int) ([]gitlab.Job, error) {
			return []gitlab.Job{{ID: 1, Name: "build", Stage: "build", Status: "success", CreatedAt: now}}, nil
		},
		fetchProjectJobsSince: func(context.Context, int, int, int) ([]gitlab.Job, error) {
			return nil, glab.ErrGlabNotFound
		},
	}
	store := NewJobStore(mc, "group/project", 25, nil)

	if _, err := store.Refresh(context.Background(), func(string) {}); err != nil {
		t.Fatalf("unexpected error on first refresh: %v", err)
	}

	_, err := store.Refresh(context.Background(), func(string) {})
	if !errors.Is(err, pipeline.ErrClientNotFound) {
		t.Errorf("expected ErrClientNotFound, got %v", err)
	}
}
