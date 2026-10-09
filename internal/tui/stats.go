package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/navitronic/gitlab-pipelines/internal/pipeline"
)

// RepoStatsLoadedMsg signals that pipeline stats for a repo have been fetched.
type RepoStatsLoadedMsg struct {
	Stats pipeline.PipelineStats
	Err   error
}

// StatsModel is the top-level Bubble Tea model for the -stats view: per-day
// pipeline run counts for a repo over a fixed window of days.
type StatsModel struct {
	spinner       spinner.Model
	repo          string
	days          int
	stats         pipeline.PipelineStats
	loading       bool
	loadingStatus string
	err           error
	width         int
	height        int
	Refresh       func() tea.Cmd
}

// NewStatsModel creates a new stats TUI model in loading state for the given
// repo, showing counts for the last `days` calendar days.
func NewStatsModel(repo string, days int) StatsModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return StatsModel{
		spinner: s,
		repo:    repo,
		days:    days,
		loading: true,
		width:   120,
		height:  24,
	}
}

func (m StatsModel) Init() tea.Cmd {
	cmds := []tea.Cmd{m.spinner.Tick}
	if m.Refresh != nil {
		cmds = append(cmds, m.Refresh())
	}
	return tea.Batch(cmds...)
}

func (m StatsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			if !m.loading && m.Refresh != nil {
				m.loading = true
				m.err = nil
				return m, tea.Batch(m.spinner.Tick, m.Refresh())
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case LoadingStatusMsg:
		m.loadingStatus = msg.Status
		return m, nil

	case RepoStatsLoadedMsg:
		m.loading = false
		m.loadingStatus = ""
		if msg.Err != nil {
			m.err = formatError(msg.Err)
			if isFatalError(msg.Err) {
				m.stats = pipeline.PipelineStats{}
			}
			return m, nil
		}
		m.err = nil
		m.stats = msg.Stats
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m StatsModel) View() string {
	if m.err != nil && len(m.stats.Days) == 0 {
		errMsg := errorStyle.Render(fmt.Sprintf("Error: %v", m.err))
		hint := helpStyle.Render("r: retry • q: quit")
		content := lipgloss.JoinVertical(lipgloss.Left, errMsg, "", hint)
		return lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Center, content)
	}
	if m.loading && len(m.stats.Days) == 0 {
		status := "Loading stats..."
		if m.loadingStatus != "" {
			status = m.loadingStatus
		}
		content := m.spinner.View() + " " + status
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
	}

	top := m.renderTop()
	statusBar := m.renderStatusBar()
	padded := lipgloss.NewStyle().Width(m.width).Height(max(m.height-1, 1)).Render(top)
	return lipgloss.JoinVertical(lipgloss.Left, padded, statusBar)
}

func (m StatsModel) renderTop() string {
	header := listTitleStyle.Render(statsHeaderText(m.repo, m.days))
	return lipgloss.JoinVertical(lipgloss.Left, header, "", renderStatsTable(m.stats))
}

func (m StatsModel) renderStatusBar() string {
	center := strings.Join([]string{"r: refresh", "q: quit"}, " • ")
	content := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, center)
	return statusBarStyle.Width(m.width).Render(content)
}

func statsHeaderText(repo string, days int) string {
	return fmt.Sprintf("Pipeline runs for %s — last %d days", repo, days)
}

const (
	statsDateWidth = 12
	statsNumWidth  = 6
)

func renderStatsTable(stats pipeline.PipelineStats) string {
	headerCells := []string{
		padRight("DATE", statsDateWidth),
		padLeft("RUNS", statsNumWidth),
		padLeft("PASS", statsNumWidth),
		padLeft("FAIL", statsNumWidth),
		padLeft("OTHER", statsNumWidth),
	}
	header := tableHeaderStyle.Render(strings.Join(headerCells, " "))
	divider := dimStyle.Render(strings.Repeat("─", lipgloss.Width(header)))

	lines := []string{header, divider}
	var total, passed, failed, other int
	for _, d := range stats.Days {
		lines = append(lines, renderStatsRow(d.Date.Format("Mon 02 Jan"), d.Total, d.Passed, d.Failed, d.Other))
		total += d.Total
		passed += d.Passed
		failed += d.Failed
		other += d.Other
	}
	lines = append(lines, divider)
	lines = append(lines, tableTotalStyle.Render(renderStatsRow("TOTAL", total, passed, failed, other)))

	return tableStyle.Render(strings.Join(lines, "\n"))
}

func renderStatsRow(date string, total, passed, failed, other int) string {
	cells := []string{
		padRight(date, statsDateWidth),
		padLeft(strconv.Itoa(total), statsNumWidth),
		padLeft(styledCount(passed, successStyle), statsNumWidth),
		padLeft(styledCount(failed, failedStyle), statsNumWidth),
		padLeft(styledCount(other, pendingStyle), statsNumWidth),
	}
	return strings.Join(cells, " ")
}
