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

func TestDefaultColumns(t *testing.T) {
	tests := []struct {
		width    int
		wantCols int
	}{
		{60, 5},
		{79, 5},
		{80, 7},
		{119, 7},
		{120, 8},
		{200, 8},
	}
	for _, tt := range tests {
		cols := defaultColumns(tt.width)
		if len(cols) != tt.wantCols {
			t.Errorf("defaultColumns(%d) = %d columns, want %d", tt.width, len(cols), tt.wantCols)
		}
	}
}

func TestBuildRows_ColumnCount(t *testing.T) {
	pipelines := []PipelineRow{
		{
			Pipeline: gitlab.Pipeline{
				ID:     1,
				SHA:    "abc123def456",
				Ref:    "main",
				Status: "success",
				Source: "push",
			},
			ProjectPath: "group/project",
		},
	}

	tests := []struct {
		width    int
		wantCols int
	}{
		{60, 5},
		{100, 7},
		{150, 8},
	}
	for _, tt := range tests {
		rows := buildRows(pipelines, tt.width)
		if len(rows) != 1 {
			t.Fatalf("buildRows(width=%d) returned %d rows, want 1", tt.width, len(rows))
		}
		if len(rows[0]) != tt.wantCols {
			t.Errorf("buildRows(width=%d) row has %d cells, want %d", tt.width, len(rows[0]), tt.wantCols)
		}
	}
}
