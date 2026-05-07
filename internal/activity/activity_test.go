package activity

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/navitronic/gitlab-builds/internal/gitlab"
)

func TestMerge(t *testing.T) {
	s := &Store{}
	now := time.Now()

	events := []gitlab.Event{
		{ID: 1, ProjectID: 10, CreatedAt: now.Add(-1 * time.Hour)},
		{ID: 2, ProjectID: 20, CreatedAt: now.Add(-2 * time.Hour)},
	}
	s.Merge(events)

	if len(s.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(s.Events))
	}
	if s.LastFetch.IsZero() {
		t.Fatal("LastFetch should be set after Merge")
	}
}

func TestMergeDeduplicates(t *testing.T) {
	now := time.Now()
	s := &Store{
		Events: []gitlab.Event{
			{ID: 1, ProjectID: 10, CreatedAt: now.Add(-1 * time.Hour)},
		},
	}

	events := []gitlab.Event{
		{ID: 1, ProjectID: 10, CreatedAt: now.Add(-1 * time.Hour)},
		{ID: 2, ProjectID: 20, CreatedAt: now.Add(-2 * time.Hour)},
	}
	s.Merge(events)

	if len(s.Events) != 2 {
		t.Fatalf("expected 2 events after dedup, got %d", len(s.Events))
	}
}

func TestPrune(t *testing.T) {
	now := time.Now()
	s := &Store{
		Events: []gitlab.Event{
			{ID: 1, ProjectID: 10, CreatedAt: now.Add(-1 * time.Hour)},
			{ID: 2, ProjectID: 20, CreatedAt: now.Add(-25 * time.Hour)},
			{ID: 3, ProjectID: 30, CreatedAt: now.Add(-48 * time.Hour)},
		},
	}
	s.Prune()

	if len(s.Events) != 1 {
		t.Fatalf("expected 1 event after prune, got %d", len(s.Events))
	}
	if s.Events[0].ID != 1 {
		t.Errorf("expected event ID 1, got %d", s.Events[0].ID)
	}
}

func TestSinceTimeWithLastFetch(t *testing.T) {
	lastFetch := time.Now().Add(-6 * time.Hour)
	s := &Store{LastFetch: lastFetch}

	since := s.SinceTime()
	expected := lastFetch.Add(-24 * time.Hour)
	diff := since.Sub(expected)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("SinceTime() = %v, want ~%v", since, expected)
	}
}

func TestSinceTimeWithoutLastFetch(t *testing.T) {
	s := &Store{}
	since := s.SinceTime()
	expected := time.Now().Add(-24 * time.Hour)
	diff := since.Sub(expected)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("SinceTime() = %v, want ~%v", since, expected)
	}
}

func TestLoadSaveRoundtrip(t *testing.T) {
	tmp := t.TempDir()

	storePath := filepath.Join(tmp, "activity.json")
	now := time.Now().Truncate(time.Second)
	s := &Store{
		LastFetch: now,
		Events: []gitlab.Event{
			{ID: 1, ProjectID: 10, CreatedAt: now.Add(-1 * time.Hour)},
		},
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(storePath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	var loaded Store
	raw, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &loaded); err != nil {
		t.Fatal(err)
	}

	if loaded.LastFetch.Unix() != now.Unix() {
		t.Errorf("LastFetch = %v, want %v", loaded.LastFetch, now)
	}
	if len(loaded.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(loaded.Events))
	}
}

func TestLoadMissingFile(t *testing.T) {
	tmp := t.TempDir()
	orig := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", orig)

	s, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if s == nil {
		t.Fatal("Load() returned nil store")
	}
	if len(s.Events) != 0 {
		t.Errorf("expected 0 events, got %d", len(s.Events))
	}
}

func TestLoadCorruptedFile(t *testing.T) {
	tmp := t.TempDir()
	orig := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", orig)

	cacheDir := filepath.Join(tmp, "Library", "Caches", dirName)
	os.MkdirAll(cacheDir, 0o755)
	os.WriteFile(filepath.Join(cacheDir, fileName), []byte("not json"), 0o644)

	s, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error for corrupted file: %v", err)
	}
	if s == nil {
		t.Fatal("Load() returned nil store for corrupted file")
	}
	if len(s.Events) != 0 {
		t.Errorf("expected 0 events for corrupted file, got %d", len(s.Events))
	}
}

func TestClear(t *testing.T) {
	tmp := t.TempDir()
	orig := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", orig)

	cacheDir := filepath.Join(tmp, "Library", "Caches", dirName)
	os.MkdirAll(cacheDir, 0o755)
	p := filepath.Join(cacheDir, fileName)
	os.WriteFile(p, []byte(`{}`), 0o644)

	Clear()

	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Error("Clear() did not remove the file")
	}
}

func TestSave(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	now := time.Now()
	s := &Store{
		LastFetch: now,
		Events: []gitlab.Event{
			{ID: 1, ProjectID: 10, CreatedAt: now.Add(-1 * time.Hour)},
		},
	}
	if err := s.Save(); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() after Save() returned error: %v", err)
	}
	if len(loaded.Events) != 1 {
		t.Errorf("expected 1 event after Save/Load, got %d", len(loaded.Events))
	}
}

func TestSave_MkdirError(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	blocker := filepath.Join(tmp, "Library", "Caches", dirName)
	os.MkdirAll(filepath.Dir(blocker), 0o755)
	os.WriteFile(blocker, []byte("not a dir"), 0o644)

	s := &Store{LastFetch: time.Now()}
	err := s.Save()
	if err == nil {
		t.Fatal("expected error when cache dir cannot be created")
	}
}

func TestSave_WriteError(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	d := filepath.Join(tmp, "Library", "Caches", dirName)
	os.MkdirAll(d, 0o755)
	os.Chmod(d, 0o555)
	defer os.Chmod(d, 0o755)

	s := &Store{LastFetch: time.Now()}
	err := s.Save()
	if err == nil {
		t.Fatal("expected error when directory is read-only")
	}
}
