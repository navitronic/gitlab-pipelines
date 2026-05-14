package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/navitronic/gitlab-pipelines/internal/pipeline"
)

func TestSaveAndLoad(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "cache"))

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
	t.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "cache"))

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
	t.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "cache"))

	pipelines := []pipeline.Pipeline{
		{ID: "1", Status: pipeline.StatusRunning},
	}
	if err := Save(pipelines); err != nil {
		t.Fatalf("Save: %v", err)
	}

	p, _ := filePath()
	old := time.Now().Add(-2 * time.Hour)
	os.Chtimes(p, old, old)

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

func TestLoadVersionMismatch(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "cache"))

	p, _ := filePath()
	d, _ := dir()
	os.MkdirAll(d, 0o755)

	e := entry{
		Version:   "old-sha-that-does-not-match",
		CachedAt:  time.Now(),
		Pipelines: []pipeline.Pipeline{{ID: "1", Status: pipeline.StatusPassed}},
	}
	data, _ := json.Marshal(e)
	os.WriteFile(p, data, 0o644)

	loaded, err := Load()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	// buildVersion() returns "" in test binaries (no VCS info),
	// so the version check is skipped and data loads normally.
	if loaded == nil {
		t.Fatal("expected cached data (version check skipped in tests)")
	}
}

func TestLoadCorrupted(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "cache"))

	if err := Save([]pipeline.Pipeline{{ID: "1"}}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	p, _ := filePath()
	os.WriteFile(p, []byte("not json"), 0o644)

	loaded, err := Load()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if loaded != nil {
		t.Fatalf("expected nil for corrupted, got %v", loaded)
	}
}

func TestClear(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "cache"))

	if err := Save([]pipeline.Pipeline{{ID: "1"}}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	p, _ := filePath()
	if _, err := os.Stat(p); os.IsNotExist(err) {
		t.Fatal("file should exist after Save")
	}

	Clear()

	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Error("Clear() did not remove the file")
	}
}

func TestSave_MkdirError(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "cache"))

	d, _ := dir()
	os.MkdirAll(filepath.Dir(d), 0o755)
	os.WriteFile(d, []byte("not a dir"), 0o644)

	err := Save([]pipeline.Pipeline{{ID: "1"}})
	if err == nil {
		t.Fatal("expected error when cache dir cannot be created")
	}
}

func TestSave_WriteError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}

	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "cache"))

	d, _ := dir()
	os.MkdirAll(d, 0o755)
	os.Chmod(d, 0o555)
	defer os.Chmod(d, 0o755)

	err := Save([]pipeline.Pipeline{{ID: "1"}})
	if err == nil {
		t.Fatal("expected error when directory is read-only")
	}
}
