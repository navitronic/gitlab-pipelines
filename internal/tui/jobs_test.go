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
	m := NewJobsModel("group/project")
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
	m := NewJobsModel("group/project")
	m.width = 120
	m.height = 24

	view := m.View()
	if view == "" {
		t.Fatal("View returned empty string in loading state")
	}
}

func TestJobsModel_ViewErrorState(t *testing.T) {
	m := NewJobsModel("group/project")
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
