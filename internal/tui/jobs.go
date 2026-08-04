package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/navitronic/gitlab-pipelines/internal/pipeline"
)

// RepoJobsLoadedMsg signals that today's jobs for a repo have been fetched.
type RepoJobsLoadedMsg struct {
	Jobs []pipeline.Job
	Err  error
}

// JobsModel is the top-level Bubble Tea model for the -jobs view: an
// aggregate table of today's job totals for a repo, plus a paged list of
// the individual jobs.
type JobsModel struct {
	spinner       spinner.Model
	repo          string
	stages        []string
	jobs          []pipeline.Job
	cursor        int
	offset        int
	loading       bool
	loadingStatus string
	err           error
	width         int
	height        int
	Refresh       func() tea.Cmd
}

// NewJobsModel creates a new jobs TUI model in loading state for the given
// repo. If stages is non-empty, it is shown in the header as an indication
// that the job list and totals are filtered.
func NewJobsModel(repo string, stages []string) JobsModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return JobsModel{
		spinner: s,
		repo:    repo,
		stages:  stages,
		loading: true,
		width:   120,
		height:  24,
	}
}

func (m JobsModel) Init() tea.Cmd {
	cmds := []tea.Cmd{m.spinner.Tick, scheduleRefresh()}
	if m.Refresh != nil {
		cmds = append(cmds, m.Refresh())
	}
	return tea.Batch(cmds...)
}

func (m JobsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			if m.cursor >= 0 && m.cursor < len(m.jobs) {
				if url := m.jobs[m.cursor].WebURL; url != "" {
					openBrowser(url)
				}
			}
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.offset {
					m.offset = m.cursor
				}
			}
		case "down", "j":
			if m.cursor < len(m.jobs)-1 {
				m.cursor++
				visible := m.listHeight()
				if m.cursor >= m.offset+visible {
					m.offset = m.cursor - visible + 1
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case LoadingStatusMsg:
		m.loadingStatus = msg.Status
		return m, nil

	case RepoJobsLoadedMsg:
		m.loading = false
		m.loadingStatus = ""
		if msg.Err != nil {
			m.err = formatError(msg.Err)
			if isFatalError(msg.Err) {
				m.jobs = nil
				m.cursor = 0
				m.offset = 0
			}
			return m, nil
		}
		m.err = nil
		m.jobs = msg.Jobs
		if m.cursor >= len(m.jobs) {
			m.cursor = max(0, len(m.jobs)-1)
		}
		if m.offset > m.cursor {
			m.offset = m.cursor
		}
		return m, nil

	case refreshTickMsg:
		var cmds []tea.Cmd
		if !m.loading && m.Refresh != nil {
			m.loading = true
			cmds = append(cmds, m.spinner.Tick, m.Refresh())
		}
		cmds = append(cmds, scheduleRefresh())
		return m, tea.Batch(cmds...)

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m JobsModel) View() string {
	if m.err != nil && len(m.jobs) == 0 {
		errMsg := errorStyle.Render(fmt.Sprintf("Error: %v", m.err))
		hint := helpStyle.Render("r: retry • q: quit")
		content := lipgloss.JoinVertical(lipgloss.Left, errMsg, "", hint)
		return lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Center, content)
	}
	if m.loading && len(m.jobs) == 0 {
		status := "Loading jobs..."
		if m.loadingStatus != "" {
			status = m.loadingStatus
		}
		content := m.spinner.View() + " " + status
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
	}

	top := m.renderTop()
	list := m.renderJobList(m.listHeight())
	statusBar := m.renderStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left, top, list, statusBar)
}

func (m JobsModel) renderTop() string {
	header := listTitleStyle.Render(jobsHeaderText(m.repo, m.stages))
	rows, total := summarizeJobs(m.jobs)
	summary := renderJobSummaryTable(rows, total, m.width)
	jobsHeader := jobHeaderStyle.Render(fmt.Sprintf("Jobs (%d)", len(m.jobs)))
	return lipgloss.JoinVertical(lipgloss.Left, header, "", summary, "", jobsHeader)
}

func jobsHeaderText(repo string, stages []string) string {
	if len(stages) == 0 {
		return fmt.Sprintf("Jobs for %s — Today", repo)
	}
	return fmt.Sprintf("Jobs for %s — Today (stages: %s)", repo, strings.Join(stages, ", "))
}

// listHeight returns how many job rows fit below the summary table and
// above the status bar.
func (m JobsModel) listHeight() int {
	top := m.renderTop()
	statusBarHeight := 1
	return max(m.height-lipgloss.Height(top)-statusBarHeight, 1)
}

func (m JobsModel) renderJobList(height int) string {
	if len(m.jobs) == 0 {
		return dimStyle.Render("  No jobs found today.")
	}

	contentHeight := height
	var toast string
	if m.loading {
		status := "syncing..."
		if m.loadingStatus != "" {
			status = m.loadingStatus
		}
		toast = toastStyle.Render(m.spinner.View() + " " + status)
		contentHeight = max(height-lipgloss.Height(toast), 1)
	}

	end := min(m.offset+contentHeight, len(m.jobs))
	rows := make([]string, 0, end-m.offset)
	for i := m.offset; i < end; i++ {
		rows = append(rows, renderJobRow(m.jobs[i], m.width, i == m.cursor))
	}
	listContent := strings.Join(rows, "\n")

	if toast != "" {
		top := lipgloss.NewStyle().Width(m.width).Height(contentHeight).Render(listContent)
		return lipgloss.JoinVertical(lipgloss.Left, top, toast)
	}
	return listContent
}

func (m JobsModel) renderStatusBar() string {
	parts := []string{"↑/↓: navigate", "o: open", "r: refresh", "q: quit"}
	center := strings.Join(parts, " • ")
	content := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, center)
	return statusBarStyle.Width(m.width).Render(content)
}

func renderJobRow(j pipeline.Job, width int, selected bool) string {
	style := jobRowStyle
	if selected {
		style = jobRowSelectedStyle
	}

	const statusWidth = 12
	const timeWidth = 12
	const durationWidth = 16
	innerWidth := max(width-4, 20) // account for jobRowStyle's left/right padding
	remaining := max(innerWidth-statusWidth-timeWidth-durationWidth-3, 10)
	nameWidth := remaining * 2 / 3
	stageWidth := remaining - nameWidth

	name := padRight(truncateStr(j.Name, nameWidth), nameWidth)
	stage := padRight(truncateStr(j.Stage, stageWidth), stageWidth)
	status := padRight(statusIcon(j.Status), statusWidth)
	timeStr := padLeft(formatTime(j.CreatedAt), timeWidth)
	duration := padRight(truncateStr(jobDuration(j), durationWidth), durationWidth)

	row := name + " " + stage + " " + status + " " + timeStr + duration
	return style.Render(row)
}

// jobStatusCounts tallies jobs per status.
type jobStatusCounts struct {
	Passed, Failed, Running, Pending, Canceled, Skipped int
}

func (c jobStatusCounts) total() int {
	return c.Passed + c.Failed + c.Running + c.Pending + c.Canceled + c.Skipped
}

func (c *jobStatusCounts) add(status pipeline.Status) {
	switch status {
	case pipeline.StatusPassed:
		c.Passed++
	case pipeline.StatusFailed:
		c.Failed++
	case pipeline.StatusRunning:
		c.Running++
	case pipeline.StatusPending:
		c.Pending++
	case pipeline.StatusCanceled:
		c.Canceled++
	case pipeline.StatusSkipped:
		c.Skipped++
	}
}

// jobSummaryRow is one row of the aggregate totals table: a job name within
// a stage, and its status counts for today.
type jobSummaryRow struct {
	Stage  string
	Name   string
	Counts jobStatusCounts
}

// summarizeJobs groups jobs by stage and name, returning one row per group
// (sorted by stage then name) plus the overall totals across all jobs.
func summarizeJobs(jobs []pipeline.Job) ([]jobSummaryRow, jobStatusCounts) {
	index := make(map[string]int)
	var rows []jobSummaryRow
	var total jobStatusCounts

	for _, j := range jobs {
		key := j.Stage + "\x00" + j.Name
		i, ok := index[key]
		if !ok {
			i = len(rows)
			index[key] = i
			rows = append(rows, jobSummaryRow{Stage: j.Stage, Name: j.Name})
		}
		rows[i].Counts.add(j.Status)
		total.add(j.Status)
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Stage != rows[j].Stage {
			return rows[i].Stage < rows[j].Stage
		}
		return rows[i].Name < rows[j].Name
	})

	return rows, total
}

const summaryNumColWidth = 6

func renderJobSummaryTable(rows []jobSummaryRow, total jobStatusCounts, width int) string {
	const numCols = 7 // PASS, FAIL, RUN, PEND, CANC, SKIP, TOTAL
	const gaps = 8    // single-space separators between 9 columns
	available := max(width-2, 20)
	fixedWidth := summaryNumColWidth*numCols + gaps
	remaining := max(available-fixedWidth, 20)
	stageWidth := max(remaining/3, 8)
	nameWidth := max(remaining-stageWidth, 8)

	headerCells := []string{
		padRight("STAGE", stageWidth),
		padRight("JOB", nameWidth),
		padLeft("PASS", summaryNumColWidth),
		padLeft("FAIL", summaryNumColWidth),
		padLeft("RUN", summaryNumColWidth),
		padLeft("PEND", summaryNumColWidth),
		padLeft("CANC", summaryNumColWidth),
		padLeft("SKIP", summaryNumColWidth),
		padLeft("TOTAL", summaryNumColWidth),
	}
	header := tableHeaderStyle.Render(strings.Join(headerCells, " "))
	divider := dimStyle.Render(strings.Repeat("─", lipgloss.Width(header)))

	lines := []string{header, divider}
	if len(rows) == 0 {
		lines = append(lines, dimStyle.Render("No jobs today."))
	} else {
		for _, r := range rows {
			lines = append(lines, renderSummaryRow(r.Stage, r.Name, r.Counts, stageWidth, nameWidth, false))
		}
		lines = append(lines, divider)
		lines = append(lines, renderSummaryRow("", "TOTAL", total, stageWidth, nameWidth, true))
	}

	return tableStyle.Render(strings.Join(lines, "\n"))
}

func renderSummaryRow(stage, name string, c jobStatusCounts, stageWidth, nameWidth int, bold bool) string {
	cells := []string{
		padRight(truncateStr(stage, stageWidth), stageWidth),
		padRight(truncateStr(name, nameWidth), nameWidth),
		padLeft(styledCount(c.Passed, successStyle), summaryNumColWidth),
		padLeft(styledCount(c.Failed, failedStyle), summaryNumColWidth),
		padLeft(styledCount(c.Running, runningStyle), summaryNumColWidth),
		padLeft(styledCount(c.Pending, pendingStyle), summaryNumColWidth),
		padLeft(styledCount(c.Canceled, canceledStyle), summaryNumColWidth),
		padLeft(styledCount(c.Skipped, skippedStyle), summaryNumColWidth),
		padLeft(strconv.Itoa(c.total()), summaryNumColWidth),
	}
	row := strings.Join(cells, " ")
	if bold {
		return tableTotalStyle.Render(row)
	}
	return row
}

func styledCount(n int, style lipgloss.Style) string {
	s := strconv.Itoa(n)
	if n == 0 {
		return dimStyle.Render(s)
	}
	return style.Render(s)
}

func padRight(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func padLeft(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return strings.Repeat(" ", width-w) + s
}
