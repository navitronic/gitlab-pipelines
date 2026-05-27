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
	"github.com/navitronic/gitlab-pipelines/internal/pipeline"
)

const refreshInterval = 30 * time.Second

// Pane identifies which pane has focus.
type Pane int

const (
	PanePipelines Pane = iota
	PaneDetail
)

// Layout mode based on terminal width.
type layoutMode int

const (
	layoutTwo layoutMode = iota // ≥80 cols: pipelines + detail
	layoutOne                   // <80: 1 pane
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

// PipelineGroup marks a group boundary in the flat pipeline list.
// Used for hierarchical display when grouping by project.
type PipelineGroup struct {
	Project string // Project name (e.g., "group/project")
	Start   int    // Index of first pipeline in this group
	Count   int    // Number of pipelines in this group
}

// Model is the top-level Bubble Tea model.
type Model struct {
	spinner        spinner.Model
	pipelines      []PipelineRow
	cursor         int
	offset         int
	focus          Pane
	loading        bool
	loadingStatus  string
	err            error
	width          int
	height         int
	detail         *DetailModel
	detailTicking  bool
	selectedID     string
	showLatestOnly bool
	FetchJobs      func(projectID, pipelineID string) tea.Cmd
	FetchPipeline  func(projectID, pipelineID string) tea.Cmd
	FetchMR        func(projectID, pipelineID, ref string) tea.Cmd
	Refresh        func() tea.Cmd
	HardRefresh    func() tea.Cmd
}

// New creates a new TUI model in loading state.
func New() Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return Model{
		spinner:        s,
		loading:        true,
		showLatestOnly: true,
		focus:          PanePipelines,
		width:          120,
		height:         24,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, scheduleRefresh())
}

func (m Model) visiblePipelines() []PipelineRow {
	if m.showLatestOnly {
		seen := make(map[string]bool)
		var result []PipelineRow
		for _, row := range m.pipelines {
			key := row.Pipeline.ProjectID + ":" + row.Pipeline.Ref
			if seen[key] {
				continue
			}
			seen[key] = true
			result = append(result, row)
		}
		return result
	}

	// In "all" mode: group by project, ordered by most recent pipeline per project
	// First, compute max UpdatedAt per project
	maxUpdatedPerProject := make(map[string]time.Time)
	for _, row := range m.pipelines {
		if row.Pipeline.UpdatedAt.After(maxUpdatedPerProject[row.Pipeline.Project]) {
			maxUpdatedPerProject[row.Pipeline.Project] = row.Pipeline.UpdatedAt
		}
	}

	// Stable sort: by (max UpdatedAt desc, ProjectID asc)
	// This groups by project and orders groups by recency, while preserving
	// chronological order within each group (due to stable sort)
	result := make([]PipelineRow, len(m.pipelines))
	copy(result, m.pipelines)
	sort.SliceStable(result, func(i, j int) bool {
		projI := result[i].Pipeline.Project
		projJ := result[j].Pipeline.Project
		if projI != projJ {
			// Different projects: order by max UpdatedAt desc
			maxI := maxUpdatedPerProject[projI]
			maxJ := maxUpdatedPerProject[projJ]
			if !maxI.Equal(maxJ) {
				return maxI.After(maxJ)
			}
			// Same max time: order by ProjectID asc
			return projI < projJ
		}
		// Same project: preserve original order (stable sort)
		return false
	})
	return result
}

func (m Model) pipelineGroups() []PipelineGroup {
	visible := m.visiblePipelines()
	if len(visible) == 0 {
		return []PipelineGroup{}
	}

	var groups []PipelineGroup
	currentProject := visible[0].Pipeline.Project
	startIdx := 0

	for i, row := range visible {
		if row.Pipeline.Project != currentProject {
			// End current group, start new one
			groups = append(groups, PipelineGroup{
				Project: currentProject,
				Start:   startIdx,
				Count:   i - startIdx,
			})
			currentProject = row.Pipeline.Project
			startIdx = i
		}
	}

	// Add final group
	groups = append(groups, PipelineGroup{
		Project: currentProject,
		Start:   startIdx,
		Count:   len(visible) - startIdx,
	})

	return groups
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
	if m.width >= 80 {
		return layoutTwo
	}
	return layoutOne
}

func (m Model) pipelinesPaneWidth() int {
	if m.layout() == layoutTwo {
		return min(max(m.width*2/5, 30), 50)
	}
	return m.width
}

func (m Model) detailPaneWidth() int {
	if m.layout() == layoutTwo {
		return m.width - m.pipelinesPaneWidth()
	}
	return m.width
}

func (m Model) visibleItems() int {
	available := m.height - 3
	if available < 4 {
		return 1
	}
	return available / 4
}

func (m Model) visibleItemsFromOffset(offset int) int {
	pipelines := m.visiblePipelines()
	if offset < 0 || offset >= len(pipelines) {
		return 0
	}

	width := m.pipelinesPaneWidth()
	if width <= 0 {
		width = m.width
	}

	header := listTitleStyle.Render("Pipelines")
	paneHeight := max(m.height-1, 1)
	availableLines := max(paneHeight-lipgloss.Height(header), 0)
	if availableLines == 0 {
		return 0
	}

	groups := m.pipelineGroups()
	showGroupHeaders := !m.showLatestOnly && len(groups) > 1
	showProject := !showGroupHeaders
	currentGroupIdx := 0
	for currentGroupIdx < len(groups) && offset >= groups[currentGroupIdx].Start+groups[currentGroupIdx].Count {
		currentGroupIdx++
	}

	linesUsed := 0
	count := 0
	seenRefs := make(map[string]bool)
	for i := range pipelines[:offset] {
		seenRefs[pipelines[i].Pipeline.ProjectID+":"+pipelines[i].Pipeline.Ref] = true
	}

	for i := offset; i < len(pipelines); i++ {
		if showGroupHeaders && currentGroupIdx < len(groups) && i == groups[currentGroupIdx].Start {
			header := renderGroupHeader(groups[currentGroupIdx].Project, groups[currentGroupIdx].Count, width)
			headerLines := lipgloss.Height(header)
			if linesUsed+headerLines > availableLines {
				break
			}
			linesUsed += headerLines
			currentGroupIdx++
		}

		key := pipelines[i].Pipeline.ProjectID + ":" + pipelines[i].Pipeline.Ref
		dimmed := seenRefs[key]
		seenRefs[key] = true
		item := renderPipelineItem(pipelines[i], width, i == m.cursor, dimmed, showProject)
		itemLines := lipgloss.Height(item)
		if linesUsed+itemLines > availableLines {
			break
		}
		linesUsed += itemLines
		count++
	}

	return count
}

func (m Model) selectPipeline() (Model, tea.Cmd) {
	visible := m.visiblePipelines()
	if len(visible) == 0 {
		m.detail = nil
		m.selectedID = ""
		return m, nil
	}
	row := visible[m.cursor]
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
		case "a":
			m.showLatestOnly = !m.showLatestOnly
			visible := m.visiblePipelines()
			if m.cursor >= len(visible) {
				m.cursor = max(0, len(visible)-1)
			}
			if m.offset > m.cursor {
				m.offset = m.cursor
			}
			m.detail = nil
			m.selectedID = ""
			return m.selectPipeline()
		case "left", "h":
			if m.focus > PanePipelines {
				m.focus--
			}
			return m, nil
		case "right", "l":
			if m.focus < PaneDetail && m.layout() == layoutTwo {
				m.focus++
			}
			return m, nil
		case "enter":
			if m.focus == PanePipelines && m.layout() != layoutTwo {
				m.focus = PaneDetail
				return m, nil
			}
		case "up", "k":
			switch m.focus {
			case PanePipelines:
				if m.cursor > 0 {
					m.cursor--
					if m.cursor < m.offset {
						m.offset = m.cursor
					}
					m.detail = nil
					m.selectedID = ""
					return m.selectPipeline()
				}
			}
		case "down", "j":
			switch m.focus {
			case PanePipelines:
				if m.cursor < len(m.visiblePipelines())-1 {
					m.cursor++
					visible := m.visibleItems()
					if m.cursor >= m.offset+visible {
						m.offset = m.cursor - visible + 1
					}
					m.detail = nil
					m.selectedID = ""
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
				m.cursor = 0
				m.offset = 0
				m.detail = nil
				m.selectedID = ""
			}
			return m, nil
		}
		m.err = nil
		m.pipelines = msg.Pipelines
		visible := m.visiblePipelines()
		if m.cursor >= len(visible) {
			m.cursor = max(0, len(visible)-1)
		}
		if m.offset > m.cursor {
			m.offset = m.cursor
		}
		m, cmd := m.selectPipeline()
		return m, cmd

	case JobsLoadedMsg:
		if m.detail != nil && msg.PipelineID == m.selectedID {
			m.detail.SetJobs(msg.Jobs, msg.Err)
			if m.detail.hasActiveJobs() && !m.detailTicking {
				m.detailTicking = true
				return m, scheduleDetailTick()
			}
		}

	case PipelineUpdatedMsg:
		if m.detail != nil && msg.Err == nil && msg.Pipeline.ID == m.selectedID {
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
	case layoutTwo:
		panes = m.renderTwoPane(paneHeight)
	default:
		panes = m.renderOnePane(paneHeight)
	}

	statusBar := m.renderStatusBar()
	return lipgloss.JoinVertical(lipgloss.Left, panes, statusBar)
}

func (m Model) renderTwoPane(height int) string {
	pipeW := m.pipelinesPaneWidth()
	detailW := m.detailPaneWidth()

	pipePane := m.renderPipelinesPane(pipeW, height)
	detailPane := ""
	if m.detail != nil {
		detailPane = m.detail.Render(detailW, height)
	}

	pipeBox := m.paneBox(pipePane, pipeW, height, m.focus == PanePipelines, false)
	detailBox := m.paneBox(detailPane, detailW, height, m.focus == PaneDetail, true)

	return lipgloss.JoinHorizontal(lipgloss.Top, pipeBox, detailBox)
}

func (m Model) renderOnePane(height int) string {
	w := m.width
	switch m.focus {
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

func (m Model) renderPipelinesPane(width, height int) string {
	header := listTitleStyle.Render("Pipelines")

	pipelines := m.visiblePipelines()
	groups := m.pipelineGroups()
	showGroupHeaders := !m.showLatestOnly && len(groups) > 1
	showProject := !showGroupHeaders
	availableLines := max(height-lipgloss.Height(header), 0)

	seenRefs := make(map[string]bool)
	for i := range pipelines[:m.offset] {
		seenRefs[pipelines[i].Pipeline.ProjectID+":"+pipelines[i].Pipeline.Ref] = true
	}

	rows := make([]string, 0, m.visibleItemsFromOffset(m.offset)+len(groups))
	linesUsed := 0
	currentGroupIdx := 0
	for currentGroupIdx < len(groups) && m.offset >= groups[currentGroupIdx].Start+groups[currentGroupIdx].Count {
		currentGroupIdx++
	}

	for i := m.offset; i < len(pipelines); i++ {
		if showGroupHeaders && currentGroupIdx < len(groups) && i == groups[currentGroupIdx].Start {
			groupHeader := renderGroupHeader(groups[currentGroupIdx].Project, groups[currentGroupIdx].Count, width)
			headerLines := lipgloss.Height(groupHeader)
			if linesUsed+headerLines > availableLines {
				break
			}
			rows = append(rows, groupHeader)
			linesUsed += headerLines
			currentGroupIdx++
		}

		selected := i == m.cursor
		key := pipelines[i].Pipeline.ProjectID + ":" + pipelines[i].Pipeline.Ref
		dimmed := seenRefs[key]
		seenRefs[key] = true
		item := renderPipelineItem(pipelines[i], width, selected, dimmed, showProject)
		itemLines := lipgloss.Height(item)
		if linesUsed+itemLines > availableLines {
			break
		}
		rows = append(rows, item)
		linesUsed += itemLines
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

func (m Model) renderStatusBar() string {
	var parts []string
	parts = append(parts, "↑/↓: navigate", "o: open")
	if m.showLatestOnly {
		parts = append(parts, "a: show all")
	} else {
		parts = append(parts, "a: latest only")
	}
	parts = append(parts, "r: refresh", "R: hard refresh", "q: quit")
	center := strings.Join(parts, " • ")
	content := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, center)
	return statusBarStyle.Width(m.width).Render(content)
}

func renderPipelineItem(p PipelineRow, width int, selected, dimmed, showProject bool) string {
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
	buildRows := func(icon, row1Text, row2Text string) string {
		row1 := icon + " " + row1Text
		if !showProject {
			return itemStyle.Render(row1)
		}

		pad := lipgloss.Width(statusIconRaw(p.Pipeline.Status)) + 1
		row2 := strings.Repeat(" ", pad) + row2Text
		return itemStyle.Render(row1 + "\n" + row2)
	}

	if selected {
		icon := statusIconRaw(p.Pipeline.Status)
		return buildRows(icon, mid1+strings.Repeat(" ", gap1)+timeStr, mid2+strings.Repeat(" ", gap2))
	}

	if dimmed {
		icon := dimStyle.Render(statusIconRaw(p.Pipeline.Status))
		return buildRows(icon, dimStyle.Render(mid1+strings.Repeat(" ", gap1)+timeStr), dimStyle.Render(mid2+strings.Repeat(" ", gap2)))
	}

	icon := statusIconCompact(p.Pipeline.Status)
	return buildRows(icon, mid1+strings.Repeat(" ", gap1)+timeStr, dimStyle.Render(mid2+strings.Repeat(" ", gap2)))
}

func renderGroupHeader(project string, count int, width int) string {
	text := fmt.Sprintf("▸ %s (%d)", project, count)

	// If text fits within width, render and return
	if lipgloss.Width(text) <= width {
		return groupHeaderStyle.Render(text)
	}

	// Truncate project name to fit within width
	// Reserve space for: "▸ " (2) + " (N)" (3 + len(count_str))
	countStr := fmt.Sprintf("%d", count)
	reserved := 2 + 3 + len(countStr) // "▸ " + " ()" + count digits
	maxProjectWidth := width - reserved

	if maxProjectWidth < 1 {
		// Width too narrow, just return arrow and count
		return groupHeaderStyle.Render(fmt.Sprintf("▸ (%s)", countStr))
	}

	truncatedProject := truncateStr(project, maxProjectWidth)
	text = fmt.Sprintf("▸ %s (%d)", truncatedProject, count)
	return groupHeaderStyle.Render(text)
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
