package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/navitronic/gitlab-builds/internal/gitlab"
)

type view int

const (
	viewList view = iota
	viewDetail
)

// PipelinesLoadedMsg signals that pipelines have been fetched.
type PipelinesLoadedMsg struct {
	Pipelines []PipelineRow
	Err       error
}

// PipelineRow holds display data for one pipeline in the list.
type PipelineRow struct {
	Pipeline    gitlab.Pipeline
	ProjectPath string
	JobSummary  string
}

// Model is the top-level Bubble Tea model.
type Model struct {
	table      table.Model
	spinner    spinner.Model
	pipelines  []PipelineRow
	loading    bool
	err        error
	width      int
	height     int
	currentView view
	detail     *DetailModel
}

// New creates a new TUI model in loading state.
func New() Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	columns := []table.Column{
		{Title: "Status", Width: 10},
		{Title: "Project", Width: 28},
		{Title: "Ref", Width: 18},
		{Title: "Commit", Width: 10},
		{Title: "Pipeline", Width: 10},
		{Title: "Jobs", Width: 10},
		{Title: "Updated", Width: 16},
		{Title: "Source", Width: 12},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
	)
	t.SetStyles(tableStyles())

	return Model{
		table:   t,
		spinner: s,
		loading: true,
	}
}

func (m Model) Init() tea.Cmd {
	return m.spinner.Tick
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
		case "enter":
			if len(m.pipelines) > 0 {
				idx := m.table.Cursor()
				if idx >= 0 && idx < len(m.pipelines) {
					m.currentView = viewDetail
					m.detail = NewDetailModel(m.pipelines[idx])
					return m, nil
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetWidth(msg.Width)
		m.table.SetHeight(msg.Height - 4)

	case PipelinesLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.pipelines = msg.Pipelines
		m.table.SetRows(buildRows(msg.Pipelines))
		return m, nil

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
	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Error: %v", m.err))
	}
	if m.loading {
		return fmt.Sprintf("\n  %s Loading pipelines...\n", m.spinner.View())
	}
	if len(m.pipelines) == 0 {
		return "\n  No pipelines found.\n"
	}

	header := titleStyle.Render("GitLab Pipelines")
	help := helpStyle.Render("↑/↓: navigate • enter: details • q: quit")
	return header + "\n" + m.table.View() + "\n" + help
}

func buildRows(pipelines []PipelineRow) []table.Row {
	rows := make([]table.Row, 0, len(pipelines))
	for _, p := range pipelines {
		rows = append(rows, table.Row{
			statusIcon(p.Pipeline.Status),
			truncate(p.ProjectPath, 26),
			truncate(p.Pipeline.Ref, 16),
			shortSHA(p.Pipeline.SHA),
			fmt.Sprintf("#%d", p.Pipeline.ID),
			p.JobSummary,
			formatTime(p.Pipeline.UpdatedAt),
			p.Pipeline.Source,
		})
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
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).MarginLeft(2)
	helpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).MarginLeft(2)
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).MarginLeft(2)
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	failedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	runningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
	pendingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	canceledStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	skippedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)


