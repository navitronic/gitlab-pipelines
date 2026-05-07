package tui

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/navitronic/gitlab-builds/internal/pipeline"
)

const refreshInterval = 30 * time.Second

// Pane identifies which pane has focus.
type Pane int

const (
	PaneRepos Pane = iota
	PanePipelines
	PaneDetail
)

// Layout mode based on terminal width.
type layoutMode int

const (
	layoutThree layoutMode = iota // ≥120 cols: all 3 panes
	layoutTwo   layoutMode = iota // 80-119: 2 panes
	layoutOne   layoutMode = iota // <80: 1 pane
)

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
	repos         []string
	repoCursor    int
	repoOffset    int
	selectedRepo  string
	filtered      []PipelineRow
	cursor        int
	offset        int
	focus         Pane
	loading       bool
	loadingStatus string
	err           error
	width         int
	height        int
	detail        *DetailModel
	detailTicking bool
	selectedID    string
	FetchJobs     func(projectID, pipelineID string) tea.Cmd
	FetchPipeline func(projectID, pipelineID string) tea.Cmd
	FetchMR       func(projectID, pipelineID, ref string) tea.Cmd
	Refresh       func() tea.Cmd
	HardRefresh   func() tea.Cmd
}

// New creates a new TUI model in loading state.
func New() Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return Model{
		spinner: s,
		loading: true,
		focus:   PaneRepos,
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

func (m Model) layout() layoutMode {
	if m.width >= 120 {
		return layoutThree
	}
	if m.width >= 80 {
		return layoutTwo
	}
	return layoutOne
}

func (m Model) reposPaneWidth() int {
	switch m.layout() {
	case layoutThree:
		return min(max(m.width/4, 20), 40)
	case layoutTwo:
		return min(max(m.width/3, 20), 35)
	default:
		return m.width
	}
}

func (m Model) pipelinesPaneWidth() int {
	switch m.layout() {
	case layoutThree:
		w := m.width - m.reposPaneWidth()
		return w * 2 / 5
	case layoutTwo:
		return m.width - m.reposPaneWidth()
	default:
		return m.width
	}
}

func (m Model) detailPaneWidth() int {
	switch m.layout() {
	case layoutThree:
		return m.width - m.reposPaneWidth() - m.pipelinesPaneWidth()
	default:
		return m.width
	}
}

func (m Model) visibleRepos() int {
	available := m.height - 4
	if available < 1 {
		return 1
	}
	return available
}

func (m Model) visibleItems() int {
	available := m.height - 3
	if available < 4 {
		return 1
	}
	return available / 4
}

func (m *Model) deriveRepos() {
	seen := make(map[string]bool)
	var repos []string
	for _, row := range m.pipelines {
		if !seen[row.Pipeline.Project] {
			seen[row.Pipeline.Project] = true
			repos = append(repos, row.Pipeline.Project)
		}
	}
	sort.Strings(repos)
	m.repos = repos
	if m.repoCursor >= len(m.repos) {
		m.repoCursor = max(0, len(m.repos)-1)
	}
	if len(m.repos) > 0 {
		if m.selectedRepo == "" {
			m.selectedRepo = m.repos[m.repoCursor]
		}
	} else {
		m.selectedRepo = ""
	}
	m.filterPipelines()
}

func (m *Model) filterPipelines() {
	m.filtered = nil
	for _, row := range m.pipelines {
		if row.Pipeline.Project == m.selectedRepo {
			m.filtered = append(m.filtered, row)
		}
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
	if m.offset > m.cursor {
		m.offset = m.cursor
	}
}

func (m Model) selectRepo() Model {
	if len(m.repos) == 0 {
		return m
	}
	repo := m.repos[m.repoCursor]
	if repo == m.selectedRepo {
		return m
	}
	m.selectedRepo = repo
	m.cursor = 0
	m.offset = 0
	m.filterPipelines()
	m.detail = nil
	m.selectedID = ""
	return m
}

func (m Model) selectPipeline() (Model, tea.Cmd) {
	if len(m.filtered) == 0 {
		m.detail = nil
		m.selectedID = ""
		return m, nil
	}
	row := m.filtered[m.cursor]
	if row.Pipeline.ID == m.selectedID {
		return m, nil
	}
	m.selectedID = row.Pipeline.ID
	m.detail = NewDetailModel(row)
	m.detailTicking = false
	var cmds []tea.Cmd
	if m.FetchJobs != nil {
		cmds = append(cmds, m.FetchJobs(row.Pipeline.ProjectID, row.Pipeline.ID))
	}
	if m.FetchPipeline != nil {
		cmds = append(cmds, m.FetchPipeline(row.Pipeline.ProjectID, row.Pipeline.ID))
	}
	if m.FetchMR != nil {
		cmds = append(cmds, m.FetchMR(row.Pipeline.ProjectID, row.Pipeline.ID, row.Pipeline.Ref))
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
		case "R":
			if !m.loading && m.HardRefresh != nil {
				m.loading = true
				m.err = nil
				return m, tea.Batch(m.spinner.Tick, m.HardRefresh())
			}
		case "o":
			if m.detail != nil {
				if url := m.detail.row.Pipeline.WebURL; url != "" {
					openBrowser(url)
				}
			}
		case "left", "h":
			if m.focus > PaneRepos {
				m.focus--
			}
			return m, nil
		case "right", "l":
			maxPane := PaneDetail
			if m.layout() == layoutTwo {
				maxPane = PanePipelines
			}
			if m.focus < maxPane {
				m.focus++
			}
			return m, nil
		case "enter":
			if m.focus == PaneRepos {
				m.focus = PanePipelines
				return m, nil
			}
			if m.focus == PanePipelines && m.layout() != layoutThree {
				m.focus = PaneDetail
				return m, nil
			}
		case "up", "k":
			switch m.focus {
			case PaneRepos:
				if m.repoCursor > 0 {
					m.repoCursor--
					if m.repoCursor < m.repoOffset {
						m.repoOffset = m.repoCursor
					}
					m = m.selectRepo()
					return m.selectPipeline()
				}
			case PanePipelines:
				if m.cursor > 0 {
					m.cursor--
					if m.cursor < m.offset {
						m.offset = m.cursor
					}
					return m.selectPipeline()
				}
			}
		case "down", "j":
			switch m.focus {
			case PaneRepos:
				if m.repoCursor < len(m.repos)-1 {
					m.repoCursor++
					visible := m.visibleRepos()
					if m.repoCursor >= m.repoOffset+visible {
						m.repoOffset = m.repoCursor - visible + 1
					}
					m = m.selectRepo()
					return m.selectPipeline()
				}
			case PanePipelines:
				if m.cursor < len(m.filtered)-1 {
					m.cursor++
					visible := m.visibleItems()
					if m.cursor >= m.offset+visible {
						m.offset = m.cursor - visible + 1
					}
					return m.selectPipeline()
				}
			}
		case "esc":
			if m.focus == PaneDetail {
				m.focus = PanePipelines
				return m, nil
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
				m.repos = nil
				m.filtered = nil
				m.cursor = 0
				m.offset = 0
				m.repoCursor = 0
				m.repoOffset = 0
				m.detail = nil
				m.selectedID = ""
				m.selectedRepo = ""
			}
			return m, nil
		}
		m.err = nil
		m.pipelines = msg.Pipelines
		m.deriveRepos()
		m, cmd := m.selectPipeline()
		return m, cmd

	case JobsLoadedMsg:
		if m.detail != nil {
			m.detail.SetJobs(msg.Jobs, msg.Err)
			if m.detail.hasActiveJobs() && !m.detailTicking {
				m.detailTicking = true
				return m, scheduleDetailTick()
			}
		}

	case PipelineUpdatedMsg:
		if m.detail != nil && msg.Err == nil {
			if msg.Pipeline.Project == "" {
				msg.Pipeline.Project = m.detail.row.Pipeline.Project
			}
			m.detail.row.Pipeline = msg.Pipeline
		}

	case MRLoadedMsg:
		if m.detail != nil && msg.Err == nil && msg.PipelineID == m.selectedID {
			m.detail.SetMRURL(msg.URL)
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
		m.detailTicking = false

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

	var panes string
	switch m.layout() {
	case layoutThree:
		panes = m.renderThreePane(paneHeight)
	case layoutTwo:
		panes = m.renderTwoPane(paneHeight)
	default:
		panes = m.renderOnePane(paneHeight)
	}

	statusBar := m.renderStatusBar()
	return lipgloss.JoinVertical(lipgloss.Left, panes, statusBar)
}

func (m Model) renderThreePane(height int) string {
	repoW := m.reposPaneWidth()
	pipeW := m.pipelinesPaneWidth()
	detailW := m.detailPaneWidth()

	repoPane := m.renderReposPane(repoW, height)
	pipePane := m.renderPipelinesPane(pipeW, height)
	detailPane := ""
	if m.detail != nil {
		detailPane = m.detail.Render(detailW, height)
	}

	repoBox := m.paneBox(repoPane, repoW, height, m.focus == PaneRepos, false)
	pipeBox := m.paneBox(pipePane, pipeW, height, m.focus == PanePipelines, true)
	detailBox := m.paneBox(detailPane, detailW, height, m.focus == PaneDetail, true)

	return lipgloss.JoinHorizontal(lipgloss.Top, repoBox, pipeBox, detailBox)
}

func (m Model) renderTwoPane(height int) string {
	repoW := m.reposPaneWidth()
	pipeW := m.pipelinesPaneWidth()

	switch m.focus {
	case PaneDetail:
		pipePane := m.renderPipelinesPane(repoW, height)
		detailPane := ""
		if m.detail != nil {
			detailPane = m.detail.Render(pipeW, height)
		}
		pipeBox := m.paneBox(pipePane, repoW, height, false, false)
		detailBox := m.paneBox(detailPane, pipeW, height, true, true)
		return lipgloss.JoinHorizontal(lipgloss.Top, pipeBox, detailBox)
	default:
		repoPane := m.renderReposPane(repoW, height)
		pipePane := m.renderPipelinesPane(pipeW, height)
		repoBox := m.paneBox(repoPane, repoW, height, m.focus == PaneRepos, false)
		pipeBox := m.paneBox(pipePane, pipeW, height, m.focus == PanePipelines, true)
		return lipgloss.JoinHorizontal(lipgloss.Top, repoBox, pipeBox)
	}
}

func (m Model) renderOnePane(height int) string {
	w := m.width
	switch m.focus {
	case PaneRepos:
		return lipgloss.NewStyle().Width(w).Height(height).Render(m.renderReposPane(w, height))
	case PanePipelines:
		return lipgloss.NewStyle().Width(w).Height(height).Render(m.renderPipelinesPane(w, height))
	default:
		content := ""
		if m.detail != nil {
			content = m.detail.Render(w, height)
		}
		return lipgloss.NewStyle().Width(w).Height(height).Render(content)
	}
}

func (m Model) paneBox(content string, width, height int, focused, borderLeft bool) string {
	borderColor := lipgloss.Color("238")
	if focused {
		borderColor = lipgloss.Color("205")
	}
	style := lipgloss.NewStyle().Width(width).Height(height)
	if borderLeft {
		style = style.BorderLeft(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(borderColor)
	}
	return style.Render(content)
}

func (m Model) renderReposPane(width, height int) string {
	header := listTitleStyle.Render("Repos")

	visible := m.visibleRepos()
	end := min(m.repoOffset+visible, len(m.repos))

	var rows []string
	for i := m.repoOffset; i < end; i++ {
		selected := i == m.repoCursor
		name := repoShortName(m.repos[i])
		name = truncateStr(name, width-4)
		if selected {
			rows = append(rows, repoSelectedStyle.Width(width).Render(name))
		} else {
			rows = append(rows, repoItemStyle.Width(width).Render(name))
		}
	}

	content := header + "\n" + strings.Join(rows, "\n")

	if m.loading {
		status := "syncing..."
		if m.loadingStatus != "" {
			status = m.loadingStatus
		}
		toast := toastStyle.Render(m.spinner.View() + " " + status)
		toastHeight := lipgloss.Height(toast)
		topHeight := max(height-toastHeight, 1)
		top := lipgloss.NewStyle().Width(width).Height(topHeight).Render(content)
		return lipgloss.JoinVertical(lipgloss.Left, top, toast)
	}

	return content
}

func (m Model) renderPipelinesPane(width, height int) string {
	header := listTitleStyle.Render("Pipelines")

	visible := m.visibleItems()
	end := min(m.offset+visible, len(m.filtered))

	var rows []string
	for i := m.offset; i < end; i++ {
		selected := i == m.cursor
		rows = append(rows, renderPipelineItem(m.filtered[i], width, selected))
	}

	listContent := header + "\n" + strings.Join(rows, "\n")

	if m.loading && m.focus == PanePipelines {
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

func (m Model) renderStatusBar() string {
	var parts []string
	parts = append(parts, "←/→: pane", "↑/↓: navigate", "o: open")
	parts = append(parts, "r: refresh", "R: hard refresh", "q: quit")
	center := strings.Join(parts, " • ")
	content := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, center)
	return statusBarStyle.Width(m.width).Render(content)
}

func repoShortName(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) <= 2 {
		return path
	}
	return strings.Join(parts[len(parts)-2:], "/")
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
