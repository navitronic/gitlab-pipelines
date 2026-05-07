package discovery

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/navitronic/gitlab-builds/internal/gitlab"
	"github.com/navitronic/gitlab-builds/internal/glab"
)

func TestNew(t *testing.T) {
	c := glab.New()
	d := New(c)
	if d == nil {
		t.Fatal("New() returned nil")
	}
}

func TestDiscoverSince(t *testing.T) {
	dir := t.TempDir()
	response := `[{"id":1,"action_name":"pushed to","project_id":42,"created_at":"2025-01-01T00:00:00Z","push_data":{"commit_count":1,"ref":"main","ref_type":"branch","commit_to":"abc"}}]`
	script := fakeDiscoveryScript(t, dir, response)

	c := &glab.Client{BinaryPath: script}
	d := New(c)

	events, repos, err := d.DiscoverSince(context.Background(), 1, time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if len(repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(repos))
	}
	if repos[0].ProjectID != 42 {
		t.Errorf("ProjectID = %d, want 42", repos[0].ProjectID)
	}
}

func TestDiscoverSince_Error(t *testing.T) {
	c := &glab.Client{BinaryPath: "/nonexistent/glab"}
	d := New(c)

	_, _, err := d.DiscoverSince(context.Background(), 1, time.Now().Add(-24*time.Hour))
	if err == nil {
		t.Fatal("expected error")
	}
}

func fakeDiscoveryScript(t *testing.T, dir string, response string) string {
	t.Helper()
	var script string
	if runtime.GOOS == "windows" {
		script = filepath.Join(dir, "glab.bat")
		os.WriteFile(script, []byte("@echo off\necho "+response+"\n"), 0o755)
	} else {
		script = filepath.Join(dir, "glab")
		os.WriteFile(script, []byte("#!/bin/sh\necho '"+response+"'\n"), 0o755)
	}
	return script
}

func TestExtractActiveRepos_AllEventTypes(t *testing.T) {
	now := time.Now()
	events := []gitlab.Event{
		{
			ID:         1,
			ActionName: "pushed to",
			ProjectID:  42,
			CreatedAt:  now,
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
			CreatedAt:  now.Add(-time.Minute),
		},
		{
			ID:         3,
			ActionName: "pushed to",
			ProjectID:  99,
			CreatedAt:  now.Add(-2 * time.Minute),
			PushData: &gitlab.PushData{
				CommitCount: 1,
				Ref:         "feature",
				RefType:     "branch",
				CommitTo:    "def456",
			},
		},
	}

	repos := ExtractActiveRepos(events)
	if len(repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(repos))
	}
	if repos[0].ProjectID != 42 {
		t.Errorf("expected first repo to be project 42, got %d", repos[0].ProjectID)
	}
	if repos[1].ProjectID != 99 {
		t.Errorf("expected second repo to be project 99, got %d", repos[1].ProjectID)
	}
}

func TestExtractActiveRepos_SkipsZeroProjectID(t *testing.T) {
	events := []gitlab.Event{
		{ID: 1, ActionName: "joined", ProjectID: 0, CreatedAt: time.Now()},
		{ID: 2, ActionName: "pushed to", ProjectID: 10, CreatedAt: time.Now()},
	}

	repos := ExtractActiveRepos(events)
	if len(repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(repos))
	}
	if repos[0].ProjectID != 10 {
		t.Errorf("expected project 10, got %d", repos[0].ProjectID)
	}
}

func TestExtractActiveRepos_TracksLatestActivity(t *testing.T) {
	now := time.Now()
	earlier := now.Add(-time.Hour)
	events := []gitlab.Event{
		{ID: 1, ActionName: "commented on", ProjectID: 5, CreatedAt: earlier},
		{ID: 2, ActionName: "pushed to", ProjectID: 5, CreatedAt: now},
	}

	repos := ExtractActiveRepos(events)
	if len(repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(repos))
	}
	if !repos[0].LastActive.Equal(now) {
		t.Errorf("expected LastActive=%v, got %v", now, repos[0].LastActive)
	}
}

func TestExtractActiveRepos_PreservesOrder(t *testing.T) {
	now := time.Now()
	events := []gitlab.Event{
		{ID: 1, ProjectID: 10, CreatedAt: now},
		{ID: 2, ProjectID: 20, CreatedAt: now},
		{ID: 3, ProjectID: 30, CreatedAt: now},
		{ID: 4, ProjectID: 10, CreatedAt: now},
	}

	repos := ExtractActiveRepos(events)
	if len(repos) != 3 {
		t.Fatalf("expected 3 repos, got %d", len(repos))
	}
	expected := []int{10, 20, 30}
	for i, want := range expected {
		if repos[i].ProjectID != want {
			t.Errorf("repos[%d].ProjectID = %d, want %d", i, repos[i].ProjectID, want)
		}
	}
}
