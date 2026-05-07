package gitlabsvc

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/navitronic/gitlab-builds/internal/gitlab"
	"github.com/navitronic/gitlab-builds/internal/glab"
	"github.com/navitronic/gitlab-builds/internal/pipeline"
)

func TestConvertStatus(t *testing.T) {
	tests := []struct {
		input string
		want  pipeline.Status
	}{
		{"success", pipeline.StatusPassed},
		{"failed", pipeline.StatusFailed},
		{"running", pipeline.StatusRunning},
		{"pending", pipeline.StatusPending},
		{"canceled", pipeline.StatusCanceled},
		{"skipped", pipeline.StatusSkipped},
		{"unknown", pipeline.StatusPending},
		{"", pipeline.StatusPending},
	}
	for _, tt := range tests {
		if got := convertStatus(tt.input); got != tt.want {
			t.Errorf("convertStatus(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestConvertPipeline(t *testing.T) {
	now := time.Now()
	input := gitlab.Pipeline{
		ID:        42,
		SHA:       "abc123",
		Ref:       "main",
		Status:    "success",
		Source:    "push",
		WebURL:    "https://gitlab.com/p/42",
		CreatedAt: now.Add(-10 * time.Minute),
		UpdatedAt: now,
		Duration:  125.5,
	}

	got := convertPipeline(input, 99, "group/project")

	if got.ID != "42" {
		t.Errorf("ID = %q, want \"42\"", got.ID)
	}
	if got.ProjectID != "99" {
		t.Errorf("ProjectID = %q, want \"99\"", got.ProjectID)
	}
	if got.Project != "group/project" {
		t.Errorf("Project = %q, want \"group/project\"", got.Project)
	}
	if got.SHA != "abc123" {
		t.Errorf("SHA = %q, want \"abc123\"", got.SHA)
	}
	if got.Ref != "main" {
		t.Errorf("Ref = %q, want \"main\"", got.Ref)
	}
	if got.Status != pipeline.StatusPassed {
		t.Errorf("Status = %v, want StatusPassed", got.Status)
	}
	if got.Source != "push" {
		t.Errorf("Source = %q, want \"push\"", got.Source)
	}
	if got.WebURL != "https://gitlab.com/p/42" {
		t.Errorf("WebURL = %q", got.WebURL)
	}
	expectedDur := time.Duration(125.5 * float64(time.Second))
	if got.Duration != expectedDur {
		t.Errorf("Duration = %v, want %v", got.Duration, expectedDur)
	}
}

func TestConvertJob(t *testing.T) {
	now := time.Now()
	input := gitlab.Job{
		ID:        7,
		Name:      "test",
		Stage:     "test",
		Status:    "running",
		WebURL:    "https://gitlab.com/j/7",
		CreatedAt: now.Add(-5 * time.Minute),
		StartedAt: now.Add(-2 * time.Minute),
		Duration:  60.0,
	}

	got := convertJob(input)

	if got.ID != "7" {
		t.Errorf("ID = %q, want \"7\"", got.ID)
	}
	if got.Name != "test" {
		t.Errorf("Name = %q, want \"test\"", got.Name)
	}
	if got.Stage != "test" {
		t.Errorf("Stage = %q, want \"test\"", got.Stage)
	}
	if got.Status != pipeline.StatusRunning {
		t.Errorf("Status = %v, want StatusRunning", got.Status)
	}
	if got.Duration != 60*time.Second {
		t.Errorf("Duration = %v, want 60s", got.Duration)
	}
}

func TestWrapErr(t *testing.T) {
	err := wrapErr(glab.ErrGlabNotFound)
	if !errors.Is(err, pipeline.ErrClientNotFound) {
		t.Errorf("wrapErr(ErrGlabNotFound) should wrap ErrClientNotFound, got: %v", err)
	}

	err = wrapErr(glab.ErrAuthRequired)
	if !errors.Is(err, pipeline.ErrAuthRequired) {
		t.Errorf("wrapErr(ErrAuthRequired) should wrap ErrAuthRequired, got: %v", err)
	}

	other := fmt.Errorf("something else")
	err = wrapErr(other)
	if err != other {
		t.Errorf("wrapErr(other) should pass through, got: %v", err)
	}
}
