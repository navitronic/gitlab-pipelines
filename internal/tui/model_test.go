package tui

import (
	"errors"
	"testing"
	"time"

	"github.com/navitronic/gitlab-builds/internal/gitlab"
	"github.com/navitronic/gitlab-builds/internal/glab"
)

func TestShortSHA(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"abc123def456", "abc123de"},
		{"abc", "abc"},
		{"12345678", "12345678"},
		{"123456789", "12345678"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := shortSHA(tt.input); got != tt.want {
			t.Errorf("shortSHA(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input string
		max   int
		want  string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 5, "hell…"},
		{"ab", 2, "ab"},
		{"abc", 2, "a…"},
		{"", 5, ""},
	}
	for _, tt := range tests {
		if got := truncate(tt.input, tt.max); got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		seconds float64
		want    string
	}{
		{0, "0s"},
		{30, "30s"},
		{59.9, "60s"},
		{60, "1m0s"},
		{90, "1m30s"},
		{3661, "61m1s"},
	}
	for _, tt := range tests {
		if got := formatDuration(tt.seconds); got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.seconds, got, tt.want)
		}
	}
}

func TestFormatTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name string
		time time.Time
		want string
	}{
		{"zero", time.Time{}, ""},
		{"just now", now.Add(-10 * time.Second), "just now"},
		{"minutes ago", now.Add(-5 * time.Minute), "5m ago"},
		{"hours ago", now.Add(-3 * time.Hour), "3h ago"},
	}
	for _, tt := range tests {
		if got := formatTime(tt.time); got != tt.want {
			t.Errorf("formatTime(%s) = %q, want %q", tt.name, got, tt.want)
		}
	}

	old := now.Add(-48 * time.Hour)
	got := formatTime(old)
	if got == "" || got == "just now" {
		t.Errorf("formatTime(48h ago) = %q, expected date format", got)
	}
}

func TestFormatError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"glab not found", glab.ErrGlabNotFound, "glab CLI not found. Install it: https://gitlab.com/gitlab-org/cli"},
		{"auth required", glab.ErrAuthRequired, "glab not authenticated. Run: glab auth login"},
		{"other error", errors.New("some error"), "some error"},
	}
	for _, tt := range tests {
		got := formatError(tt.err)
		if got.Error() != tt.want {
			t.Errorf("formatError(%s) = %q, want %q", tt.name, got.Error(), tt.want)
		}
	}
}

func TestVisibleItems(t *testing.T) {
	tests := []struct {
		height int
		want   int
	}{
		{3, 1},
		{5, 1},
		{7, 1},
		{8, 1},
		{9, 2},
		{24, 7},
		{40, 12},
	}
	for _, tt := range tests {
		m := Model{height: tt.height}
		if got := m.visibleItems(); got != tt.want {
			t.Errorf("visibleItems(height=%d) = %d, want %d", tt.height, got, tt.want)
		}
	}
}

func TestStatusIconCompact(t *testing.T) {
	tests := []struct {
		status string
	}{
		{"success"},
		{"failed"},
		{"running"},
		{"pending"},
		{"canceled"},
		{"skipped"},
		{"unknown"},
	}
	for _, tt := range tests {
		got := statusIconCompact(tt.status)
		if got == "" {
			t.Errorf("statusIconCompact(%q) returned empty string", tt.status)
		}
	}
}

func TestRenderPipelineItem(t *testing.T) {
	row := PipelineRow{
		Pipeline: gitlab.Pipeline{
			ID:        1,
			SHA:       "abc123def456",
			Ref:       "main",
			Status:    "success",
			UpdatedAt: time.Now().Add(-5 * time.Minute),
		},
		ProjectPath: "group/project",
	}

	got := renderPipelineItem(row, 80, false)
	if got == "" {
		t.Fatal("renderPipelineItem returned empty string")
	}

	selected := renderPipelineItem(row, 80, true)
	if selected == "" {
		t.Fatal("renderPipelineItem (selected) returned empty string")
	}
}
