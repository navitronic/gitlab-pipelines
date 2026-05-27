package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/navitronic/gitlab-pipelines/internal/pipeline"
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

	got := renderPipelineItem(row, 80, false, true)
	if got == "" {
		t.Fatal("renderPipelineItem returned empty string")
	}

	selected := renderPipelineItem(row, 80, true, true)
	if selected == "" {
		t.Fatal("renderPipelineItem (selected) returned empty string")
	}
}

func TestRenderPipelineItem_NoProject(t *testing.T) {
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

	compact := renderPipelineItem(row, 80, false, false)
	full := renderPipelineItem(row, 80, false, true)

	if strings.Contains(compact, row.Pipeline.Project) {
		t.Error("expected no project name when showProject=false")
	}
	if !strings.Contains(compact, "abc123de") || !strings.Contains(compact, "main") {
		t.Error("expected compact rendering to include SHA and ref")
	}
	if lipgloss.Height(compact) >= lipgloss.Height(full) {
		t.Errorf("expected compact item to be shorter than full item, got compact=%d full=%d", lipgloss.Height(compact), lipgloss.Height(full))
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
	}

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

func TestLayoutMode(t *testing.T) {
	tests := []struct {
		width int
		want  layoutMode
	}{
		{120, layoutTwo},
		{140, layoutTwo},
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

func TestDetailTickingPreventsStackedChains(t *testing.T) {
	m := Model{
		width:  120,
		height: 24,
		pipelines: []PipelineRow{
			{Pipeline: pipeline.Pipeline{ID: "1", ProjectID: "10", Project: "a/b", Status: pipeline.StatusRunning}},
		},
	}
	m, _ = m.selectPipeline()

	activeJobs := []pipeline.Job{
		{ID: "1", Name: "build", Status: pipeline.StatusRunning, Stage: "build"},
	}

	result, cmd := m.Update(JobsLoadedMsg{PipelineID: "1", Jobs: activeJobs})
	m = result.(Model)
	if !m.detailTicking {
		t.Fatal("detailTicking should be true after first JobsLoadedMsg with active jobs")
	}
	if cmd == nil {
		t.Fatal("expected a tick cmd from first JobsLoadedMsg")
	}

	result, cmd = m.Update(JobsLoadedMsg{PipelineID: "1", Jobs: activeJobs})
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
		detailTicking: true,
	}
	m.selectedID = "1"
	m.cursor = 1

	m, _ = m.selectPipeline()
	if m.detailTicking {
		t.Fatal("detailTicking should reset when selecting a new pipeline")
	}
}

func testModel() Model {
	m := New()
	m.width = 120
	m.height = 24
	m.loading = false
	m.pipelines = []PipelineRow{
		{Pipeline: pipeline.Pipeline{ID: "1", ProjectID: "10", Project: "group/alpha", Status: pipeline.StatusPassed, Ref: "main", SHA: "abc12345", UpdatedAt: time.Now()}},
		{Pipeline: pipeline.Pipeline{ID: "2", ProjectID: "10", Project: "group/alpha", Status: pipeline.StatusRunning, Ref: "feat", SHA: "def67890", UpdatedAt: time.Now()}},
		{Pipeline: pipeline.Pipeline{ID: "3", ProjectID: "20", Project: "group/beta", Status: pipeline.StatusFailed, Ref: "main", SHA: "fff11111", UpdatedAt: time.Now()}},
	}
	m, _ = m.selectPipeline()
	return m
}

func TestNavigateDownInPipelines(t *testing.T) {
	m := testModel()
	m.focus = PanePipelines

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = result.(Model)

	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.cursor)
	}
	if m.selectedID != "2" {
		t.Errorf("selectedID = %q, want \"2\"", m.selectedID)
	}
}

func TestNavigateUpInPipelines(t *testing.T) {
	m := testModel()
	m.focus = PanePipelines
	m.cursor = 1
	m.selectedID = "2"

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = result.(Model)

	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.cursor)
	}
	if m.selectedID != "1" {
		t.Errorf("selectedID = %q, want \"1\"", m.selectedID)
	}
}

func TestLeftRightNavigation(t *testing.T) {
	m := testModel()
	m.focus = PanePipelines

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = result.(Model)
	if m.focus != PaneDetail {
		t.Errorf("focus = %d, want PaneDetail", m.focus)
	}

	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = result.(Model)
	if m.focus != PaneDetail {
		t.Errorf("focus should stay at PaneDetail, got %d", m.focus)
	}

	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = result.(Model)
	if m.focus != PanePipelines {
		t.Errorf("focus = %d, want PanePipelines", m.focus)
	}

	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = result.(Model)
	if m.focus != PanePipelines {
		t.Errorf("focus should stay at PanePipelines, got %d", m.focus)
	}
}

func TestViewLoadingState(t *testing.T) {
	m := New()
	m.width = 120
	m.height = 24
	m.loading = true

	view := m.View()
	if view == "" {
		t.Fatal("View returned empty string in loading state")
	}
}

func TestViewErrorState(t *testing.T) {
	m := New()
	m.width = 120
	m.height = 24
	m.loading = false
	m.err = errors.New("connection failed")

	view := m.View()
	if view == "" {
		t.Fatal("View returned empty string in error state")
	}
}

func TestViewEmptyState(t *testing.T) {
	m := New()
	m.width = 120
	m.height = 24
	m.loading = false

	view := m.View()
	if view == "" {
		t.Fatal("View returned empty string in empty state")
	}
}

func TestViewTwoPaneLayout(t *testing.T) {
	m := testModel()
	view := m.View()
	if view == "" {
		t.Fatal("View returned empty string in 2-pane layout")
	}
}

func TestViewOnePaneLayout(t *testing.T) {
	m := testModel()
	m.width = 60
	view := m.View()
	if view == "" {
		t.Fatal("View returned empty string in 1-pane layout")
	}
}

func TestViewOnePaneAllFocusStates(t *testing.T) {
	m := testModel()
	m.width = 60

	for _, pane := range []Pane{PanePipelines, PaneDetail} {
		m.focus = pane
		view := m.View()
		if view == "" {
			t.Fatalf("View returned empty string with focus=%d in 1-pane layout", pane)
		}
	}
}

func TestPipelinesLoadedMsg(t *testing.T) {
	m := New()
	m.width = 120
	m.height = 24

	rows := []PipelineRow{
		{Pipeline: pipeline.Pipeline{ID: "1", ProjectID: "10", Project: "a/b", Status: pipeline.StatusPassed}},
		{Pipeline: pipeline.Pipeline{ID: "2", ProjectID: "20", Project: "c/d", Status: pipeline.StatusRunning}},
	}

	result, _ := m.Update(PipelinesLoadedMsg{Pipelines: rows})
	m = result.(Model)

	if m.loading {
		t.Error("should not be loading after PipelinesLoadedMsg")
	}
	if len(m.pipelines) != 2 {
		t.Errorf("expected 2 pipelines, got %d", len(m.pipelines))
	}
	if m.detail == nil {
		t.Error("detail should be set after loading pipelines")
	}
}

func TestPipelinesLoadedMsgWithError(t *testing.T) {
	m := New()
	m.width = 120
	m.height = 24

	result, _ := m.Update(PipelinesLoadedMsg{Err: errors.New("network error")})
	m = result.(Model)

	if m.loading {
		t.Error("should not be loading after error")
	}
	if m.err == nil {
		t.Error("err should be set")
	}
}

func TestWindowSizeMsg(t *testing.T) {
	m := New()
	result, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	m = result.(Model)

	if m.width != 200 {
		t.Errorf("width = %d, want 200", m.width)
	}
	if m.height != 50 {
		t.Errorf("height = %d, want 50", m.height)
	}
}

func TestStatusIcons(t *testing.T) {
	statuses := []pipeline.Status{
		pipeline.StatusPassed,
		pipeline.StatusFailed,
		pipeline.StatusRunning,
		pipeline.StatusPending,
		pipeline.StatusCanceled,
		pipeline.StatusSkipped,
	}
	for _, s := range statuses {
		if statusIcon(s) == "" {
			t.Errorf("statusIcon(%v) returned empty", s)
		}
		if statusIconCompact(s) == "" {
			t.Errorf("statusIconCompact(%v) returned empty", s)
		}
		if statusIconRaw(s) == "" {
			t.Errorf("statusIconRaw(%v) returned empty", s)
		}
	}
}

func TestRenderPipelineItemWidths(t *testing.T) {
	row := PipelineRow{
		Pipeline: pipeline.Pipeline{
			ID:        "1",
			ProjectID: "10",
			Project:   "group/project",
			SHA:       "abc123def456",
			Ref:       "main",
			Status:    pipeline.StatusPassed,
			UpdatedAt: time.Now(),
		},
	}

	for _, width := range []int{40, 60, 80, 120} {
		got := renderPipelineItem(row, width, false, true)
		if got == "" {
			t.Errorf("renderPipelineItem(width=%d, selected=false) returned empty", width)
		}
		got = renderPipelineItem(row, width, true, true)
		if got == "" {
			t.Errorf("renderPipelineItem(width=%d, selected=true) returned empty", width)
		}
	}
}

func TestEscFromDetail(t *testing.T) {
	m := testModel()
	m.focus = PaneDetail

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = result.(Model)

	if m.focus != PanePipelines {
		t.Errorf("focus = %d after Esc, want PanePipelines", m.focus)
	}
}

func TestEnterInPipelines_OnePane(t *testing.T) {
	m := testModel()
	m.width = 60
	m.focus = PanePipelines

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = result.(Model)

	if m.focus != PaneDetail {
		t.Errorf("focus = %d, want PaneDetail after Enter in 1-pane pipelines view", m.focus)
	}
}

func TestRefreshKey(t *testing.T) {
	m := testModel()
	called := false
	m.Refresh = func() tea.Cmd {
		called = true
		return nil
	}

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = result.(Model)

	if !called {
		t.Error("Refresh callback was not called")
	}
	if !m.loading {
		t.Error("model should be in loading state after refresh")
	}
}

func TestHardRefreshKey(t *testing.T) {
	m := testModel()
	called := false
	m.HardRefresh = func() tea.Cmd {
		called = true
		return nil
	}

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	m = result.(Model)

	if !called {
		t.Error("HardRefresh callback was not called")
	}
	if !m.loading {
		t.Error("model should be in loading state after hard refresh")
	}
}

func TestTruncateStr(t *testing.T) {
	tests := []struct {
		input    string
		maxWidth int
		fits     bool
	}{
		{"short", 10, true},
		{"this is a longer string", 10, false},
		{"exact", 5, true},
		{"x", 1, true},
	}
	for _, tt := range tests {
		got := truncateStr(tt.input, tt.maxWidth)
		if got == "" {
			t.Errorf("truncateStr(%q, %d) returned empty", tt.input, tt.maxWidth)
		}
	}
}

func TestDetailSetMRURL(t *testing.T) {
	d := NewDetailModel(PipelineRow{
		Pipeline: pipeline.Pipeline{ID: "1", Project: "a/b", Ref: "main", Status: pipeline.StatusPassed},
	})
	d.SetMRURL("https://gitlab.com/mr/5")
	got := d.Render(80, 24)
	if !contains(got, "https://gitlab.com/mr/5") {
		t.Error("MR URL not rendered in detail")
	}
}

func TestDetailTick(t *testing.T) {
	d := NewDetailModel(PipelineRow{
		Pipeline: pipeline.Pipeline{ID: "1", Project: "a/b", Ref: "main", Status: pipeline.StatusRunning},
	})
	d.SetJobs([]pipeline.Job{
		{ID: "1", Name: "build", Stage: "build", Status: pipeline.StatusRunning},
	}, nil)
	frame0 := d.frame
	d.Tick()
	if d.frame == frame0 {
		t.Error("Tick did not advance frame")
	}
}

func TestDetailRenderJobs(t *testing.T) {
	d := NewDetailModel(PipelineRow{
		Pipeline: pipeline.Pipeline{ID: "1", Project: "a/b", Ref: "main", Status: pipeline.StatusPassed, Duration: 120 * time.Second},
	})
	d.SetJobs([]pipeline.Job{
		{ID: "1", Name: "build", Stage: "build", Status: pipeline.StatusPassed, Duration: 30 * time.Second},
		{ID: "2", Name: "test", Stage: "test", Status: pipeline.StatusFailed, Duration: 45 * time.Second},
		{ID: "3", Name: "deploy", Stage: "deploy", Status: pipeline.StatusRunning, StartedAt: time.Now().Add(-10 * time.Second)},
		{ID: "4", Name: "lint", Stage: "test", Status: pipeline.StatusPending, CreatedAt: time.Now().Add(-5 * time.Second)},
		{ID: "5", Name: "cleanup", Stage: "deploy", Status: pipeline.StatusCanceled},
		{ID: "6", Name: "skip", Stage: "deploy", Status: pipeline.StatusSkipped},
	}, nil)

	got := d.Render(80, 40)
	if got == "" {
		t.Fatal("Render returned empty with jobs")
	}
	if !contains(got, "build") {
		t.Error("expected 'build' stage in output")
	}
	if !contains(got, "test") {
		t.Error("expected 'test' stage in output")
	}
}

func TestDetailRenderJobsError(t *testing.T) {
	d := NewDetailModel(PipelineRow{
		Pipeline: pipeline.Pipeline{ID: "1", Project: "a/b", Ref: "main", Status: pipeline.StatusPassed},
	})
	d.SetJobs(nil, errors.New("fetch failed"))

	got := d.Render(80, 24)
	if !contains(got, "Error loading jobs") {
		t.Error("expected error message in output")
	}
}

func TestDetailRenderNoJobs(t *testing.T) {
	d := NewDetailModel(PipelineRow{
		Pipeline: pipeline.Pipeline{ID: "1", Project: "a/b", Ref: "main", Status: pipeline.StatusPassed},
	})
	d.SetJobs([]pipeline.Job{}, nil)

	got := d.Render(80, 24)
	if !contains(got, "No jobs found") {
		t.Error("expected 'No jobs found' message")
	}
}

func TestPipelineDurationRunning(t *testing.T) {
	d := NewDetailModel(PipelineRow{
		Pipeline: pipeline.Pipeline{
			ID:        "1",
			Project:   "a/b",
			Status:    pipeline.StatusRunning,
			CreatedAt: time.Now().Add(-90 * time.Second),
		},
	})
	got := d.Render(80, 24)
	if !contains(got, "1m") {
		t.Error("expected running duration in output")
	}
}

func TestMRLoadedMsg(t *testing.T) {
	m := testModel()
	m.selectedID = "1"

	result, _ := m.Update(MRLoadedMsg{PipelineID: "1", URL: "https://gitlab.com/mr/10"})
	m = result.(Model)

	if m.detail.mrURL != "https://gitlab.com/mr/10" {
		t.Errorf("mrURL = %q, want https://gitlab.com/mr/10", m.detail.mrURL)
	}
}

func TestMRLoadedMsg_WrongPipeline(t *testing.T) {
	m := testModel()
	m.selectedID = "1"

	result, _ := m.Update(MRLoadedMsg{PipelineID: "999", URL: "https://gitlab.com/mr/10"})
	m = result.(Model)

	if m.detail.mrURL != "" {
		t.Errorf("mrURL should be empty for wrong pipeline ID, got %q", m.detail.mrURL)
	}
}

func TestPipelineUpdatedMsg(t *testing.T) {
	m := testModel()

	result, _ := m.Update(PipelineUpdatedMsg{
		Pipeline: pipeline.Pipeline{ID: "1", Status: pipeline.StatusFailed, Ref: "main"},
	})
	m = result.(Model)

	if m.detail.row.Pipeline.Status != pipeline.StatusFailed {
		t.Errorf("status = %v, want StatusFailed", m.detail.row.Pipeline.Status)
	}
	if m.detail.row.Pipeline.Project != "group/alpha" {
		t.Error("Project should be preserved when PipelineUpdatedMsg has empty project")
	}
}

func TestRefreshTickMsg(t *testing.T) {
	m := testModel()
	refreshCalled := false
	m.Refresh = func() tea.Cmd {
		refreshCalled = true
		return nil
	}
	m.FetchJobs = func(_, _ string) tea.Cmd { return nil }
	m.FetchPipeline = func(_, _ string) tea.Cmd { return nil }

	result, cmd := m.Update(refreshTickMsg{})
	m = result.(Model)

	if !refreshCalled {
		t.Error("Refresh not called on refreshTickMsg")
	}
	if !m.loading {
		t.Error("should be loading after refreshTickMsg")
	}
	if cmd == nil {
		t.Error("expected commands from refreshTickMsg")
	}
}

func TestDetailTickMsg_ActiveJobs(t *testing.T) {
	m := testModel()
	m.detailTicking = true
	m.detail.SetJobs([]pipeline.Job{
		{ID: "1", Name: "build", Status: pipeline.StatusRunning, Stage: "build"},
	}, nil)
	initialFrame := m.detail.frame

	result, cmd := m.Update(detailTickMsg{})
	m = result.(Model)

	if m.detail.frame == initialFrame {
		t.Error("frame should advance on detailTickMsg with active jobs")
	}
	if cmd == nil {
		t.Error("should schedule another tick when active jobs exist")
	}
}

func TestRenderPipelinesPane_LoadingFocused(t *testing.T) {
	m := testModel()
	m.loading = true
	m.focus = PanePipelines
	m.loadingStatus = "syncing data..."

	view := m.View()
	if view == "" {
		t.Fatal("View returned empty during loading with pipelines focus")
	}
}

func TestFatalError_ClearsState(t *testing.T) {
	m := testModel()

	result, _ := m.Update(PipelinesLoadedMsg{Err: pipeline.ErrAuthRequired})
	m = result.(Model)

	if m.detail != nil {
		t.Error("detail should be nil after fatal error")
	}
	if len(m.pipelines) != 0 {
		t.Error("pipelines should be cleared after fatal error")
	}
	if m.selectedID != "" {
		t.Error("selectedID should be empty after fatal error")
	}
}

func TestNonFatalError_PreservesState(t *testing.T) {
	m := testModel()
	origLen := len(m.pipelines)

	result, _ := m.Update(PipelinesLoadedMsg{Err: errors.New("timeout")})
	m = result.(Model)

	if len(m.pipelines) != origLen {
		t.Error("pipelines should be preserved on non-fatal error")
	}
	if m.err == nil {
		t.Error("err should be set")
	}
}

func TestLoadingStatusMsg(t *testing.T) {
	m := New()
	m.width = 120
	m.height = 24
	m.loading = true

	result, _ := m.Update(LoadingStatusMsg{Status: "discovering..."})
	m = result.(Model)

	if m.loadingStatus != "discovering..." {
		t.Errorf("loadingStatus = %q, want \"discovering...\"", m.loadingStatus)
	}

	view := m.View()
	if !contains(view, "discovering...") {
		t.Error("loading status should appear in view")
	}
}

func TestSelectPipeline_EmptyList(t *testing.T) {
	m := Model{
		width:  120,
		height: 24,
	}
	m, _ = m.selectPipeline()

	if m.detail != nil {
		t.Error("detail should be nil with empty pipeline list")
	}
	if m.selectedID != "" {
		t.Error("selectedID should be empty with empty pipeline list")
	}
}

func TestQuitKey(t *testing.T) {
	m := testModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected quit cmd")
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func TestNavigateDown_GroupedMode(t *testing.T) {
	now := time.Now()
	m := Model{
		width:          80,
		height:         10,
		showLatestOnly: false,
		pipelines: []PipelineRow{
			{Pipeline: pipeline.Pipeline{ID: "1", ProjectID: "10", Project: "alpha", Ref: "main", SHA: "aaa11111", UpdatedAt: now.Add(-4 * time.Minute)}},
			{Pipeline: pipeline.Pipeline{ID: "2", ProjectID: "10", Project: "alpha", Ref: "feat", SHA: "aaa22222", UpdatedAt: now.Add(-3 * time.Minute)}},
			{Pipeline: pipeline.Pipeline{ID: "3", ProjectID: "20", Project: "beta", Ref: "main", SHA: "bbb11111", UpdatedAt: now.Add(-2 * time.Minute)}},
			{Pipeline: pipeline.Pipeline{ID: "4", ProjectID: "20", Project: "beta", Ref: "dev", SHA: "bbb22222", UpdatedAt: now.Add(-1 * time.Minute)}},
		},
		cursor: 0,
		offset: 0,
		focus:  PanePipelines,
	}

	for i := 0; i < 3; i++ {
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = result.(Model)
	}

	if m.cursor >= len(m.visiblePipelines()) {
		t.Errorf("cursor %d should not exceed visible pipelines %d", m.cursor, len(m.visiblePipelines()))
	}
	if m.cursor < m.offset {
		t.Errorf("cursor %d should not be less than offset %d", m.cursor, m.offset)
	}
}

func TestToggleMode_PreservesCursorPipeline(t *testing.T) {
	now := time.Now()
	m := Model{
		width:          80,
		height:         30,
		showLatestOnly: true,
		pipelines: []PipelineRow{
			{Pipeline: pipeline.Pipeline{ID: "1", ProjectID: "10", Project: "alpha", Ref: "main", UpdatedAt: now.Add(-4 * time.Minute)}},
			{Pipeline: pipeline.Pipeline{ID: "2", ProjectID: "10", Project: "alpha", Ref: "feat", UpdatedAt: now.Add(-3 * time.Minute)}},
			{Pipeline: pipeline.Pipeline{ID: "3", ProjectID: "20", Project: "beta", Ref: "main", UpdatedAt: now.Add(-2 * time.Minute)}},
			{Pipeline: pipeline.Pipeline{ID: "4", ProjectID: "20", Project: "beta", Ref: "dev", UpdatedAt: now.Add(-1 * time.Minute)}},
		},
		cursor: 2,
		focus:  PanePipelines,
	}

	preservedID := m.visiblePipelines()[m.cursor].Pipeline.ID

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = result.(Model)

	if m.cursor >= len(m.visiblePipelines()) {
		t.Errorf("cursor out of bounds after toggle: %d >= %d", m.cursor, len(m.visiblePipelines()))
		return
	}

	if m.visiblePipelines()[m.cursor].Pipeline.ID != preservedID {
		t.Errorf("cursor should preserve pipeline ID %q, got %q", preservedID, m.visiblePipelines()[m.cursor].Pipeline.ID)
	}
}

func TestVisiblePipelines_AllMode_GroupedByProject(t *testing.T) {
	now := time.Now()
	m := Model{
		width:          120,
		height:         24,
		showLatestOnly: false,
		pipelines: []PipelineRow{
			{Pipeline: pipeline.Pipeline{ID: "1", ProjectID: "10", Project: "alpha", Ref: "main", UpdatedAt: now.Add(-10 * time.Second)}},
			{Pipeline: pipeline.Pipeline{ID: "2", ProjectID: "20", Project: "beta", Ref: "main", UpdatedAt: now.Add(-5 * time.Second)}},
			{Pipeline: pipeline.Pipeline{ID: "3", ProjectID: "10", Project: "alpha", Ref: "feat", UpdatedAt: now.Add(-3 * time.Second)}},
			{Pipeline: pipeline.Pipeline{ID: "4", ProjectID: "20", Project: "beta", Ref: "dev", UpdatedAt: now.Add(-1 * time.Second)}},
		},
	}

	visible := m.visiblePipelines()

	// Should have 4 pipelines (no deduplication in "all" mode)
	if len(visible) != 4 {
		t.Errorf("expected 4 visible pipelines, got %d", len(visible))
	}

	// Groups should be ordered by most recent pipeline per project
	// beta has most recent at now-1s, alpha has most recent at now-3s
	// So beta should come first
	if visible[0].Pipeline.Project != "beta" {
		t.Errorf("first group should be beta (most recent), got %s", visible[0].Pipeline.Project)
	}
	if visible[2].Pipeline.Project != "alpha" {
		t.Errorf("second group should be alpha, got %s", visible[2].Pipeline.Project)
	}

	// Within each group, maintain original order (stable sort preserves it)
	// Beta group: ID 2 comes before ID 4 (original order)
	if visible[0].Pipeline.ID != "2" {
		t.Errorf("first beta pipeline should be ID 2, got %s", visible[0].Pipeline.ID)
	}
	if visible[1].Pipeline.ID != "4" {
		t.Errorf("second beta pipeline should be ID 4, got %s", visible[1].Pipeline.ID)
	}

	// Alpha group: ID 1 comes before ID 3 (original order)
	if visible[2].Pipeline.ID != "1" {
		t.Errorf("first alpha pipeline should be ID 1, got %s", visible[2].Pipeline.ID)
	}
	if visible[3].Pipeline.ID != "3" {
		t.Errorf("second alpha pipeline should be ID 3, got %s", visible[3].Pipeline.ID)
	}
}

func TestPipelineGroups(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name      string
		pipelines []PipelineRow
		want      []PipelineGroup
	}{
		{
			name:      "empty pipelines",
			pipelines: []PipelineRow{},
			want:      []PipelineGroup{},
		},
		{
			name: "single pipeline",
			pipelines: []PipelineRow{
				{Pipeline: pipeline.Pipeline{ID: "1", Project: "alpha", UpdatedAt: now}},
			},
			want: []PipelineGroup{
				{Project: "alpha", Start: 0, Count: 1},
			},
		},
		{
			name: "single project multiple pipelines",
			pipelines: []PipelineRow{
				{Pipeline: pipeline.Pipeline{ID: "1", Project: "alpha", UpdatedAt: now}},
				{Pipeline: pipeline.Pipeline{ID: "2", Project: "alpha", UpdatedAt: now}},
				{Pipeline: pipeline.Pipeline{ID: "3", Project: "alpha", UpdatedAt: now}},
			},
			want: []PipelineGroup{
				{Project: "alpha", Start: 0, Count: 3},
			},
		},
		{
			name: "two projects grouped",
			pipelines: []PipelineRow{
				{Pipeline: pipeline.Pipeline{ID: "1", Project: "beta", UpdatedAt: now.Add(-1 * time.Second)}},
				{Pipeline: pipeline.Pipeline{ID: "2", Project: "beta", UpdatedAt: now.Add(-2 * time.Second)}},
				{Pipeline: pipeline.Pipeline{ID: "3", Project: "alpha", UpdatedAt: now.Add(-3 * time.Second)}},
				{Pipeline: pipeline.Pipeline{ID: "4", Project: "alpha", UpdatedAt: now.Add(-4 * time.Second)}},
			},
			want: []PipelineGroup{
				{Project: "beta", Start: 0, Count: 2},
				{Project: "alpha", Start: 2, Count: 2},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				width:          120,
				height:         24,
				showLatestOnly: false,
				pipelines:      tt.pipelines,
			}

			got := m.pipelineGroups()

			if len(got) != len(tt.want) {
				t.Errorf("pipelineGroups() returned %d groups, want %d", len(got), len(tt.want))
				return
			}

			for i, g := range got {
				if g.Project != tt.want[i].Project {
					t.Errorf("group %d: Project = %q, want %q", i, g.Project, tt.want[i].Project)
				}
				if g.Start != tt.want[i].Start {
					t.Errorf("group %d: Start = %d, want %d", i, g.Start, tt.want[i].Start)
				}
				if g.Count != tt.want[i].Count {
					t.Errorf("group %d: Count = %d, want %d", i, g.Count, tt.want[i].Count)
				}
			}
		})
	}
}

func TestRenderGroupHeader(t *testing.T) {
	tests := []struct {
		name    string
		project string
		count   int
		width   int
		check   func(string) bool
	}{
		{
			name:    "normal case",
			project: "acme/frontend",
			count:   3,
			width:   80,
			check: func(s string) bool {
				return s != "" && strings.Contains(s, "acme/frontend") && strings.Contains(s, "3")
			},
		},
		{
			name:    "empty project name",
			project: "",
			count:   5,
			width:   80,
			check: func(s string) bool {
				return s != "" && strings.Contains(s, "5")
			},
		},
		{
			name:    "zero count",
			project: "test/project",
			count:   0,
			width:   80,
			check: func(s string) bool {
				return s != "" && strings.Contains(s, "test/project") && strings.Contains(s, "0")
			},
		},
		{
			name:    "width constraint truncates project",
			project: "very-long-project-name-that-exceeds-width",
			count:   5,
			width:   20,
			check: func(s string) bool {
				// Should contain arrow, count, and be non-empty
				return s != "" && strings.Contains(s, "5") && strings.Contains(s, "▸")
			},
		},
		{
			name:    "very narrow width",
			project: "project",
			count:   99,
			width:   8,
			check: func(s string) bool {
				// Should still render arrow and count
				return s != "" && strings.Contains(s, "99")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderGroupHeader(tt.project, tt.count, tt.width)
			if !tt.check(got) {
				t.Errorf("renderGroupHeader(%q, %d, %d) = %q, check failed", tt.project, tt.count, tt.width, got)
			}
		})
	}
}

func TestRenderPipelinesPane_WithHeaders(t *testing.T) {
	now := time.Now()
	m := Model{
		width:          80,
		height:         30,
		showLatestOnly: false,
		pipelines: []PipelineRow{
			{Pipeline: pipeline.Pipeline{ID: "1", ProjectID: "10", Project: "alpha", Ref: "main", SHA: "aaa11111", UpdatedAt: now.Add(-2 * time.Minute)}},
			{Pipeline: pipeline.Pipeline{ID: "2", ProjectID: "10", Project: "alpha", Ref: "feat", SHA: "aaa22222", UpdatedAt: now.Add(-3 * time.Minute)}},
			{Pipeline: pipeline.Pipeline{ID: "3", ProjectID: "20", Project: "beta", Ref: "main", SHA: "bbb11111", UpdatedAt: now.Add(-30 * time.Second)}},
			{Pipeline: pipeline.Pipeline{ID: "4", ProjectID: "20", Project: "beta", Ref: "dev", SHA: "bbb22222", UpdatedAt: now.Add(-1 * time.Minute)}},
		},
	}

	got := m.renderPipelinesPane(80, 30)

	if !strings.Contains(got, "▸") {
		t.Fatal("expected group headers in all mode with multiple projects")
	}
	if !strings.Contains(got, "alpha") || !strings.Contains(got, "beta") {
		t.Fatal("expected both project names in headers")
	}
	if strings.Count(got, "▸") != 2 {
		t.Fatalf("expected one header per project boundary, got %d headers", strings.Count(got, "▸"))
	}
	if strings.Count(got, "alpha") != 1 || strings.Count(got, "beta") != 1 {
		t.Fatal("expected grouped rendering to suppress per-item project rows")
	}
}

func TestRenderPipelinesPane_ScrolledIntoGroup_ShowsLaterHeaders(t *testing.T) {
	now := time.Now()
	m := Model{
		width:          80,
		height:         40,
		showLatestOnly: false,
		pipelines: []PipelineRow{
			{Pipeline: pipeline.Pipeline{ID: "a1", ProjectID: "10", Project: "alpha", Ref: "main", UpdatedAt: now.Add(-20 * time.Minute)}},
			{Pipeline: pipeline.Pipeline{ID: "b1", ProjectID: "20", Project: "beta", Ref: "main", UpdatedAt: now.Add(-5 * time.Minute)}},
			{Pipeline: pipeline.Pipeline{ID: "a2", ProjectID: "10", Project: "alpha", Ref: "feat", UpdatedAt: now.Add(-21 * time.Minute)}},
			{Pipeline: pipeline.Pipeline{ID: "b2", ProjectID: "20", Project: "beta", Ref: "dev", UpdatedAt: now.Add(-4 * time.Minute)}},
			{Pipeline: pipeline.Pipeline{ID: "a3", ProjectID: "10", Project: "alpha", Ref: "fix", UpdatedAt: now.Add(-22 * time.Minute)}},
			{Pipeline: pipeline.Pipeline{ID: "b3", ProjectID: "20", Project: "beta", Ref: "dev", UpdatedAt: now.Add(-3 * time.Minute)}},
			{Pipeline: pipeline.Pipeline{ID: "a4", ProjectID: "10", Project: "alpha", Ref: "fix", UpdatedAt: now.Add(-23 * time.Minute)}},
			{Pipeline: pipeline.Pipeline{ID: "b4", ProjectID: "20", Project: "beta", Ref: "dev", UpdatedAt: now.Add(-2 * time.Minute)}},
			{Pipeline: pipeline.Pipeline{ID: "a5", ProjectID: "10", Project: "alpha", Ref: "fix", UpdatedAt: now.Add(-24 * time.Minute)}},
			{Pipeline: pipeline.Pipeline{ID: "b5", ProjectID: "20", Project: "beta", Ref: "dev", UpdatedAt: now.Add(-1 * time.Minute)}},
		},
		focus:  PanePipelines,
		cursor: 3,
		offset: 3,
	}

	got := m.renderPipelinesPane(80, 40)

	// offset starts mid first group; we must still render later group headers.
	if !strings.Contains(got, "▸ alpha (5)") {
		t.Fatalf("expected later group header to render when scrolled mid-group; output:\n%s", got)
	}
}


func TestSingleGroupNoHeader(t *testing.T) {
	now := time.Now()
	m := Model{
		width:          80,
		height:         30,
		showLatestOnly: false,
		pipelines: []PipelineRow{
			{Pipeline: pipeline.Pipeline{ID: "1", ProjectID: "10", Project: "alpha", Ref: "main", SHA: "aaa11111", UpdatedAt: now.Add(-2 * time.Minute)}},
			{Pipeline: pipeline.Pipeline{ID: "2", ProjectID: "10", Project: "alpha", Ref: "feat", SHA: "aaa22222", UpdatedAt: now.Add(-1 * time.Minute)}},
		},
	}

	got := m.renderPipelinesPane(80, 30)

	if strings.Contains(got, "▸") {
		t.Fatal("expected no group header when only one project group exists")
	}
	if strings.Count(got, "alpha") < 2 {
		t.Fatal("expected per-item project rows when headers are suppressed")
	}
}

func TestVisibleItemsFromOffset(t *testing.T) {
	now := time.Now()
	m := Model{
		width:          80,
		height:         10,
		showLatestOnly: false,
		pipelines: []PipelineRow{
			{Pipeline: pipeline.Pipeline{ID: "1", ProjectID: "10", Project: "alpha", Ref: "main", SHA: "aaa11111", UpdatedAt: now.Add(-4 * time.Minute)}},
			{Pipeline: pipeline.Pipeline{ID: "2", ProjectID: "10", Project: "alpha", Ref: "feat", SHA: "aaa22222", UpdatedAt: now.Add(-3 * time.Minute)}},
			{Pipeline: pipeline.Pipeline{ID: "3", ProjectID: "20", Project: "beta", Ref: "main", SHA: "bbb11111", UpdatedAt: now.Add(-2 * time.Minute)}},
			{Pipeline: pipeline.Pipeline{ID: "4", ProjectID: "20", Project: "beta", Ref: "dev", SHA: "bbb22222", UpdatedAt: now.Add(-1 * time.Minute)}},
		},
	}

	if got := m.visibleItemsFromOffset(0); got != 2 {
		t.Errorf("visibleItemsFromOffset(0) = %d, want 2", got)
	}
	if got := m.visibleItemsFromOffset(1); got != 2 {
		t.Errorf("visibleItemsFromOffset(1) = %d, want 2", got)
	}
	if got := m.visibleItemsFromOffset(2); got != 2 {
		t.Errorf("visibleItemsFromOffset(2) = %d, want 2", got)
	}
	if got := m.visibleItemsFromOffset(3); got != 1 {
		t.Errorf("visibleItemsFromOffset(3) = %d, want 1", got)
	}
}
