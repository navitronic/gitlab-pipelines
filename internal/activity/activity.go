package activity

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/navitronic/gitlab-pipelines/internal/gitlab"
)

const (
	dirName  = "gitlab-pipelines"
	fileName = "activity.json"
	window   = 24 * time.Hour
)

type Store struct {
	LastFetch time.Time      `json:"last_fetch"`
	Events    []gitlab.Event `json:"events"`
}

func dir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, dirName), nil
}

func filePath() (string, error) {
	d, err := dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, fileName), nil
}

func Load() (*Store, error) {
	p, err := filePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &Store{}, nil
		}
		return nil, err
	}
	var s Store
	if err := json.Unmarshal(data, &s); err != nil {
		return &Store{}, nil
	}
	s.Prune()
	return &s, nil
}

func (s *Store) Save() error {
	d, err := dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(d, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	tmp := filepath.Join(d, fileName+".tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	p, err := filePath()
	if err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// Merge adds new events, deduplicates by event ID, and prunes old entries.
func (s *Store) Merge(events []gitlab.Event) {
	seen := make(map[int]struct{}, len(s.Events))
	for _, e := range s.Events {
		seen[e.ID] = struct{}{}
	}
	for _, e := range events {
		if _, ok := seen[e.ID]; !ok {
			s.Events = append(s.Events, e)
			seen[e.ID] = struct{}{}
		}
	}
	s.LastFetch = time.Now()
	s.Prune()
}

// Prune removes events older than the 24h window.
func (s *Store) Prune() {
	cutoff := time.Now().Add(-window)
	filtered := s.Events[:0]
	for _, e := range s.Events {
		if e.CreatedAt.After(cutoff) {
			filtered = append(filtered, e)
		}
	}
	s.Events = filtered
}

// SinceTime returns the time to fetch events from.
// If we have a last fetch, use that. Otherwise, 24h ago.
func (s *Store) SinceTime() time.Time {
	if !s.LastFetch.IsZero() {
		// Subtract a day because the GitLab API uses date granularity.
		// This ensures we don't miss events from the boundary day.
		return s.LastFetch.Add(-24 * time.Hour)
	}
	return time.Now().Add(-window)
}

func Clear() {
	p, err := filePath()
	if err != nil {
		return
	}
	os.Remove(p)
}
