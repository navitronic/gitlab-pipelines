package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/navitronic/gitlab-builds/internal/gitlab"
)

// JobsLoadedMsg signals that jobs for a pipeline have been fetched.
type JobsLoadedMsg struct {
	Jobs []gitlab.Job
	Err  error
}

// DetailModel renders the detail view for a selected pipeline.
type DetailModel struct {
	row         PipelineRow
	jobs        []gitlab.Job
	jobsLoading bool
	jobsErr     error
}

// NewDetailModel creates a detail model for the given pipeline row.
func NewDetailModel(row PipelineRow) *DetailModel {
	return &DetailModel{row: row, jobsLoading: true}
}

func (d *DetailModel) SetJobs(jobs []gitlab.Job, err error) {
	d.jobsLoading = false
	d.jobs = jobs
	d.jobsErr = err
}

func (d *DetailModel) View() string {
	p := d.row.Pipeline

	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf("Pipeline #%d", p.ID)))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("  Project:  %s\n", d.row.ProjectPath))
	b.WriteString(fmt.Sprintf("  Status:   %s\n", statusIcon(p.Status)))
	b.WriteString(fmt.Sprintf("  Ref:      %s\n", p.Ref))
	b.WriteString(fmt.Sprintf("  SHA:      %s\n", p.SHA))
	b.WriteString(fmt.Sprintf("  Source:   %s\n", p.Source))
	b.WriteString(fmt.Sprintf("  Updated:  %s\n", formatTime(p.UpdatedAt)))
	if p.WebURL != "" {
		b.WriteString(fmt.Sprintf("  URL:      %s\n", p.WebURL))
	}
	b.WriteString("\n")

	b.WriteString(jobHeaderStyle.Render("Jobs"))
	b.WriteString("\n\n")

	if d.jobsErr != nil {
		b.WriteString(fmt.Sprintf("  Error loading jobs: %v\n", d.jobsErr))
	} else if d.jobsLoading {
		b.WriteString("  Loading jobs...\n")
	} else if len(d.jobs) == 0 {
		b.WriteString("  No jobs found.\n")
	} else {
		b.WriteString(d.renderJobs())
	}

	b.WriteString("\n")
	b.WriteString(detailHelpStyle.Render("esc/backspace: back • q: quit"))
	return b.String()
}

func (d *DetailModel) hasActiveJobs() bool {
	for _, job := range d.jobs {
		if job.Status == "running" || job.Status == "pending" {
			return true
		}
	}
	return false
}

func jobDuration(job gitlab.Job) string {
	switch job.Status {
	case "running":
		if !job.StartedAt.IsZero() {
			return fmt.Sprintf(" (%s)", formatDuration(time.Since(job.StartedAt).Seconds()))
		}
	case "pending":
		if !job.CreatedAt.IsZero() {
			return fmt.Sprintf(" (waiting %s)", formatDuration(time.Since(job.CreatedAt).Seconds()))
		}
	default:
		if job.Duration > 0 {
			return fmt.Sprintf(" (%s)", formatDuration(job.Duration))
		}
	}
	return ""
}

func (d *DetailModel) renderJobs() string {
	var b strings.Builder

	stageJobs := make(map[string][]gitlab.Job)
	var stageOrder []string
	seen := make(map[string]bool)
	for _, job := range d.jobs {
		if !seen[job.Stage] {
			seen[job.Stage] = true
			stageOrder = append(stageOrder, job.Stage)
		}
		stageJobs[job.Stage] = append(stageJobs[job.Stage], job)
	}

	for _, stage := range stageOrder {
		b.WriteString(fmt.Sprintf("  %s\n", stageStyle.Render(stage)))
		for _, job := range stageJobs[stage] {
			b.WriteString(fmt.Sprintf("    %s %s%s\n", jobStatusIcon(job.Status), job.Name, jobDuration(job)))
		}
	}
	return b.String()
}

func jobStatusIcon(status string) string {
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
	case "manual":
		return pendingStyle.Render("▶")
	default:
		return "?"
	}
}

func formatDuration(seconds float64) string {
	if seconds < 60 {
		return fmt.Sprintf("%.0fs", seconds)
	}
	m := int(seconds) / 60
	s := int(seconds) % 60
	return fmt.Sprintf("%dm%ds", m, s)
}

var (
	detailHelpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).MarginLeft(2).MarginTop(1)
	jobHeaderStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).MarginLeft(2)
	stageStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("69"))
)
