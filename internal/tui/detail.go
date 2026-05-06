package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/navitronic/gitlab-builds/internal/pipeline"
)

type MRLoadedMsg struct {
	URL string
	Err error
}

type JobsLoadedMsg struct {
	Jobs []pipeline.Job
	Err  error
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type DetailModel struct {
	row         PipelineRow
	jobs        []pipeline.Job
	jobsLoading bool
	jobsErr     error
	mrURL       string
	frame       int
}

func NewDetailModel(row PipelineRow) *DetailModel {
	return &DetailModel{row: row, jobsLoading: true}
}

func (d *DetailModel) SetJobs(jobs []pipeline.Job, err error) {
	d.jobsLoading = false
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].ID < jobs[j].ID })
	d.jobs = jobs
	d.jobsErr = err
}

func (d *DetailModel) SetMRURL(url string) {
	d.mrURL = url
}

func (d *DetailModel) Tick() {
	d.frame = (d.frame + 1) % len(spinnerFrames)
}

func (d *DetailModel) Render(width, height int) string {
	p := d.row.Pipeline

	var b strings.Builder
	b.WriteString(detailTitleStyle.Render(fmt.Sprintf("Pipeline #%s", p.ID)))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("  Project:  %s\n", p.Project))
	b.WriteString(fmt.Sprintf("  Status:   %s\n", statusIcon(p.Status)))
	b.WriteString(fmt.Sprintf("  Ref:      %s\n", p.Ref))
	b.WriteString(fmt.Sprintf("  SHA:      %s\n", p.SHA))
	if d.mrURL != "" {
		b.WriteString(fmt.Sprintf("  MR:       %s\n", d.mrURL))
	}
	b.WriteString(fmt.Sprintf("  Source:   %s\n", p.Source))
	b.WriteString(fmt.Sprintf("  Updated:  %s\n", formatTime(p.UpdatedAt)))
	b.WriteString(fmt.Sprintf("  Duration: %s\n", pipelineDuration(p)))
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

	return b.String()
}

func pipelineDuration(p pipeline.Pipeline) string {
	switch p.Status {
	case pipeline.StatusRunning, pipeline.StatusPending:
		if !p.CreatedAt.IsZero() {
			return formatDuration(time.Since(p.CreatedAt))
		}
	default:
		if p.Duration > 0 {
			return formatDuration(p.Duration)
		}
	}
	return "-"
}

func (d *DetailModel) hasActiveJobs() bool {
	for _, job := range d.jobs {
		if job.Status == pipeline.StatusRunning || job.Status == pipeline.StatusPending {
			return true
		}
	}
	return false
}

func jobDuration(job pipeline.Job) string {
	switch job.Status {
	case pipeline.StatusRunning:
		if !job.StartedAt.IsZero() {
			return fmt.Sprintf(" (%s)", formatDuration(time.Since(job.StartedAt)))
		}
	case pipeline.StatusPending:
		if !job.CreatedAt.IsZero() {
			return fmt.Sprintf(" (waiting %s)", formatDuration(time.Since(job.CreatedAt)))
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

	stageJobs := make(map[string][]pipeline.Job)
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
			icon := jobStatusIcon(job.Status)
			name := job.Name
			dur := jobDuration(job)
			if job.Status == pipeline.StatusRunning {
				icon = runningStyle.Render(spinnerFrames[d.frame%len(spinnerFrames)])
				name = runningBoldStyle.Render(name)
			}
			b.WriteString(fmt.Sprintf("    %s %s%s\n", icon, name, dur))
		}
	}
	return b.String()
}

func jobStatusIcon(status pipeline.Status) string {
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

func formatDuration(d time.Duration) string {
	seconds := int(d.Seconds())
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	m := seconds / 60
	s := seconds % 60
	return fmt.Sprintf("%dm%ds", m, s)
}
