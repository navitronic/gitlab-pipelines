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

type RefSubGroup struct {
	Ref   string
	Start int
	Count int
}

type ProjectGroup struct {
	Project string
	Start   int
	Count   int
	Refs    []RefSubGroup
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

	// In "all" mode: group by project, then ref.
	maxUpdatedPerProject := make(map[string]time.Time)
	maxUpdatedPerRef := make(map[string]time.Time)
	for _, row := range m.pipelines {
		project := row.Pipeline.Project
		refKey := project + "\x00" + row.Pipeline.Ref
		if row.Pipeline.UpdatedAt.After(maxUpdatedPerProject[project]) {
			maxUpdatedPerProject[project] = row.Pipeline.UpdatedAt
		}
		if row.Pipeline.UpdatedAt.After(maxUpdatedPerRef[refKey]) {
			maxUpdatedPerRef[refKey] = row.Pipeline.UpdatedAt
		}
	}

	result := make([]PipelineRow, len(m.pipelines))
	copy(result, m.pipelines)
	sort.SliceStable(result, func(i, j int) bool {
		projI := result[i].Pipeline.Project
		projJ := result[j].Pipeline.Project
		if projI != projJ {
			maxI := maxUpdatedPerProject[projI]
			maxJ := maxUpdatedPerProject[projJ]
			if !maxI.Equal(maxJ) {
				return maxI.After(maxJ)
			}
			return projI < projJ
		}

		refI := result[i].Pipeline.Ref
		refJ := result[j].Pipeline.Ref
		if refI != refJ {
			maxI := maxUpdatedPerRef[projI+"\x00"+refI]
			maxJ := maxUpdatedPerRef[projJ+"\x00"+refJ]
			if !maxI.Equal(maxJ) {
				return maxI.After(maxJ)
			}
			return refI < refJ
		}

		return false
	})
	return result
}

func (m Model) pipelineHierarchy() []ProjectGroup {
	visible := m.visiblePipelines()
	if len(visible) == 0 {
		return []ProjectGroup{}
	}

	groups := make([]ProjectGroup, 0, len(visible))
	for i, row := range visible {
		project := row.Pipeline.Project
		ref := row.Pipeline.Ref

		if len(groups) == 0 || groups[len(groups)-1].Project != project {
			groups = append(groups, ProjectGroup{
				Project: project,
				Start:   i,
				Refs: []RefSubGroup{{
					Ref:   ref,
					Start: i,
					Count: 1,
				}},
			})
			continue
		}

		projectGroup := &groups[len(groups)-1]
		lastRef := len(projectGroup.Refs) - 1
		if lastRef < 0 || projectGroup.Refs[lastRef].Ref != ref {
			projectGroup.Refs = append(projectGroup.Refs, RefSubGroup{
				Ref:   ref,
				Start: i,
				Count: 1,
			})
		} else {
			projectGroup.Refs[lastRef].Count++
		}
	}

	for i := range groups {
		count := 0
		for _, ref := range groups[i].Refs {
			count += ref.Count
		}
		groups[i].Count = count
	}

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

	linesUsed := 0
	count := 0
	if m.showLatestOnly {
		for i := offset; i < len(pipelines); i++ {
			item := renderPipelineItem(pipelines[i], width, i == m.cursor, "", false)
			itemLines := lipgloss.Height(item)
			if linesUsed+itemLines > availableLines {
				break
			}
			linesUsed += itemLines
			count++
		}
		return count
	}

	hierarchy := m.pipelineHierarchy()
	for projectIdx, project := range hierarchy {
		projectEnd := project.Start + project.Count
		if offset >= projectEnd {
			continue
		}

		projectHeader := renderProjectHeader(project.Project, projectIdx == len(hierarchy)-1, width)
		projectHeaderLines := lipgloss.Height(projectHeader)
		if linesUsed+projectHeaderLines > availableLines {
			break
		}
		linesUsed += projectHeaderLines

		for refIdx, ref := range project.Refs {
			refEnd := ref.Start + ref.Count
			if offset >= refEnd {
				continue
			}

			refHeader := renderRefHeader(ref.Ref, ref.Count, projectIdx == len(hierarchy)-1, refIdx == len(project.Refs)-1, width)
			refHeaderLines := lipgloss.Height(refHeader)
			if linesUsed+refHeaderLines > availableLines {
				return count
			}
			linesUsed += refHeaderLines

			start := max(offset, ref.Start)
			for i := start; i < refEnd; i++ {
				item := renderPipelineItem(pipelines[i], width, i == m.cursor, treePrefix(projectIdx == len(hierarchy)-1, refIdx == len(project.Refs)-1, i == refEnd-1, "item"), true)
				itemLines := lipgloss.Height(item)
				if linesUsed+itemLines > availableLines {
					return count
				}
				linesUsed += itemLines
				count++
			}
		}
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
			// Preserve cursor by pipeline ID
			var preserveID string
			visible := m.visiblePipelines()
			if m.cursor < len(visible) {
				preserveID = visible[m.cursor].Pipeline.ID
			}

			m.showLatestOnly = !m.showLatestOnly
			newVisible := m.visiblePipelines()

			// Find same pipeline in new view
			newCursor := 0
			for i, row := range newVisible {
				if row.Pipeline.ID == preserveID {
					newCursor = i
					break
				}
			}
			m.cursor = newCursor

			// Adjust offset if needed
			if m.cursor >= len(newVisible) {
				m.cursor = max(0, len(newVisible)-1)
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
					visible := m.visibleItemsFromOffset(m.offset)
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
	hierarchy := m.pipelineHierarchy()
	showTree := !m.showLatestOnly
	availableLines := max(height-lipgloss.Height(header), 0)

	rows := make([]string, 0, m.visibleItemsFromOffset(m.offset)+len(hierarchy)*2)
	linesUsed := 0
	if !showTree {
		for i := m.offset; i < len(pipelines); i++ {
			selected := i == m.cursor
			item := renderPipelineItem(pipelines[i], width, selected, "", false)
			itemLines := lipgloss.Height(item)
			if linesUsed+itemLines > availableLines {
				break
			}
			rows = append(rows, item)
			linesUsed += itemLines
		}
	} else {
		for projectIdx, project := range hierarchy {
			projectEnd := project.Start + project.Count
			if m.offset >= projectEnd {
				continue
			}

			projectHeader := renderProjectHeader(project.Project, projectIdx == len(hierarchy)-1, width)
			projectHeaderLines := lipgloss.Height(projectHeader)
			if linesUsed+projectHeaderLines > availableLines {
				break
			}
			rows = append(rows, projectHeader)
			linesUsed += projectHeaderLines

			for refIdx, ref := range project.Refs {
				refEnd := ref.Start + ref.Count
				if m.offset >= refEnd {
					continue
				}

				refHeader := renderRefHeader(ref.Ref, ref.Count, projectIdx == len(hierarchy)-1, refIdx == len(project.Refs)-1, width)
				refHeaderLines := lipgloss.Height(refHeader)
				if linesUsed+refHeaderLines > availableLines {
					goto done
				}
				rows = append(rows, refHeader)
				linesUsed += refHeaderLines

				start := max(m.offset, ref.Start)
				for i := start; i < refEnd; i++ {
					selected := i == m.cursor
					item := renderPipelineItem(pipelines[i], width, selected, treePrefix(projectIdx == len(hierarchy)-1, refIdx == len(project.Refs)-1, i == refEnd-1, "item"), true)
					itemLines := lipgloss.Height(item)
					if linesUsed+itemLines > availableLines {
						goto done
					}
					rows = append(rows, item)
					linesUsed += itemLines
				}
			}
		}
	}

done:

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

func renderPipelineItem(p PipelineRow, width int, selected bool, prefix string, grouped bool) string {
	var itemStyle lipgloss.Style
	if grouped {
		itemStyle = itemTreeStyle.Width(width)
		if selected {
			itemStyle = itemTreeSelectedStyle.Width(width)
		}
	} else {
		itemStyle = itemBaseStyle.Width(width)
		if selected {
			itemStyle = itemSelectedStyle.Width(width)
		}
	}

	innerWidth := max(width, 20)
	if !grouped {
		innerWidth = max(width-4, 20) // account for padding in flat mode
	}
	prefixWidth := lipgloss.Width(prefix)
	iconWidth := prefixWidth + 2
	timeWidth := 12
	midWidth := max(innerWidth-iconWidth-timeWidth, 8)

	row1Content := shortSHA(p.Pipeline.SHA)
	if !grouped {
		row1Content += " - " + p.Pipeline.Ref
	}
	mid1 := truncateStr(row1Content, midWidth)
	mid2 := truncateStr(p.Pipeline.Project, midWidth)
	timeStr := formatTime(p.Pipeline.UpdatedAt)

	gap1 := max(innerWidth-iconWidth-lipgloss.Width(mid1)-lipgloss.Width(timeStr), 0)
	gap2 := max(innerWidth-iconWidth-lipgloss.Width(mid2), 0)
	buildRows := func(iconRaw, iconRendered, row1Text, row2Text string) string {
		row1 := treeStyle.Render(prefix) + iconRendered + " " + row1Text
		if grouped {
			return itemStyle.Render(row1)
		}

		pad := prefixWidth + lipgloss.Width(iconRaw) + 1
		row2 := strings.Repeat(" ", pad) + row2Text
		return itemStyle.Render(row1 + "\n" + row2)
	}

	if selected {
		icon := statusIconRaw(p.Pipeline.Status)
		return buildRows(icon, icon, mid1+strings.Repeat(" ", gap1)+timeStr, mid2+strings.Repeat(" ", gap2))
	}

	iconRaw := statusIconRaw(p.Pipeline.Status)
	icon := statusIconCompact(p.Pipeline.Status)
	return buildRows(iconRaw, icon, mid1+strings.Repeat(" ", gap1)+timeStr, mid2+strings.Repeat(" ", gap2))
}

func treePrefix(isLastProject, isLastRef, isLastItem bool, level string) string {
	switch level {
	case "project":
		if isLastProject {
			return "└─ "
		}
		return "├─ "
	case "ref":
		projectPrefix := "│  "
		if isLastProject {
			projectPrefix = "   "
		}
		if isLastRef {
			return projectPrefix + "└─ "
		}
		return projectPrefix + "├─ "
	case "item":
		projectPrefix := "│  "
		if isLastProject {
			projectPrefix = "   "
		}
		refPrefix := "│  "
		if isLastRef {
			refPrefix = "   "
		}
		if isLastItem {
			return projectPrefix + refPrefix + "└─ "
		}
		return projectPrefix + refPrefix + "├─ "
	default:
		return ""
	}
}

func renderProjectHeader(project string, isLast bool, width int) string {
	prefix := treePrefix(isLast, false, false, "project")
	text := truncateStr(project, max(width-lipgloss.Width(prefix), 1))
	return lipgloss.JoinHorizontal(lipgloss.Top, treeStyle.Render(prefix), projectHeaderStyle.Render(text))
}

func renderRefHeader(ref string, count int, isLastProject, isLastRef bool, width int) string {
	prefix := treePrefix(isLastProject, isLastRef, false, "ref")
	suffix := fmt.Sprintf(" (%d)", count)
	text := truncateStr(ref, max(width-lipgloss.Width(prefix)-lipgloss.Width(suffix), 1)) + suffix
	return lipgloss.JoinHorizontal(lipgloss.Top, treeStyle.Render(prefix), refHeaderStyle.Render(text))
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
