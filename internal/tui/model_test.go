package tui

import (
	"errors"
	"testing"
	"time"

	"github.com/navitronic/gitlab-builds/internal/pipeline"
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

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		dur  time.Duration
		want string
	}{
		{0, "0s"},
		{30 * time.Second, "30s"},
		{59 * time.Second, "59s"},
		{60 * time.Second, "1m0s"},
		{90 * time.Second, "1m30s"},
		{3661 * time.Second, "61m1s"},
	}
	for _, tt := range tests {
		if got := formatDuration(tt.dur); got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.dur, got, tt.want)
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
		{"client not found", pipeline.ErrClientNotFound, "glab CLI not found. Install it: https://gitlab.com/gitlab-org/cli"},
		{"auth required", pipeline.ErrAuthRequired, "glab not authenticated. Run: glab auth login"},
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
		{6, 1},
		{7, 1},
		{8, 1},
		{11, 2},
		{24, 5},
		{40, 9},
	}
	for _, tt := range tests {
		m := Model{height: tt.height}
		if got := m.visibleItems(); got != tt.want {
			t.Errorf("visibleItems(height=%d) = %d, want %d", tt.height, got, tt.want)
		}
	}
}

func TestStatusIconCompact(t *testing.T) {
	tests := []pipeline.Status{
		pipeline.StatusPassed,
		pipeline.StatusFailed,
		pipeline.StatusRunning,
		pipeline.StatusPending,
		pipeline.StatusCanceled,
		pipeline.StatusSkipped,
	}
	for _, s := range tests {
		got := statusIconCompact(s)
		if got == "" {
			t.Errorf("statusIconCompact(%v) returned empty string", s)
		}
	}
}

func TestRenderPipelineItem(t *testing.T) {
	row := PipelineRow{
		Pipeline: pipeline.Pipeline{
			ID:        "1",
			ProjectID: "10",
			Project:   "group/project",
			SHA:       "abc123def456",
			Ref:       "main",
			Status:    pipeline.StatusPassed,
			UpdatedAt: time.Now().Add(-5 * time.Minute),
		},
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

func TestSelectPipeline(t *testing.T) {
	m := Model{
		width:  120,
		height: 24,
		pipelines: []PipelineRow{
			{Pipeline: pipeline.Pipeline{ID: "1", ProjectID: "10", Status: pipeline.StatusPassed, Project: "a/b"}},
			{Pipeline: pipeline.Pipeline{ID: "2", ProjectID: "20", Status: pipeline.StatusRunning, Project: "a/b"}},
		},
		selectedRepo: "a/b",
	}
	m.deriveRepos()

	m, _ = m.selectPipeline()
	if m.detail == nil {
		t.Fatal("detail should be set after selectPipeline")
	}
	if m.selectedID != "1" {
		t.Errorf("selectedID = %q, want \"1\"", m.selectedID)
	}

	m.cursor = 1
	m, _ = m.selectPipeline()
	if m.selectedID != "2" {
		t.Errorf("selectedID = %q, want \"2\"", m.selectedID)
	}
}

func TestDetailRender(t *testing.T) {
	d := NewDetailModel(PipelineRow{
		Pipeline: pipeline.Pipeline{
			ID:      "42",
			Project: "group/project",
			SHA:     "abc123",
			Ref:     "main",
			Status:  pipeline.StatusPassed,
			Source:  "push",
		},
	})

	got := d.Render(60, 24)
	if got == "" {
		t.Fatal("DetailModel.Render returned empty string")
	}
}

func TestReposPaneWidth(t *testing.T) {
	tests := []struct {
		width int
	}{
		{120},
		{100},
		{80},
		{60},
	}
	for _, tt := range tests {
		m := Model{width: tt.width}
		repoW := m.reposPaneWidth()
		if repoW <= 0 || repoW > tt.width {
			t.Errorf("reposPaneWidth(width=%d) = %d, out of range", tt.width, repoW)
		}
	}
}

func TestLayoutMode(t *testing.T) {
	tests := []struct {
		width int
		want  layoutMode
	}{
		{120, layoutThree},
		{140, layoutThree},
		{100, layoutTwo},
		{80, layoutTwo},
		{79, layoutOne},
		{60, layoutOne},
	}
	for _, tt := range tests {
		m := Model{width: tt.width}
		if got := m.layout(); got != tt.want {
			t.Errorf("layout(width=%d) = %d, want %d", tt.width, got, tt.want)
		}
	}
}

func TestDeriveRepos(t *testing.T) {
	m := Model{
		pipelines: []PipelineRow{
			{Pipeline: pipeline.Pipeline{ID: "1", Project: "group/project-b"}},
			{Pipeline: pipeline.Pipeline{ID: "2", Project: "group/project-a"}},
			{Pipeline: pipeline.Pipeline{ID: "3", Project: "group/project-b"}},
		},
	}
	m.deriveRepos()

	if len(m.repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(m.repos))
	}
	if m.repos[0] != "group/project-a" {
		t.Errorf("repos[0] = %q, want group/project-a", m.repos[0])
	}
	if m.repos[1] != "group/project-b" {
		t.Errorf("repos[1] = %q, want group/project-b", m.repos[1])
	}
	if m.selectedRepo != "group/project-a" {
		t.Errorf("selectedRepo = %q, want group/project-a", m.selectedRepo)
	}
}

func TestRepoShortName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"group/project", "group/project"},
		{"org/group/project", "group/project"},
		{"a/b/c/d", "c/d"},
		{"project", "project"},
	}
	for _, tt := range tests {
		if got := repoShortName(tt.input); got != tt.want {
			t.Errorf("repoShortName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestDetailTickingPreventsStackedChains(t *testing.T) {
	m := Model{
		width:  120,
		height: 24,
		pipelines: []PipelineRow{
			{Pipeline: pipeline.Pipeline{ID: "1", ProjectID: "10", Project: "a/b", Status: pipeline.StatusRunning}},
		},
		selectedRepo: "a/b",
	}
	m.deriveRepos()
	m, _ = m.selectPipeline()

	activeJobs := []pipeline.Job{
		{ID: "1", Name: "build", Status: pipeline.StatusRunning, Stage: "build"},
	}

	result, cmd := m.Update(JobsLoadedMsg{Jobs: activeJobs})
	m = result.(Model)
	if !m.detailTicking {
		t.Fatal("detailTicking should be true after first JobsLoadedMsg with active jobs")
	}
	if cmd == nil {
		t.Fatal("expected a tick cmd from first JobsLoadedMsg")
	}

	result, cmd = m.Update(JobsLoadedMsg{Jobs: activeJobs})
	m = result.(Model)
	if cmd != nil {
		t.Fatal("second JobsLoadedMsg should not spawn another tick chain")
	}
	if !m.detailTicking {
		t.Fatal("detailTicking should remain true")
	}
}

func TestDetailTickingClearsWhenNoActiveJobs(t *testing.T) {
	m := Model{
		width:         120,
		height:        24,
		detailTicking: true,
		detail: NewDetailModel(PipelineRow{
			Pipeline: pipeline.Pipeline{ID: "1", ProjectID: "10", Project: "a/b", Status: pipeline.StatusPassed},
		}),
	}
	m.detail.SetJobs([]pipeline.Job{
		{ID: "1", Name: "build", Status: pipeline.StatusPassed, Stage: "build"},
	}, nil)

	result, cmd := m.Update(detailTickMsg{})
	m = result.(Model)
	if m.detailTicking {
		t.Fatal("detailTicking should be false when no active jobs")
	}
	if cmd != nil {
		t.Fatal("should not schedule another tick when no active jobs")
	}
}

func TestDetailTickingResetsOnPipelineChange(t *testing.T) {
	m := Model{
		width:  120,
		height: 24,
		pipelines: []PipelineRow{
			{Pipeline: pipeline.Pipeline{ID: "1", ProjectID: "10", Project: "a/b", Status: pipeline.StatusRunning}},
			{Pipeline: pipeline.Pipeline{ID: "2", ProjectID: "10", Project: "a/b", Status: pipeline.StatusPassed}},
		},
		selectedRepo:  "a/b",
		detailTicking: true,
	}
	m.deriveRepos()
	m.selectedID = "1"
	m.cursor = 1

	m, _ = m.selectPipeline()
	if m.detailTicking {
		t.Fatal("detailTicking should reset when selecting a new pipeline")
	}
}
