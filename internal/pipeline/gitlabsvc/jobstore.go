package gitlabsvc

import (
	"context"
	"sort"
	"time"

	"github.com/navitronic/gitlab-pipelines/internal/gitlab"
	"github.com/navitronic/gitlab-pipelines/internal/pipeline"
)

// JobStore tracks a project's jobs across repeated refreshes so that, after
// the first fetch, a refresh only needs to fetch jobs created since the
// last refresh and re-fetch details for jobs still in progress — rather
// than re-fetching the entire day's job list every time.
type JobStore struct {
	client      GitLabClient
	projectPath string
	limit       int
	stages      map[string]struct{}

	projectID int
	jobs      map[int]gitlab.Job
	maxID     int
}

// NewJobStore creates a store for a project's jobs. If stages is non-empty,
// jobs returned by Refresh are filtered to those stages.
func NewJobStore(client GitLabClient, projectPath string, limit int, stages []string) *JobStore {
	stageSet := make(map[string]struct{}, len(stages))
	for _, s := range stages {
		stageSet[s] = struct{}{}
	}
	return &JobStore{
		client:      client,
		projectPath: projectPath,
		limit:       limit,
		stages:      stageSet,
		jobs:        make(map[int]gitlab.Job),
	}
}

// Refresh updates the store and returns today's known jobs (filtered by
// stage if configured), newest first.
//
// The first call fetches all of today's jobs. Every call after that only
// fetches jobs created since the highest job ID seen so far, plus re-fetches
// (one request per job) any job that was still running or pending as of the
// last refresh.
func (js *JobStore) Refresh(ctx context.Context, progress func(string)) ([]pipeline.Job, error) {
	if js.projectID == 0 {
		progress("fetching project...")
		project, err := js.client.FetchProjectByPath(ctx, js.projectPath)
		if err != nil {
			return nil, wrapErr(err)
		}
		js.projectID = project.ID
	}

	if len(js.jobs) == 0 {
		progress("fetching jobs...")
		jobs, err := js.client.FetchProjectJobs(ctx, js.projectID, startOfDay(time.Now()), js.limit)
		if err != nil {
			return nil, wrapErr(err)
		}
		js.upsert(jobs)
		return js.filtered(), nil
	}

	// Capture in-progress jobs from before this cycle's fetch: jobs newly
	// discovered below already carry fresh status, so they don't need an
	// immediate re-fetch too.
	inProgress := js.inProgressIDs()

	progress("checking for new jobs...")
	newJobs, err := js.client.FetchProjectJobsSince(ctx, js.projectID, js.maxID, js.limit)
	if err != nil {
		return nil, wrapErr(err)
	}
	js.upsert(newJobs)

	if len(inProgress) > 0 {
		progress("checking in-progress jobs...")
		for _, id := range inProgress {
			job, err := js.client.FetchJob(ctx, js.projectID, id)
			if err != nil {
				// Best-effort: keep the stale entry and retry on the next refresh
				// rather than failing the whole cycle over one job.
				continue
			}
			js.upsert([]gitlab.Job{job})
		}
	}

	return js.filtered(), nil
}

func (js *JobStore) upsert(jobs []gitlab.Job) {
	for _, j := range jobs {
		js.jobs[j.ID] = j
		if j.ID > js.maxID {
			js.maxID = j.ID
		}
	}
}

func (js *JobStore) inProgressIDs() []int {
	var ids []int
	for id, j := range js.jobs {
		if !isTerminalJobStatus(j.Status) {
			ids = append(ids, id)
		}
	}
	return ids
}

// isTerminalJobStatus reports whether a job's raw GitLab status is a final
// state that will never change. Anything else (pending, running, and the
// less common created/preparing/scheduled/waiting_for_resource/
// waiting_for_callback/manual/canceling states) is treated as still in
// progress so it keeps getting re-checked on future refreshes, rather than
// only recognizing the two most common non-terminal statuses.
func isTerminalJobStatus(status string) bool {
	switch status {
	case "success", "failed", "canceled", "skipped":
		return true
	default:
		return false
	}
}

// filtered returns the store's jobs that fall on today's calendar day and
// match the configured stage filter (if any), converted for display and
// sorted newest first.
func (js *JobStore) filtered() []pipeline.Job {
	cutoff := startOfDay(time.Now())
	out := make([]pipeline.Job, 0, len(js.jobs))
	for _, j := range js.jobs {
		if j.CreatedAt.Before(cutoff) {
			continue
		}
		if len(js.stages) > 0 {
			if _, ok := js.stages[j.Stage]; !ok {
				continue
			}
		}
		out = append(out, convertJob(j))
	}
	sort.Slice(out, func(i, k int) bool {
		return out[i].CreatedAt.After(out[k].CreatedAt)
	})
	return out
}
