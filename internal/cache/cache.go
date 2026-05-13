package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/navitronic/gitlab-pipelines/internal/pipeline"
)

const (
	dirName  = "gitlab-pipelines"
	fileName = "pipelines.json"
	ttl      = 1 * time.Hour
)

type entry struct {
	CachedAt  time.Time           `json:"cached_at"`
	Pipelines []pipeline.Pipeline `json:"pipelines"`
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

// Load returns cached pipelines if the cache exists and is within TTL.
// Returns nil, nil if the cache is missing or expired.
func Load() ([]pipeline.Pipeline, error) {
	p, err := filePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var e entry
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, nil
	}
	if time.Since(e.CachedAt) > ttl {
		return nil, nil
	}
	return e.Pipelines, nil
}

// Clear removes the cache file.
func Clear() {
	p, err := filePath()
	if err != nil {
		return
	}
	os.Remove(p)
}

// Save writes pipelines to the cache file atomically.
func Save(pipelines []pipeline.Pipeline) error {
	d, err := dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(d, 0o755); err != nil {
		return err
	}
	e := entry{
		CachedAt:  time.Now(),
		Pipelines: pipelines,
	}
	data, err := json.Marshal(e)
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
