package discovery

import (
	"testing"
	"time"

	"github.com/navitronic/gitlab-builds/internal/gitlab"
)

func TestExtractCandidates_PushEvents(t *testing.T) {
	events := []gitlab.Event{
		{
			ID:         1,
			ActionName: "pushed to",
			ProjectID:  42,
			CreatedAt:  time.Now(),
			PushData: &gitlab.PushData{
				CommitCount: 1,
				Ref:         "main",
				RefType:     "branch",
				CommitTo:    "abc123",
			},
		},
		{
			ID:         2,
			ActionName: "commented on",
			ProjectID:  42,
			CreatedAt:  time.Now(),
		},
	}

	candidates := ExtractCandidates(events)
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if candidates[0].SHA != "abc123" {
		t.Errorf("expected SHA abc123, got %q", candidates[0].SHA)
	}
	if candidates[0].Ref != "main" {
		t.Errorf("expected ref main, got %q", candidates[0].Ref)
	}
}

func TestExtractCandidates_SkipsEmptyCommitTo(t *testing.T) {
	events := []gitlab.Event{
		{
			ID:         1,
			ActionName: "pushed to",
			ProjectID:  42,
			CreatedAt:  time.Now(),
			PushData: &gitlab.PushData{
				CommitCount: 0,
				Ref:         "feature",
				RefType:     "branch",
				CommitTo:    "",
			},
		},
	}

	candidates := ExtractCandidates(events)
	if len(candidates) != 0 {
		t.Fatalf("expected 0 candidates for empty CommitTo, got %d", len(candidates))
	}
}

func TestDeduplicate(t *testing.T) {
	now := time.Now()
	candidates := []gitlab.PipelineCandidate{
		{ProjectID: 1, Ref: "main", SHA: "aaa", Reason: "pushed to", EventTime: now},
		{ProjectID: 1, Ref: "main", SHA: "aaa", Reason: "pushed to", EventTime: now.Add(-time.Hour)},
		{ProjectID: 1, Ref: "dev", SHA: "bbb", Reason: "pushed to", EventTime: now},
		{ProjectID: 2, Ref: "main", SHA: "aaa", Reason: "pushed to", EventTime: now},
	}

	result := Deduplicate(candidates)
	if len(result) != 3 {
		t.Fatalf("expected 3 unique candidates, got %d", len(result))
	}
}
