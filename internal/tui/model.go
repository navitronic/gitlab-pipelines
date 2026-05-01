package tui

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/navitronic/gitlab-builds/internal/gitlab"
	"github.com/navitronic/gitlab-builds/internal/glab"
)

const refreshInterval = 30 * time.Second

// PipelinesLoadedMsg signals that pipelines have been fetched.
type PipelinesLoadedMsg struct {
	Pipelines []PipelineRow
	Err       error
}

// PipelineUpdatedMsg signals that a single pipeline has been re-fetched.
type PipelineUpdatedMsg struct {
	Pipeline gitlab.Pipeline
	Err      error
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
	spinner       spinner.Model
	pipelines     []PipelineRow
	cursor        int
	offset        int
	loading       bool
	err           error
	width         int
	height        int
	detail        *DetailModel
	selectedID    int
	FetchJobs     func(projectID, pipelineID int) tea.Cmd
	FetchPipeline func(projectID, pipelineID int) tea.Cmd
	Refresh       func() tea.Cmd
}

// New creates a new TUI model in loading state.
func New() Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return Model{
		spinner: s,
		loading: true,
		width:   120,
		height:  24,
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
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
		return detailTickMsg{}
	})
}

func (m Model) listWidth() int {
	return min(max(m.width*2/5, 30), m.width)
}

func (m Model) detailWidth() int {
	return m.width - m.listWidth()
}

// visibleItems returns how many 2-line pipeline items fit in the viewport.
func (m Model) visibleItems() int {
	available := m.height - 3
	if available < 3 {
		return 1
	}
	return (available + 1) / 3
}

func (m Model) selectPipeline() (Model, tea.Cmd) {
	if len(m.pipelines) == 0 {
		m.detail = nil
		m.selectedID = 0
		return m, nil
	}
	row := m.pipelines[m.cursor]
	if row.Pipeline.ID == m.selectedID {
		return m, nil
	}
	m.selectedID = row.Pipeline.ID
	m.detail = NewDetailModel(row)
	var cmds []tea.Cmd
	if m.FetchJobs != nil {
		cmds = append(cmds, m.FetchJobs(row.Pipeline.ProjectID, row.Pipeline.ID))
	}
	if m.FetchPipeline != nil {
		cmds = append(cmds, m.FetchPipeline(row.Pipeline.ProjectID, row.Pipeline.ID))
	}
	return m, tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		case "o":
			if len(m.pipelines) > 0 && m.cursor < len(m.pipelines) {
				if url := m.pipelines[m.cursor].Pipeline.WebURL; url != "" {
					openBrowser(url)
				}
			}
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.offset {
					m.offset = m.cursor
				}
				return m.selectPipeline()
			}
		case "down", "j":
			if m.cursor < len(m.pipelines)-1 {
				m.cursor++
				visible := m.visibleItems()
				if m.cursor >= m.offset+visible {
					m.offset = m.cursor - visible + 1
				}
				return m.selectPipeline()
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case PipelinesLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = formatError(msg.Err)
			return m, nil
		}
		m.err = nil
		m.pipelines = msg.Pipelines
		if m.cursor >= len(m.pipelines) {
			m.cursor = max(0, len(m.pipelines)-1)
		}
		m, cmd := m.selectPipeline()
		return m, cmd

	case JobsLoadedMsg:
		if m.detail != nil {
			m.detail.SetJobs(msg.Jobs, msg.Err)
			if m.detail.hasActiveJobs() {
				return m, scheduleDetailTick()
			}
		}

	case PipelineUpdatedMsg:
		if m.detail != nil && msg.Err == nil {
			m.detail.row.Pipeline = msg.Pipeline
		}

	case refreshTickMsg:
		var cmds []tea.Cmd
		if !m.loading && m.Refresh != nil {
			m.loading = true
			cmds = append(cmds, m.spinner.Tick, m.Refresh())
		}
		if m.detail != nil && m.FetchJobs != nil {
			row := m.detail.row
			cmds = append(cmds, m.FetchJobs(row.Pipeline.ProjectID, row.Pipeline.ID))
			if m.FetchPipeline != nil {
				cmds = append(cmds, m.FetchPipeline(row.Pipeline.ProjectID, row.Pipeline.ID))
			}
		}
		cmds = append(cmds, scheduleRefresh())
		return m, tea.Batch(cmds...)

	case detailTickMsg:
		if m.detail != nil && m.detail.hasActiveJobs() {
			m.detail.Tick()
			return m, scheduleDetailTick()
		}

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m Model) View() string {
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

	statusBar := m.renderStatusBar()

	paneHeight := max(m.height-1, 1)

	listW := m.listWidth()
	detailW := m.detailWidth()

	listPane := m.renderListPane(listW)
	detailPane := ""
	if m.detail != nil {
		detailPane = m.detail.Render(detailW, paneHeight)
	}

	listBox := lipgloss.NewStyle().Width(listW).Height(paneHeight).Render(listPane)
	detailBox := lipgloss.NewStyle().Width(detailW).Height(paneHeight).
		BorderLeft(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		Render(detailPane)

	panes := lipgloss.JoinHorizontal(lipgloss.Top, listBox, detailBox)
	return panes + "\n" + statusBar
}

func (m Model) renderStatusBar() string {
	left := ""
	if m.loading {
		left = dimStyle.Render("syncing...")
	}
	center := "↑/↓: navigate • o: open • r: refresh • q: quit"

	leftWidth := lipgloss.Width(left)
	centerWidth := lipgloss.Width(center)
	padding := (m.width - centerWidth) / 2

	leftPad := max(padding-leftWidth, 0)
	rightPad := max(m.width-leftWidth-leftPad-centerWidth, 0)

	content := left + strings.Repeat(" ", leftPad) + center + strings.Repeat(" ", rightPad)
	return statusBarStyle.Width(m.width).Render(content)
}

func (m Model) renderListPane(width int) string {
	header := listTitleStyle.Render("Pipelines")

	visible := m.visibleItems()
	end := min(m.offset+visible, len(m.pipelines))

	var rows []string
	for i := m.offset; i < end; i++ {
		selected := i == m.cursor
		rows = append(rows, renderPipelineItem(m.pipelines[i], width, selected))
	}

	return header + "\n\n" + strings.Join(rows, "\n\n")
}

func renderPipelineItem(p PipelineRow, width int, selected bool) string {
	padding := 2
	iconWidth := 4
	timeWidth := 12
	midWidth := max(width-iconWidth-timeWidth-padding*2, 20)

	bg := lipgloss.NewStyle()
	dim := dimStyle
	if selected {
		bg = selectedStyle
		dim = selectedStyle
	}

	icon := statusIconCompact(p.Pipeline.Status)
	iconCol := bg.
		Width(iconWidth).
		Height(2).
		Align(lipgloss.Center, lipgloss.Center).
		Render(icon)

	line1 := shortSHA(p.Pipeline.SHA) + " - " + truncate(p.Pipeline.Ref, midWidth-11)
	line2 := dim.Render(truncate(p.ProjectPath, midWidth))
	midCol := bg.
		Width(midWidth).
		Render(line1 + "\n" + line2)

	timeStr := dim.Render(formatTime(p.Pipeline.UpdatedAt))
	timeCol := bg.
		Width(timeWidth).
		Height(2).
		Align(lipgloss.Right, lipgloss.Center).
		Render(timeStr)

	row := lipgloss.JoinHorizontal(lipgloss.Top, iconCol, midCol, timeCol)
	rowStyle := lipgloss.NewStyle().PaddingLeft(padding).PaddingRight(padding)
	if selected {
		rowStyle = rowStyle.Background(selectedStyle.GetBackground()).Foreground(selectedStyle.GetForeground())
	}
	return rowStyle.Render(row)
}

func statusIconCompact(status string) string {
	switch status {
	case "success":
		return successStyle.Render("✓")
	case "failed":
		return failedStyle.Render("✗")
	case "running":
		return runningStyle.Render("●")
	case "pending":
		return pendingStyle.Render("○")
	case "canceled":
		return canceledStyle.Render("⊘")
	case "skipped":
		return skippedStyle.Render("⊘")
	default:
		return "?"
	}
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

func openBrowser(url string) {
	switch runtime.GOOS {
	case "darwin":
		exec.Command("open", url).Start()
	case "linux":
		exec.Command("xdg-open", url).Start()
	case "windows":
		exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
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

var (
	listTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).MarginLeft(2).MarginTop(1)
	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("236")).
			PaddingLeft(1).
			PaddingRight(1)
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).MarginLeft(2)
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).MarginLeft(2)
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	successStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	failedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	runningStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
	pendingStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	canceledStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	skippedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	selectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("238")).
			Foreground(lipgloss.Color("229"))
)
