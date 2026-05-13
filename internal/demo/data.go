package demo

import (
	"time"

	"github.com/navitronic/gitlab-pipelines/internal/pipeline"
)

// Pipelines returns a fixed set of demo pipelines across multiple projects.
func Pipelines() []pipeline.Pipeline {
	now := time.Now()
	return []pipeline.Pipeline{
		// acme/frontend
		{
			ID: "48291", ProjectID: "112", Project: "acme/frontend",
			Ref: "main", SHA: "a1b2c3d4e5f6a7b8",
			Status: pipeline.StatusPassed, Source: "push",
			WebURL:    "https://gitlab.com/acme/frontend/-/pipelines/48291",
			CreatedAt: now.Add(-12 * time.Minute), UpdatedAt: now.Add(-8 * time.Minute),
			Duration: 4*time.Minute + 12*time.Second,
		},
		{
			ID: "48290", ProjectID: "112", Project: "acme/frontend",
			Ref: "feat/dark-mode", SHA: "d4e5f6a7b8c9d0e1",
			Status: pipeline.StatusRunning, Source: "push",
			WebURL:    "https://gitlab.com/acme/frontend/-/pipelines/48290",
			CreatedAt: now.Add(-3 * time.Minute), UpdatedAt: now.Add(-30 * time.Second),
		},
		{
			ID: "48285", ProjectID: "112", Project: "acme/frontend",
			Ref: "fix/nav-overflow", SHA: "c9d0e1f2a3b4c5d6",
			Status: pipeline.StatusFailed, Source: "merge_request",
			WebURL:    "https://gitlab.com/acme/frontend/-/pipelines/48285",
			CreatedAt: now.Add(-45 * time.Minute), UpdatedAt: now.Add(-40 * time.Minute),
			Duration: 2*time.Minute + 47*time.Second,
		},

		// acme/api-gateway
		{
			ID: "31074", ProjectID: "85", Project: "acme/api-gateway",
			Ref: "main", SHA: "f2a3b4c5d6e7f8a9",
			Status: pipeline.StatusPassed, Source: "push",
			WebURL:    "https://gitlab.com/acme/api-gateway/-/pipelines/31074",
			CreatedAt: now.Add(-25 * time.Minute), UpdatedAt: now.Add(-19 * time.Minute),
			Duration: 5*time.Minute + 38*time.Second,
		},
		{
			ID: "31073", ProjectID: "85", Project: "acme/api-gateway",
			Ref: "feat/rate-limiting", SHA: "b4c5d6e7f8a9b0c1",
			Status: pipeline.StatusPassed, Source: "merge_request",
			WebURL:    "https://gitlab.com/acme/api-gateway/-/pipelines/31073",
			CreatedAt: now.Add(-1 * time.Hour), UpdatedAt: now.Add(-52 * time.Minute),
			Duration: 7*time.Minute + 15*time.Second,
		},

		// acme/shared-libs
		{
			ID: "9823", ProjectID: "47", Project: "acme/shared-libs",
			Ref: "main", SHA: "e7f8a9b0c1d2e3f4",
			Status: pipeline.StatusPassed, Source: "push",
			WebURL:    "https://gitlab.com/acme/shared-libs/-/pipelines/9823",
			CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now.Add(-115 * time.Minute),
			Duration: 3*time.Minute + 5*time.Second,
		},
		{
			ID: "9822", ProjectID: "47", Project: "acme/shared-libs",
			Ref: "chore/bump-deps", SHA: "a9b0c1d2e3f4a5b6",
			Status: pipeline.StatusCanceled, Source: "push",
			WebURL:    "https://gitlab.com/acme/shared-libs/-/pipelines/9822",
			CreatedAt: now.Add(-3 * time.Hour), UpdatedAt: now.Add(-170 * time.Minute),
			Duration: 1*time.Minute + 22*time.Second,
		},
	}
}

// Jobs returns demo jobs for a given pipeline ID.
func Jobs(pipelineID string) []pipeline.Job {
	now := time.Now()
	jobs, ok := demoJobs(now)[pipelineID]
	if !ok {
		return nil
	}
	return jobs
}

// MRURLs maps pipeline IDs to merge request URLs for demo data.
var MRURLs = map[string]string{
	"48285": "https://gitlab.com/acme/frontend/-/merge_requests/142",
	"48290": "https://gitlab.com/acme/frontend/-/merge_requests/158",
	"31073": "https://gitlab.com/acme/api-gateway/-/merge_requests/87",
}

func demoJobs(now time.Time) map[string][]pipeline.Job {
	return map[string][]pipeline.Job{
		// acme/frontend — main (passed)
		"48291": {
			{ID: "120001", Name: "install", Stage: "prepare", Status: pipeline.StatusPassed, CreatedAt: now.Add(-12 * time.Minute), StartedAt: now.Add(-12 * time.Minute), Duration: 18 * time.Second},
			{ID: "120002", Name: "lint", Stage: "test", Status: pipeline.StatusPassed, CreatedAt: now.Add(-12 * time.Minute), StartedAt: now.Add(-11 * time.Minute), Duration: 35 * time.Second},
			{ID: "120003", Name: "unit-tests", Stage: "test", Status: pipeline.StatusPassed, CreatedAt: now.Add(-12 * time.Minute), StartedAt: now.Add(-11 * time.Minute), Duration: 1*time.Minute + 42*time.Second},
			{ID: "120004", Name: "build", Stage: "build", Status: pipeline.StatusPassed, CreatedAt: now.Add(-12 * time.Minute), StartedAt: now.Add(-9 * time.Minute), Duration: 55 * time.Second},
			{ID: "120005", Name: "deploy-staging", Stage: "deploy", Status: pipeline.StatusPassed, CreatedAt: now.Add(-12 * time.Minute), StartedAt: now.Add(-8 * time.Minute), Duration: 30 * time.Second},
		},

		// acme/frontend — feat/dark-mode (running)
		"48290": {
			{ID: "120010", Name: "install", Stage: "prepare", Status: pipeline.StatusPassed, CreatedAt: now.Add(-3 * time.Minute), StartedAt: now.Add(-3 * time.Minute), Duration: 20 * time.Second},
			{ID: "120011", Name: "lint", Stage: "test", Status: pipeline.StatusPassed, CreatedAt: now.Add(-3 * time.Minute), StartedAt: now.Add(-2*time.Minute - 30*time.Second), Duration: 28 * time.Second},
			{ID: "120012", Name: "unit-tests", Stage: "test", Status: pipeline.StatusRunning, CreatedAt: now.Add(-3 * time.Minute), StartedAt: now.Add(-2 * time.Minute)},
			{ID: "120013", Name: "build", Stage: "build", Status: pipeline.StatusPending, CreatedAt: now.Add(-3 * time.Minute)},
			{ID: "120014", Name: "deploy-staging", Stage: "deploy", Status: pipeline.StatusPending, CreatedAt: now.Add(-3 * time.Minute)},
		},

		// acme/frontend — fix/nav-overflow (failed)
		"48285": {
			{ID: "120020", Name: "install", Stage: "prepare", Status: pipeline.StatusPassed, CreatedAt: now.Add(-45 * time.Minute), StartedAt: now.Add(-45 * time.Minute), Duration: 19 * time.Second},
			{ID: "120021", Name: "lint", Stage: "test", Status: pipeline.StatusPassed, CreatedAt: now.Add(-45 * time.Minute), StartedAt: now.Add(-44 * time.Minute), Duration: 31 * time.Second},
			{ID: "120022", Name: "unit-tests", Stage: "test", Status: pipeline.StatusFailed, CreatedAt: now.Add(-45 * time.Minute), StartedAt: now.Add(-44 * time.Minute), Duration: 1*time.Minute + 15*time.Second},
			{ID: "120023", Name: "build", Stage: "build", Status: pipeline.StatusSkipped, CreatedAt: now.Add(-45 * time.Minute)},
			{ID: "120024", Name: "deploy-staging", Stage: "deploy", Status: pipeline.StatusSkipped, CreatedAt: now.Add(-45 * time.Minute)},
		},

		// acme/api-gateway — main (passed)
		"31074": {
			{ID: "120030", Name: "build", Stage: "build", Status: pipeline.StatusPassed, CreatedAt: now.Add(-25 * time.Minute), StartedAt: now.Add(-25 * time.Minute), Duration: 1*time.Minute + 10*time.Second},
			{ID: "120031", Name: "unit-tests", Stage: "test", Status: pipeline.StatusPassed, CreatedAt: now.Add(-25 * time.Minute), StartedAt: now.Add(-23 * time.Minute), Duration: 2*time.Minute + 5*time.Second},
			{ID: "120032", Name: "integration-tests", Stage: "test", Status: pipeline.StatusPassed, CreatedAt: now.Add(-25 * time.Minute), StartedAt: now.Add(-23 * time.Minute), Duration: 3*time.Minute + 18*time.Second},
			{ID: "120033", Name: "security-scan", Stage: "test", Status: pipeline.StatusPassed, AllowFailure: true, CreatedAt: now.Add(-25 * time.Minute), StartedAt: now.Add(-23 * time.Minute), Duration: 1*time.Minute + 45*time.Second},
			{ID: "120034", Name: "deploy-staging", Stage: "deploy", Status: pipeline.StatusPassed, CreatedAt: now.Add(-25 * time.Minute), StartedAt: now.Add(-20 * time.Minute), Duration: 42 * time.Second},
		},

		// acme/api-gateway — feat/rate-limiting (passed)
		"31073": {
			{ID: "120040", Name: "build", Stage: "build", Status: pipeline.StatusPassed, CreatedAt: now.Add(-1 * time.Hour), StartedAt: now.Add(-1 * time.Hour), Duration: 1*time.Minute + 8*time.Second},
			{ID: "120041", Name: "unit-tests", Stage: "test", Status: pipeline.StatusPassed, CreatedAt: now.Add(-1 * time.Hour), StartedAt: now.Add(-58 * time.Minute), Duration: 2*time.Minute + 12*time.Second},
			{ID: "120042", Name: "integration-tests", Stage: "test", Status: pipeline.StatusPassed, CreatedAt: now.Add(-1 * time.Hour), StartedAt: now.Add(-58 * time.Minute), Duration: 4*time.Minute + 2*time.Second},
			{ID: "120043", Name: "security-scan", Stage: "test", Status: pipeline.StatusPassed, AllowFailure: true, CreatedAt: now.Add(-1 * time.Hour), StartedAt: now.Add(-58 * time.Minute), Duration: 1*time.Minute + 50*time.Second},
			{ID: "120044", Name: "deploy-staging", Stage: "deploy", Status: pipeline.StatusPassed, CreatedAt: now.Add(-1 * time.Hour), StartedAt: now.Add(-53 * time.Minute), Duration: 38 * time.Second},
		},

		// acme/shared-libs — main (passed)
		"9823": {
			{ID: "120050", Name: "build", Stage: "build", Status: pipeline.StatusPassed, CreatedAt: now.Add(-2 * time.Hour), StartedAt: now.Add(-2 * time.Hour), Duration: 45 * time.Second},
			{ID: "120051", Name: "test", Stage: "test", Status: pipeline.StatusPassed, CreatedAt: now.Add(-2 * time.Hour), StartedAt: now.Add(-118 * time.Minute), Duration: 1*time.Minute + 55*time.Second},
			{ID: "120052", Name: "publish", Stage: "deploy", Status: pipeline.StatusPassed, CreatedAt: now.Add(-2 * time.Hour), StartedAt: now.Add(-116 * time.Minute), Duration: 25 * time.Second},
		},

		// acme/shared-libs — chore/bump-deps (canceled)
		"9822": {
			{ID: "120060", Name: "build", Stage: "build", Status: pipeline.StatusPassed, CreatedAt: now.Add(-3 * time.Hour), StartedAt: now.Add(-3 * time.Hour), Duration: 48 * time.Second},
			{ID: "120061", Name: "test", Stage: "test", Status: pipeline.StatusCanceled, CreatedAt: now.Add(-3 * time.Hour), StartedAt: now.Add(-178 * time.Minute), Duration: 34 * time.Second},
			{ID: "120062", Name: "publish", Stage: "deploy", Status: pipeline.StatusCanceled, CreatedAt: now.Add(-3 * time.Hour)},
		},
	}
}
