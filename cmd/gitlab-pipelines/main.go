package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/navitronic/gitlab-pipelines/internal/activity"
	"github.com/navitronic/gitlab-pipelines/internal/cache"
	"github.com/navitronic/gitlab-pipelines/internal/demo"
	"github.com/navitronic/gitlab-pipelines/internal/glab"
	"github.com/navitronic/gitlab-pipelines/internal/pipeline"
	"github.com/navitronic/gitlab-pipelines/internal/pipeline/gitlabsvc"
	"github.com/navitronic/gitlab-pipelines/internal/tui"
)

func main() {
	demoMode := flag.Bool("demo", false, "run with demo fixture data (no network, no polling)")
	repo := flag.String("repo", "", "show pipelines for a specific GitLab project path or ID")
	jobsRepo := flag.String("jobs", "", "show today's jobs for a specific GitLab project path or ID")
	limit := flag.Int("limit", 100, "maximum pipelines/jobs to fetch when using -repo or -jobs")
	flag.Parse()

	if *repo != "" && *jobsRepo != "" {
		fmt.Fprintln(os.Stderr, "Error: -repo and -jobs cannot be used together")
		os.Exit(1)
	}

	if *jobsRepo != "" {
		runJobs(*jobsRepo, *limit)
		return
	}

	m := tui.New()

	if *demoMode {
		runDemo(m)
	} else {
		runLive(m, *repo, *limit)
	}
}

func runDemo(m tui.Model) {
	m.FetchJobs = func(_, pipelineID string) tea.Cmd {
		return func() tea.Msg {
			return tui.JobsLoadedMsg{PipelineID: pipelineID, Jobs: demo.Jobs(pipelineID)}
		}
	}
	m.FetchPipeline = func(_, pipelineID string) tea.Cmd {
		return func() tea.Msg {
			for _, p := range demo.Pipelines() {
				if p.ID == pipelineID {
					return tui.PipelineUpdatedMsg{Pipeline: p}
				}
			}
			return tui.PipelineUpdatedMsg{}
		}
	}
	m.FetchMR = func(_, pipelineID, _ string) tea.Cmd {
		return func() tea.Msg {
			return tui.MRLoadedMsg{PipelineID: pipelineID, URL: demo.MRURLs[pipelineID]}
		}
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	go func() {
		p.Send(tui.PipelinesLoadedMsg{Pipelines: toRows(demo.Pipelines())})
	}()
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runLive(m tui.Model, repo string, limit int) {
	ctx := context.Background()
	client := glab.New()
	svc := gitlabsvc.New(client)

	m.FetchJobs = func(projectID, pipelineID string) tea.Cmd {
		return func() tea.Msg {
			jobs, err := svc.ListJobs(ctx, projectID, pipelineID)
			return tui.JobsLoadedMsg{PipelineID: pipelineID, Jobs: jobs, Err: err}
		}
	}
	m.FetchPipeline = func(projectID, pipelineID string) tea.Cmd {
		return func() tea.Msg {
			p, err := svc.GetPipeline(ctx, projectID, pipelineID)
			return tui.PipelineUpdatedMsg{Pipeline: p, Err: err}
		}
	}
	m.FetchMR = func(projectID, pipelineID, ref string) tea.Cmd {
		return func() tea.Msg {
			url, err := svc.GetMergeRequestURL(ctx, projectID, ref)
			return tui.MRLoadedMsg{PipelineID: pipelineID, URL: url, Err: err}
		}
	}

	var prog *tea.Program
	sendStatus := func(status string) {
		prog.Send(tui.LoadingStatusMsg{Status: status})
	}

	fetchPipelines := func() ([]pipeline.Pipeline, error) {
		if repo != "" {
			return svc.ListProjectPipelines(ctx, repo, limit, sendStatus)
		}
		return svc.ListPipelines(ctx, sendStatus)
	}

	m.Refresh = func() tea.Cmd {
		return func() tea.Msg {
			pipelines, err := fetchPipelines()
			if repo == "" {
				if err != nil && isFatalErr(err) {
					cache.Clear()
				} else if len(pipelines) > 0 {
					cache.Save(pipelines)
				}
			}
			return tui.PipelinesLoadedMsg{Pipelines: toRows(pipelines), Err: err}
		}
	}
	m.HardRefresh = func() tea.Cmd {
		return func() tea.Msg {
			if repo == "" {
				activity.Clear()
				cache.Clear()
			}
			pipelines, err := fetchPipelines()
			if repo == "" && len(pipelines) > 0 {
				cache.Save(pipelines)
			}
			return tui.PipelinesLoadedMsg{Pipelines: toRows(pipelines), Err: err}
		}
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	prog = p

	go func() {
		if repo == "" {
			if cached, err := cache.Load(); err == nil && len(cached) > 0 {
				p.Send(tui.PipelinesLoadedMsg{Pipelines: toRows(cached)})
			}
		}
		pipelines, err := fetchPipelines()
		if repo == "" {
			if err != nil && isFatalErr(err) {
				cache.Clear()
			} else if len(pipelines) > 0 {
				cache.Save(pipelines)
			}
		}
		p.Send(tui.PipelinesLoadedMsg{Pipelines: toRows(pipelines), Err: err})
	}()

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runJobs(repo string, limit int) {
	ctx := context.Background()
	client := glab.New()
	svc := gitlabsvc.New(client)

	m := tui.NewJobsModel(repo)

	var prog *tea.Program
	sendStatus := func(status string) {
		prog.Send(tui.LoadingStatusMsg{Status: status})
	}

	m.Refresh = func() tea.Cmd {
		return func() tea.Msg {
			jobs, err := svc.ListProjectJobsToday(ctx, repo, limit, sendStatus)
			return tui.RepoJobsLoadedMsg{Jobs: jobs, Err: err}
		}
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	prog = p

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func toRows(pipelines []pipeline.Pipeline) []tui.PipelineRow {
	rows := make([]tui.PipelineRow, len(pipelines))
	for i, p := range pipelines {
		rows[i] = tui.PipelineRow{Pipeline: p}
	}
	return rows
}

func isFatalErr(err error) bool {
	return errors.Is(err, pipeline.ErrAuthRequired) || errors.Is(err, pipeline.ErrClientNotFound)
}
