package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
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

func TestLoadCorrupted(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

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

	blocker := filepath.Join(tmp, "Library", "Caches", dirName)
	os.MkdirAll(filepath.Dir(blocker), 0o755)
	os.WriteFile(blocker, []byte("not a dir"), 0o644)

	err := Save([]pipeline.Pipeline{{ID: "1"}})
	if err == nil {
		t.Fatal("expected error when cache dir cannot be created")
	}
}

func TestSave_WriteError(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	d, _ := dir()
	os.MkdirAll(d, 0o755)
	os.Chmod(d, 0o555)
	defer os.Chmod(d, 0o755)

	err := Save([]pipeline.Pipeline{{ID: "1"}})
	if err == nil {
		t.Fatal("expected error when directory is read-only")
	}
}
