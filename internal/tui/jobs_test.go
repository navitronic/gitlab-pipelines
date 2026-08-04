package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/navitronic/gitlab-pipelines/internal/pipeline"
)

func testJobsModel() JobsModel {
	m := NewJobsModel("group/project", nil)
	m.width = 120
	m.height = 24
	m.loading = false
	m.jobs = []pipeline.Job{
		{ID: "1", Name: "build", Stage: "build", Status: pipeline.StatusPassed, WebURL: "https://gitlab.com/j/1", CreatedAt: time.Now()},
		{ID: "2", Name: "unit", Stage: "test", Status: pipeline.StatusFailed, WebURL: "https://gitlab.com/j/2", CreatedAt: time.Now()},
		{ID: "3", Name: "unit", Stage: "test", Status: pipeline.StatusPassed, WebURL: "https://gitlab.com/j/3", CreatedAt: time.Now()},
	}
	return m
}

func TestSummarizeJobs(t *testing.T) {
	jobs := []pipeline.Job{
		{Name: "unit", Stage: "test", Status: pipeline.StatusPassed},
		{Name: "unit", Stage: "test", Status: pipeline.StatusFailed},
		{Name: "build", Stage: "build", Status: pipeline.StatusPassed},
	}

	rows, total := summarizeJobs(jobs)

	if len(rows) != 2 {
		t.Fatalf("expected 2 summary rows, got %d", len(rows))
	}
	// Sorted by stage then name: "build" < "test".
	if rows[0].Stage != "build" || rows[0].Name != "build" {
		t.Errorf("rows[0] = %+v, want stage=build name=build", rows[0])
	}
	if rows[0].Counts.Passed != 1 {
		t.Errorf("rows[0].Counts.Passed = %d, want 1", rows[0].Counts.Passed)
	}
	if rows[1].Stage != "test" || rows[1].Name != "unit" {
		t.Errorf("rows[1] = %+v, want stage=test name=unit", rows[1])
	}
	if rows[1].Counts.Passed != 1 || rows[1].Counts.Failed != 1 {
		t.Errorf("rows[1].Counts = %+v, want Passed=1 Failed=1", rows[1].Counts)
	}
	if total.Passed != 2 || total.Failed != 1 {
		t.Errorf("total = %+v, want Passed=2 Failed=1", total)
	}
	if total.total() != 3 {
		t.Errorf("total.total() = %d, want 3", total.total())
	}
}

func TestSummarizeJobs_Empty(t *testing.T) {
	rows, total := summarizeJobs(nil)
	if len(rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(rows))
	}
	if total.total() != 0 {
		t.Errorf("expected 0 total, got %d", total.total())
	}
}

func TestJobsHeaderText(t *testing.T) {
	if got := jobsHeaderText("group/project", nil); got != "Jobs for group/project — Today" {
		t.Errorf("jobsHeaderText(nil) = %q, want no stage suffix", got)
	}
	got := jobsHeaderText("group/project", []string{"build", "test"})
	want := "Jobs for group/project — Today (stages: build, test)"
	if got != want {
		t.Errorf("jobsHeaderText(stages) = %q, want %q", got, want)
	}
}

func TestPadRightPadLeft(t *testing.T) {
	if got := padRight("ab", 5); got != "ab   " {
		t.Errorf("padRight = %q, want %q", got, "ab   ")
	}
	if got := padLeft("ab", 5); got != "   ab" {
		t.Errorf("padLeft = %q, want %q", got, "   ab")
	}
	if got := padRight("abcdef", 3); got != "abcdef" {
		t.Errorf("padRight should not truncate, got %q", got)
	}
}

func TestRenderJobSummaryTable_ContainsTotal(t *testing.T) {
	rows, total := summarizeJobs([]pipeline.Job{
		{Name: "build", Stage: "build", Status: pipeline.StatusPassed},
	})
	out := renderJobSummaryTable(rows, total, 120)
	if !strings.Contains(out, "TOTAL") {
		t.Errorf("expected table to contain TOTAL row, got:\n%s", out)
	}
	if !strings.Contains(out, "build") {
		t.Errorf("expected table to contain job row, got:\n%s", out)
	}
}

func TestJobsModel_NavigateDown(t *testing.T) {
	m := testJobsModel()

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = result.(JobsModel)

	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.cursor)
	}
}

func TestJobsModel_NavigateUp(t *testing.T) {
	m := testJobsModel()
	m.cursor = 2

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = result.(JobsModel)

	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.cursor)
	}
}

func TestJobsModel_NavigateDown_StopsAtEnd(t *testing.T) {
	m := testJobsModel()
	m.cursor = len(m.jobs) - 1

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = result.(JobsModel)

	if m.cursor != len(m.jobs)-1 {
		t.Errorf("cursor = %d, want %d (should not move past end)", m.cursor, len(m.jobs)-1)
	}
}

func TestJobsModel_QuitKey(t *testing.T) {
	m := testJobsModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

func TestJobsModel_OpenKey_NoPanicWhenEmptyURL(t *testing.T) {
	m := testJobsModel()
	m.jobs[m.cursor].WebURL = ""
	// Should not panic and should not attempt to open a browser.
	m.Update(tea.KeyMsg{Runes: []rune("o"), Type: tea.KeyRunes})
}

func TestJobsModel_OpenKey_NoPanicWhenNoJobs(t *testing.T) {
	m := testJobsModel()
	m.jobs = nil
	m.cursor = 0
	m.Update(tea.KeyMsg{Runes: []rune("o"), Type: tea.KeyRunes})
}

func TestJobsModel_RefreshKey_NoOpWhileLoading(t *testing.T) {
	m := testJobsModel()
	m.loading = true
	called := false
	m.Refresh = func() tea.Cmd {
		called = true
		return nil
	}

	m.Update(tea.KeyMsg{Runes: []rune("r"), Type: tea.KeyRunes})
	if called {
		t.Error("Refresh should not be called while already loading")
	}
}

func TestJobsModel_RefreshKey(t *testing.T) {
	m := testJobsModel()
	called := false
	m.Refresh = func() tea.Cmd {
		called = true
		return nil
	}

	result, cmd := m.Update(tea.KeyMsg{Runes: []rune("r"), Type: tea.KeyRunes})
	m = result.(JobsModel)

	if !called {
		t.Error("expected Refresh to be called")
	}
	if !m.loading {
		t.Error("expected loading to be true after refresh")
	}
	if cmd == nil {
		t.Error("expected a batched command")
	}
}

func TestJobsModel_RefreshTickMsg_TriggersRefresh(t *testing.T) {
	m := testJobsModel()
	called := false
	m.Refresh = func() tea.Cmd {
		called = true
		return nil
	}

	result, cmd := m.Update(refreshTickMsg{})
	m = result.(JobsModel)

	if !called {
		t.Error("expected Refresh to be called on refreshTickMsg")
	}
	if !m.loading {
		t.Error("expected loading to be true after refresh tick")
	}
	if cmd == nil {
		t.Error("expected a batched command (including the next scheduled tick)")
	}
}

func TestJobsModel_RefreshTickMsg_NoOpWhileLoading(t *testing.T) {
	m := testJobsModel()
	m.loading = true
	called := false
	m.Refresh = func() tea.Cmd {
		called = true
		return nil
	}

	result, cmd := m.Update(refreshTickMsg{})
	m = result.(JobsModel)

	if called {
		t.Error("Refresh should not be called while already loading")
	}
	// The next tick should still be scheduled even when this one was a no-op.
	if cmd == nil {
		t.Error("expected the next refresh tick to still be scheduled")
	}
}

func TestJobsModel_Init_SchedulesRefresh(t *testing.T) {
	m := NewJobsModel("group/project", nil)
	refreshCalled := false
	m.Refresh = func() tea.Cmd {
		refreshCalled = true
		return nil
	}

	cmd := m.Init()
	if cmd == nil {
		t.Fatal("expected Init to return a command")
	}
	if !refreshCalled {
		t.Error("expected Init to call Refresh for the initial fetch")
	}
}

func TestJobsModel_RepoJobsLoadedMsg(t *testing.T) {
	m := testJobsModel()
	m.loading = true

	jobs := []pipeline.Job{{ID: "9", Name: "deploy", Status: pipeline.StatusPassed}}
	result, _ := m.Update(RepoJobsLoadedMsg{Jobs: jobs})
	m = result.(JobsModel)

	if m.loading {
		t.Error("expected loading to be false after load")
	}
	if len(m.jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(m.jobs))
	}
}

func TestJobsModel_RepoJobsLoadedMsg_StartsTickingWhenJobsActive(t *testing.T) {
	m := testJobsModel()
	m.loading = true

	jobs := []pipeline.Job{{ID: "9", Name: "deploy", Status: pipeline.StatusRunning}}
	result, cmd := m.Update(RepoJobsLoadedMsg{Jobs: jobs})
	m = result.(JobsModel)

	if !m.ticking {
		t.Error("expected ticking to start when a running job is loaded")
	}
	if cmd == nil {
		t.Error("expected a tick command to be scheduled")
	}
}

func TestJobsModel_RepoJobsLoadedMsg_NoTickWhenNoActiveJobs(t *testing.T) {
	m := testJobsModel()
	m.loading = true

	jobs := []pipeline.Job{{ID: "9", Name: "deploy", Status: pipeline.StatusPassed}}
	result, _ := m.Update(RepoJobsLoadedMsg{Jobs: jobs})
	m = result.(JobsModel)

	if m.ticking {
		t.Error("expected ticking not to start when no job is active")
	}
}

func TestJobsModel_RepoJobsLoadedMsg_DoesNotStackTickChains(t *testing.T) {
	m := testJobsModel()
	m.ticking = true // already ticking from a previous load

	jobs := []pipeline.Job{{ID: "9", Name: "deploy", Status: pipeline.StatusRunning}}
	result, cmd := m.Update(RepoJobsLoadedMsg{Jobs: jobs})
	m = result.(JobsModel)

	if !m.ticking {
		t.Error("expected ticking to remain true")
	}
	if cmd != nil {
		t.Error("expected no new tick command when already ticking")
	}
}

func TestJobsModel_JobsTickMsg_ReschedulesWhileActive(t *testing.T) {
	m := testJobsModel()
	m.ticking = true
	m.jobs[0].Status = pipeline.StatusRunning

	result, cmd := m.Update(jobsTickMsg{})
	m = result.(JobsModel)

	if !m.ticking {
		t.Error("expected ticking to remain true while a job is active")
	}
	if cmd == nil {
		t.Error("expected the tick to reschedule itself")
	}
}

func TestJobsModel_JobsTickMsg_StopsWhenNoLongerActive(t *testing.T) {
	m := testJobsModel()
	m.ticking = true
	for i := range m.jobs {
		m.jobs[i].Status = pipeline.StatusPassed
	}

	result, _ := m.Update(jobsTickMsg{})
	m = result.(JobsModel)

	if m.ticking {
		t.Error("expected ticking to stop once no job is active")
	}
}

func TestJobsModel_ViewWithJobs_ShowsLiveDuration(t *testing.T) {
	m := testJobsModel()
	m.jobs = []pipeline.Job{
		{ID: "1", Name: "deploy", Stage: "deploy", Status: pipeline.StatusRunning, StartedAt: time.Now().Add(-90 * time.Second)},
	}

	view := m.View()
	if !strings.Contains(view, "1m30s") {
		t.Errorf("expected view to show a live-computed running duration, got:\n%s", view)
	}
}

func TestJobsModel_RepoJobsLoadedMsg_FatalError(t *testing.T) {
	m := testJobsModel()

	result, _ := m.Update(RepoJobsLoadedMsg{Err: pipeline.ErrAuthRequired})
	m = result.(JobsModel)

	if m.err == nil {
		t.Fatal("expected error to be set")
	}
	if m.jobs != nil {
		t.Errorf("expected jobs to be cleared on fatal error, got %d", len(m.jobs))
	}
}

func TestJobsModel_RepoJobsLoadedMsg_NonFatalError_PreservesJobs(t *testing.T) {
	m := testJobsModel()
	existing := len(m.jobs)

	result, _ := m.Update(RepoJobsLoadedMsg{Err: errors.New("transient")})
	m = result.(JobsModel)

	if m.err == nil {
		t.Fatal("expected error to be set")
	}
	if len(m.jobs) != existing {
		t.Errorf("expected jobs to be preserved, got %d want %d", len(m.jobs), existing)
	}
}

func TestJobsModel_LoadingStatusMsg(t *testing.T) {
	m := testJobsModel()
	result, _ := m.Update(LoadingStatusMsg{Status: "fetching jobs..."})
	m = result.(JobsModel)

	if m.loadingStatus != "fetching jobs..." {
		t.Errorf("loadingStatus = %q, want %q", m.loadingStatus, "fetching jobs...")
	}
}

func TestJobsModel_WindowSizeMsg(t *testing.T) {
	m := testJobsModel()
	result, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = result.(JobsModel)

	if m.width != 100 || m.height != 40 {
		t.Errorf("width/height = %d/%d, want 100/40", m.width, m.height)
	}
}

func TestJobsModel_ViewLoadingState(t *testing.T) {
	m := NewJobsModel("group/project", nil)
	m.width = 120
	m.height = 24

	view := m.View()
	if view == "" {
		t.Fatal("View returned empty string in loading state")
	}
}

func TestJobsModel_ViewErrorState(t *testing.T) {
	m := NewJobsModel("group/project", nil)
	m.width = 120
	m.height = 24
	m.loading = false
	m.err = errors.New("boom")

	view := m.View()
	if !strings.Contains(view, "boom") {
		t.Errorf("expected view to contain error message, got:\n%s", view)
	}
}

func TestJobsModel_ViewEmptyState(t *testing.T) {
	m := testJobsModel()
	m.jobs = nil

	view := m.View()
	if !strings.Contains(view, "No jobs found today.") {
		t.Errorf("expected empty state message, got:\n%s", view)
	}
}

func TestJobsModel_ViewWithJobs(t *testing.T) {
	m := testJobsModel()
	view := m.View()

	if !strings.Contains(view, "build") {
		t.Errorf("expected view to contain job name, got:\n%s", view)
	}
	if !strings.Contains(view, "TOTAL") {
		t.Errorf("expected view to contain totals table, got:\n%s", view)
	}
}

func TestJobsModel_ViewShowsStageFilter(t *testing.T) {
	m := testJobsModel()
	m.stages = []string{"test"}

	view := m.View()
	if !strings.Contains(view, "stages: test") {
		t.Errorf("expected view to show active stage filter, got:\n%s", view)
	}
}

func TestJobsModel_ViewShowsSyncingToastDuringBackgroundRefresh(t *testing.T) {
	m := testJobsModel()
	m.loading = true // background refresh with existing jobs already shown

	view := m.View()
	if !strings.Contains(view, "syncing...") {
		t.Errorf("expected view to show a syncing toast during background refresh, got:\n%s", view)
	}
	if !strings.Contains(view, "build") {
		t.Errorf("expected existing jobs to remain visible during background refresh, got:\n%s", view)
	}
}

func TestJobsModel_ViewNoToastWhenNotLoading(t *testing.T) {
	m := testJobsModel()
	m.loading = false

	view := m.View()
	if strings.Contains(view, "syncing...") {
		t.Errorf("expected no syncing toast when not loading, got:\n%s", view)
	}
}
