package cache

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/navitronic/gitlab-builds/internal/pipeline"
)

func TestSaveAndLoad(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmp)
	// On macOS, os.UserCacheDir() uses ~/Library/Caches and ignores XDG.
	// Override via our dir() by setting HOME so UserCacheDir resolves to tmp.
	if cacheDir, err := os.UserCacheDir(); err == nil && cacheDir != tmp {
		t.Setenv("HOME", tmp)
	}

	pipelines := []pipeline.Pipeline{
		{
			ID:        "123",
			ProjectID: "1",
			Project:   "group/project",
			Ref:       "main",
			SHA:       "abc123def",
			Status:    pipeline.StatusPassed,
			Source:    "push",
			WebURL:    "https://gitlab.com/p/123",
			CreatedAt: time.Now().Add(-10 * time.Minute),
			UpdatedAt: time.Now().Add(-5 * time.Minute),
			Duration:  90 * time.Second,
		},
	}

	if err := Save(pipelines); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 pipeline, got %d", len(loaded))
	}
	if loaded[0].ID != "123" {
		t.Errorf("expected ID=123, got %s", loaded[0].ID)
	}
	if loaded[0].Project != "group/project" {
		t.Errorf("expected Project=group/project, got %s", loaded[0].Project)
	}
	if loaded[0].Status != pipeline.StatusPassed {
		t.Errorf("expected StatusPassed, got %v", loaded[0].Status)
	}
}

func TestLoadMissing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	loaded, err := Load()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if loaded != nil {
		t.Fatalf("expected nil pipelines, got %v", loaded)
	}
}

func TestLoadExpired(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	pipelines := []pipeline.Pipeline{
		{ID: "1", Status: pipeline.StatusRunning},
	}
	if err := Save(pipelines); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Backdate the cache file modification time beyond TTL.
	p, _ := filePath()
	old := time.Now().Add(-2 * time.Hour)
	os.Chtimes(p, old, old)

	// Load reads cached_at from JSON, not mtime, so we need to manipulate the JSON.
	// Instead, write an entry with an old CachedAt directly.
	e := entry{
		CachedAt:  time.Now().Add(-2 * time.Hour),
		Pipelines: pipelines,
	}
	data, _ := json.Marshal(e)
	os.WriteFile(p, data, 0o644)

	loaded, err := Load()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if loaded != nil {
		t.Fatalf("expected nil (expired), got %v", loaded)
	}
}
