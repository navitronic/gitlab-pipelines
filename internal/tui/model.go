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
	"github.com/navitronic/gitlab-builds/internal/pipeline"
)

const refreshInterval = 30 * time.Second

// PipelinesLoadedMsg signals that pipelines have been fetched.
type PipelinesLoadedMsg struct {
	Pipelines []PipelineRow
	Err       error
}

// PipelineUpdatedMsg signals that a single pipeline has been re-fetched.
type PipelineUpdatedMsg struct {
	Pipeline pipeline.Pipeline
	Err      error
}

// LoadingStatusMsg provides progress information during loading.
type LoadingStatusMsg struct {
	Status string
}

type refreshTickMsg struct{}
type detailTickMsg struct{}

// PipelineRow holds display data for one pipeline in the list.
type PipelineRow struct {
	Pipeline pipeline.Pipeline
}

// Model is the top-level Bubble Tea model.
type Model struct {
	spinner       spinner.Model
	pipelines     []PipelineRow
	cursor        int
	offset        int
	loading       bool
	loadingStatus string
	err           error
	width         int
	height        int
	detail        *DetailModel
	selectedID    string
	FetchJobs     func(projectID, pipelineID string) tea.Cmd
	FetchPipeline func(projectID, pipelineID string) tea.Cmd
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

func (m Model) visibleItems() int {
	available := m.height - 3
	if available < 4 {
		return 1
	}
	return available / 4
}

func (m Model) selectPipeline() (Model, tea.Cmd) {
	if len(m.pipelines) == 0 {
		m.detail = nil
		m.selectedID = ""
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

	case LoadingStatusMsg:
		m.loadingStatus = msg.Status
		return m, nil

	case PipelinesLoadedMsg:
		m.loading = false
		m.loadingStatus = ""
		if msg.Err != nil {
			m.err = formatError(msg.Err)
			if isFatalError(msg.Err) {
				m.pipelines = nil
				m.cursor = 0
				m.offset = 0
				m.detail = nil
				m.selectedID = ""
			}
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
		content := lipgloss.JoinVertical(lipgloss.Left, errMsg, "", hint)
		return lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Center, content)
	}
	if m.loading && len(m.pipelines) == 0 {
		status := "Loading pipelines..."
		if m.loadingStatus != "" {
			status = m.loadingStatus
		}
		content := m.spinner.View() + " " + status
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
	}
	if len(m.pipelines) == 0 {
		content := lipgloss.JoinVertical(lipgloss.Left,
			"No pipelines found.",
			"",
			helpStyle.Render("r: refresh • q: quit"),
		)
		return lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Center, content)
	}

	paneHeight := max(m.height-1, 1)
	listW := m.listWidth()
	detailW := m.detailWidth()

	listPane := m.renderListPane(listW, paneHeight)
	detailPane := ""
	if m.detail != nil {
		detailPane = m.detail.Render(detailW, paneHeight)
	}

	listBox := lipgloss.NewStyle().Width(listW).Height(paneHeight).Render(listPane)
	detailBox := lipgloss.NewStyle().
		Width(detailW).
		Height(paneHeight).
		BorderLeft(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		Render(detailPane)

	panes := lipgloss.JoinHorizontal(lipgloss.Top, listBox, detailBox)
	statusBar := m.renderStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left, panes, statusBar)
}

func (m Model) renderStatusBar() string {
	center := "↑/↓: navigate • o: open • r: refresh • q: quit"
	content := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, center)
	return statusBarStyle.Width(m.width).Render(content)
}

func (m Model) renderListPane(width, height int) string {
	header := listTitleStyle.Render("Pipelines")

	visible := m.visibleItems()
	end := min(m.offset+visible, len(m.pipelines))

	var rows []string
	for i := m.offset; i < end; i++ {
		selected := i == m.cursor
		rows = append(rows, renderPipelineItem(m.pipelines[i], width, selected))
	}

	listContent := header + "\n" + strings.Join(rows, "\n")

	if m.loading {
		status := "syncing..."
		if m.loadingStatus != "" {
			status = m.loadingStatus
		}
		toast := toastStyle.Render(m.spinner.View() + " " + status)
		toastHeight := lipgloss.Height(toast)
		topHeight := max(height-toastHeight, 1)

		top := lipgloss.NewStyle().Width(width).Height(topHeight).Render(listContent)
		return lipgloss.JoinVertical(lipgloss.Left, top, toast)
	}

	return listContent
}

func renderPipelineItem(p PipelineRow, width int, selected bool) string {
	innerWidth := max(width-4, 20)
	iconWidth := 3
	timeWidth := 12
	midWidth := max(innerWidth-iconWidth-timeWidth, 20)

	itemStyle := itemBaseStyle.Width(width)
	if selected {
		itemStyle = itemSelectedStyle.Width(width)
	}

	mid1 := truncateStr(shortSHA(p.Pipeline.SHA)+" - "+p.Pipeline.Ref, midWidth)
	mid2 := truncateStr(p.Pipeline.Project, midWidth)
	timeStr := formatTime(p.Pipeline.UpdatedAt)

	gap1 := max(innerWidth-iconWidth-lipgloss.Width(mid1)-lipgloss.Width(timeStr), 0)
	gap2 := max(innerWidth-iconWidth-lipgloss.Width(mid2), 0)

	if selected {
		icon := statusIconRaw(p.Pipeline.Status)
		pad := lipgloss.Width(icon) + 1
		row1 := icon + " " + mid1 + strings.Repeat(" ", gap1) + timeStr
		row2 := strings.Repeat(" ", pad) + mid2 + strings.Repeat(" ", gap2)
		return itemStyle.Render(row1 + "\n" + row2)
	}

	icon := statusIconCompact(p.Pipeline.Status)
	pad := lipgloss.Width(icon) + 1
	row1 := icon + " " + mid1 + strings.Repeat(" ", gap1) + timeStr
	row2 := strings.Repeat(" ", pad) + dimStyle.Render(mid2+strings.Repeat(" ", gap2))

	return itemStyle.Render(row1 + "\n" + row2)
}

func truncateStr(s string, maxWidth int) string {
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	for i := range s {
		if lipgloss.Width(s[:i]) >= maxWidth-1 {
			return s[:i] + "…"
		}
	}
	return s
}

func statusIconRaw(status pipeline.Status) string {
	switch status {
	case pipeline.StatusPassed:
		return "✓"
	case pipeline.StatusFailed:
		return "✗"
	case pipeline.StatusRunning:
		return "●"
	case pipeline.StatusPending:
		return "○"
	case pipeline.StatusCanceled, pipeline.StatusSkipped:
		return "⊘"
	default:
		return "?"
	}
}

func statusIconCompact(status pipeline.Status) string {
	switch status {
	case pipeline.StatusPassed:
		return successStyle.Render("✓")
	case pipeline.StatusFailed:
		return failedStyle.Render("✗")
	case pipeline.StatusRunning:
		return runningStyle.Render("●")
	case pipeline.StatusPending:
		return pendingStyle.Render("○")
	case pipeline.StatusCanceled:
		return canceledStyle.Render("⊘")
	case pipeline.StatusSkipped:
		return skippedStyle.Render("⊘")
	default:
		return "?"
	}
}

func statusIcon(status pipeline.Status) string {
	switch status {
	case pipeline.StatusPassed:
		return successStyle.Render("✓ passed")
	case pipeline.StatusFailed:
		return failedStyle.Render("✗ failed")
	case pipeline.StatusRunning:
		return runningStyle.Render("● running")
	case pipeline.StatusPending:
		return pendingStyle.Render("○ pending")
	case pipeline.StatusCanceled:
		return canceledStyle.Render("⊘ canceled")
	case pipeline.StatusSkipped:
		return skippedStyle.Render("⊘ skipped")
	default:
		return "unknown"
	}
}

func shortSHA(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
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
	if errors.Is(err, pipeline.ErrClientNotFound) {
		return fmt.Errorf("glab CLI not found. Install it: https://gitlab.com/gitlab-org/cli")
	}
	if errors.Is(err, pipeline.ErrAuthRequired) {
		return fmt.Errorf("glab not authenticated. Run: glab auth login")
	}
	return err
}

func isFatalError(err error) bool {
	return errors.Is(err, pipeline.ErrAuthRequired) || errors.Is(err, pipeline.ErrClientNotFound)
}
