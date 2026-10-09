package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/navitronic/gitlab-pipelines/internal/pipeline"
)

func testStatsModel() StatsModel {
	m := NewStatsModel("group/project", 7)
	m.width = 120
	m.height = 24
	m.loading = false
	now := time.Now()
	stats := pipeline.PipelineStats{Project: "group/project"}
	for i := 6; i >= 0; i-- {
		stats.Days = append(stats.Days, pipeline.DailyPipelineStats{Date: now.AddDate(0, 0, -i)})
	}
	stats.Days[6].Total = 3
	stats.Days[6].Passed = 2
	stats.Days[6].Failed = 1
	m.stats = stats
	return m
}

func TestStatsHeaderText(t *testing.T) {
	want := "Pipeline runs for group/project — last 7 days"
	if got := statsHeaderText("group/project", 7); got != want {
		t.Errorf("statsHeaderText = %q, want %q", got, want)
	}
}

func TestStatsModel_Init_CallsRefresh(t *testing.T) {
	m := NewStatsModel("group/project", 7)
	called := false
	m.Refresh = func() tea.Cmd {
		called = true
		return nil
	}

	cmd := m.Init()
	if cmd == nil {
		t.Fatal("expected Init to return a command")
	}
	if !called {
		t.Error("expected Init to call Refresh for the initial fetch")
	}
}

func TestStatsModel_RepoStatsLoadedMsg(t *testing.T) {
	m := testStatsModel()
	m.loading = true

	stats := pipeline.PipelineStats{Project: "group/project", Days: []pipeline.DailyPipelineStats{{Total: 1}}}
	result, _ := m.Update(RepoStatsLoadedMsg{Stats: stats})
	m = result.(StatsModel)

	if m.loading {
		t.Error("expected loading to be false after load")
	}
	if m.stats.Project != "group/project" || len(m.stats.Days) != 1 {
		t.Errorf("stats not applied: %+v", m.stats)
	}
}

func TestStatsModel_RepoStatsLoadedMsg_FatalError(t *testing.T) {
	m := testStatsModel()

	result, _ := m.Update(RepoStatsLoadedMsg{Err: pipeline.ErrAuthRequired})
	m = result.(StatsModel)

	if m.err == nil {
		t.Fatal("expected error to be set")
	}
	if len(m.stats.Days) != 0 {
		t.Errorf("expected stats to be cleared on fatal error, got %d days", len(m.stats.Days))
	}
}

func TestStatsModel_RepoStatsLoadedMsg_NonFatalError_PreservesStats(t *testing.T) {
	m := testStatsModel()
	existing := len(m.stats.Days)

	result, _ := m.Update(RepoStatsLoadedMsg{Err: errors.New("transient")})
	m = result.(StatsModel)

	if m.err == nil {
		t.Fatal("expected error to be set")
	}
	if len(m.stats.Days) != existing {
		t.Errorf("expected stats to be preserved, got %d want %d", len(m.stats.Days), existing)
	}
}

func TestStatsModel_RefreshKey(t *testing.T) {
	m := testStatsModel()
	called := false
	m.Refresh = func() tea.Cmd {
		called = true
		return nil
	}

	result, cmd := m.Update(tea.KeyMsg{Runes: []rune("r"), Type: tea.KeyRunes})
	m = result.(StatsModel)

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

func TestStatsModel_QuitKey(t *testing.T) {
	m := testStatsModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

func TestStatsModel_ViewLoadingState(t *testing.T) {
	m := NewStatsModel("group/project", 7)
	m.width = 120
	m.height = 24

	if view := m.View(); view == "" {
		t.Fatal("View returned empty string in loading state")
	}
}

func TestStatsModel_ViewErrorState(t *testing.T) {
	m := NewStatsModel("group/project", 7)
	m.width = 120
	m.height = 24
	m.loading = false
	m.err = errors.New("boom")

	view := m.View()
	if !strings.Contains(view, "boom") {
		t.Errorf("expected view to contain error message, got:\n%s", view)
	}
}

func TestStatsModel_ViewWithStats(t *testing.T) {
	m := testStatsModel()
	view := m.View()

	if !strings.Contains(view, "last 7 days") {
		t.Errorf("expected view to contain header, got:\n%s", view)
	}
	if !strings.Contains(view, "TOTAL") {
		t.Errorf("expected view to contain totals row, got:\n%s", view)
	}
	if !strings.Contains(view, "RUNS") {
		t.Errorf("expected view to contain table header, got:\n%s", view)
	}
}
