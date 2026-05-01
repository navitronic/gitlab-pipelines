package tui

import (
	"errors"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/navitronic/gitlab-builds/internal/gitlab"
	"github.com/navitronic/gitlab-builds/internal/glab"
)

type view int

const (
	viewList view = iota
	viewDetail
)

const refreshInterval = 30 * time.Second

// PipelinesLoadedMsg signals that pipelines have been fetched.
type PipelinesLoadedMsg struct {
	Pipelines []PipelineRow
	Err       error
}

type refreshTickMsg struct{}
type detailTickMsg struct{}

// PipelineRow holds display data for one pipeline in the list.
type PipelineRow struct {
	Pipeline    gitlab.Pipeline
	ProjectPath string
	JobSummary  string
}

// Model is the top-level Bubble Tea model.
type Model struct {
	table       table.Model
	spinner     spinner.Model
	pipelines   []PipelineRow
	loading     bool
	err         error
	width       int
	height      int
	currentView view
	detail      *DetailModel
	FetchJobs   func(projectID, pipelineID int) tea.Cmd
	Refresh     func() tea.Cmd
}

// New creates a new TUI model in loading state.
func New() Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	columns := defaultColumns(120)
	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
	)
	t.SetStyles(tableStyles())

	return Model{
		table:   t,
		spinner: s,
		loading: true,
		width:   120,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, scheduleRefresh())
}

func scheduleRefresh() tea.Cmd {
	return tea.Tick(refreshInterval, func(time.Time) tea.Msg {
		return refreshTickMsg{}
	})
}

func scheduleDetailTick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return detailTickMsg{}
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.currentView {
	case viewDetail:
		return m.updateDetail(msg)
	default:
		return m.updateList(msg)
	}
}

func (m Model) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		case "enter":
			if len(m.pipelines) > 0 {
				idx := m.table.Cursor()
				if idx >= 0 && idx < len(m.pipelines) {
					m.currentView = viewDetail
					m.detail = NewDetailModel(m.pipelines[idx])
					var cmds []tea.Cmd
					cmds = append(cmds, scheduleDetailTick())
					if m.FetchJobs != nil {
						row := m.pipelines[idx]
						cmds = append(cmds, m.FetchJobs(row.Pipeline.ProjectID, row.Pipeline.ID))
					}
					return m, tea.Batch(cmds...)
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if msg.Width > 0 {
			m.table.SetWidth(msg.Width)
		}
		h := msg.Height - 4
		if h < 1 {
			h = 1
		}
		m.table.SetHeight(h)
		m.table.SetRows(nil)
		m.table.SetColumns(defaultColumns(msg.Width))
		if len(m.pipelines) > 0 {
			m.table.SetRows(buildRows(m.pipelines, msg.Width))
		}
		return m, nil

	case PipelinesLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = formatError(msg.Err)
			return m, nil
		}
		m.err = nil
		m.pipelines = msg.Pipelines
		m.table.SetRows(buildRows(msg.Pipelines, m.width))
		return m, nil

	case refreshTickMsg:
		if !m.loading && m.Refresh != nil && m.currentView == viewList {
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, m.Refresh(), scheduleRefresh())
		}
		return m, scheduleRefresh()

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m Model) updateDetail(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc", "backspace":
			m.currentView = viewList
			m.detail = nil
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case JobsLoadedMsg:
		if m.detail != nil {
			m.detail.SetJobs(msg.Jobs, msg.Err)
		}
	case refreshTickMsg:
		if m.detail != nil && m.FetchJobs != nil {
			row := m.detail.row
			return m, tea.Batch(m.FetchJobs(row.Pipeline.ProjectID, row.Pipeline.ID), scheduleRefresh())
		}
		return m, scheduleRefresh()
	case detailTickMsg:
		if m.detail != nil && m.detail.hasActiveJobs() {
			return m, scheduleDetailTick()
		}
	}
	return m, nil
}

func (m Model) View() string {
	switch m.currentView {
	case viewDetail:
		if m.detail != nil {
			return m.detail.View()
		}
		return ""
	default:
		return m.viewList()
	}
}

func (m Model) viewList() string {
	if m.err != nil && len(m.pipelines) == 0 {
		errMsg := errorStyle.Render(fmt.Sprintf("Error: %v", m.err))
		hint := helpStyle.Render("r: retry • q: quit")
		return fmt.Sprintf("\n%s\n\n%s\n", errMsg, hint)
	}
	if m.loading && len(m.pipelines) == 0 {
		return fmt.Sprintf("\n  %s Loading pipelines...\n", m.spinner.View())
	}
	if len(m.pipelines) == 0 {
		return "\n  No pipelines found.\n\n" + helpStyle.Render("r: refresh • q: quit") + "\n"
	}

	header := titleStyle.Render("GitLab Pipelines")
	if m.loading {
		header += " " + m.spinner.View()
	}
	help := helpStyle.Render("↑/↓: navigate • enter: details • r: refresh • q: quit")
	return header + "\n" + m.table.View() + "\n" + help
}

func defaultColumns(width int) []table.Column {
	if width < 80 {
		return []table.Column{
			{Title: "Status", Width: 10},
			{Title: "Project", Width: 20},
			{Title: "Ref", Width: 12},
			{Title: "Pipeline", Width: 8},
			{Title: "Jobs", Width: 8},
		}
	}
	if width < 120 {
		return []table.Column{
			{Title: "Status", Width: 10},
			{Title: "Project", Width: 25},
			{Title: "Ref", Width: 16},
			{Title: "Commit", Width: 10},
			{Title: "Pipeline", Width: 10},
			{Title: "Jobs", Width: 10},
			{Title: "Updated", Width: 14},
		}
	}
	return []table.Column{
		{Title: "Status", Width: 10},
		{Title: "Project", Width: 28},
		{Title: "Ref", Width: 18},
		{Title: "Commit", Width: 10},
		{Title: "Pipeline", Width: 10},
		{Title: "Jobs", Width: 10},
		{Title: "Updated", Width: 16},
		{Title: "Source", Width: 12},
	}
}

func buildRows(pipelines []PipelineRow, width int) []table.Row {
	rows := make([]table.Row, 0, len(pipelines))
	for _, p := range pipelines {
		var row table.Row
		if width < 80 {
			row = table.Row{
				statusIcon(p.Pipeline.Status),
				truncate(p.ProjectPath, 18),
				truncate(p.Pipeline.Ref, 10),
				fmt.Sprintf("#%d", p.Pipeline.ID),
				p.JobSummary,
			}
		} else if width < 120 {
			row = table.Row{
				statusIcon(p.Pipeline.Status),
				truncate(p.ProjectPath, 23),
				truncate(p.Pipeline.Ref, 14),
				shortSHA(p.Pipeline.SHA),
				fmt.Sprintf("#%d", p.Pipeline.ID),
				p.JobSummary,
				formatTime(p.Pipeline.UpdatedAt),
			}
		} else {
			row = table.Row{
				statusIcon(p.Pipeline.Status),
				truncate(p.ProjectPath, 26),
				truncate(p.Pipeline.Ref, 16),
				shortSHA(p.Pipeline.SHA),
				fmt.Sprintf("#%d", p.Pipeline.ID),
				p.JobSummary,
				formatTime(p.Pipeline.UpdatedAt),
				p.Pipeline.Source,
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func statusIcon(status string) string {
	switch status {
	case "success":
		return successStyle.Render("✓ passed")
	case "failed":
		return failedStyle.Render("✗ failed")
	case "running":
		return runningStyle.Render("● running")
	case "pending":
		return pendingStyle.Render("○ pending")
	case "canceled":
		return canceledStyle.Render("⊘ canceled")
	case "skipped":
		return skippedStyle.Render("⊘ skipped")
	default:
		return status
	}
}

func shortSHA(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	now := time.Now()
	diff := now.Sub(t)
	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	default:
		return t.Format("Jan 02 15:04")
	}
}

func formatError(err error) error {
	if errors.Is(err, glab.ErrGlabNotFound) {
		return fmt.Errorf("glab CLI not found. Install it: https://gitlab.com/gitlab-org/cli")
	}
	if errors.Is(err, glab.ErrAuthRequired) {
		return fmt.Errorf("glab not authenticated. Run: glab auth login")
	}
	return err
}

func tableStyles() table.Styles {
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	return s
}

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).MarginLeft(2)
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).MarginLeft(2)
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).MarginLeft(2)
	successStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	failedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	runningStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
	pendingStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	canceledStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	skippedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)
