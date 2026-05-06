package pipeline

import (
	"context"
	"errors"
	"time"
)

// Sentinel errors for user-facing messages.
var (
	ErrClientNotFound = errors.New("pipeline client not found")
	ErrAuthRequired   = errors.New("authentication required")
)

type Status int

const (
	StatusPending Status = iota
	StatusRunning
	StatusPassed
	StatusFailed
	StatusCanceled
	StatusSkipped
)

func (s Status) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusRunning:
		return "running"
	case StatusPassed:
		return "passed"
	case StatusFailed:
		return "failed"
	case StatusCanceled:
		return "canceled"
	case StatusSkipped:
		return "skipped"
	default:
		return "unknown"
	}
}

type Pipeline struct {
	ID        string
	ProjectID string
	Project   string
	Ref       string
	SHA       string
	Status    Status
	Source    string
	WebURL    string
	CreatedAt time.Time
	UpdatedAt time.Time
	Duration  time.Duration
}

type Job struct {
	ID        string
	Name      string
	Stage     string
	Status    Status
	WebURL    string
	CreatedAt time.Time
	StartedAt time.Time
	Duration  time.Duration
}

type Service interface {
	ListPipelines(ctx context.Context, progress func(string)) ([]Pipeline, error)
	GetPipeline(ctx context.Context, projectID string, id string) (Pipeline, error)
	ListJobs(ctx context.Context, projectID string, pipelineID string) ([]Job, error)
	GetMergeRequestURL(ctx context.Context, projectID string, ref string) (string, error)
}
