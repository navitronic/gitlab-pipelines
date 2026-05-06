package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/navitronic/gitlab-builds/internal/cache"
	"github.com/navitronic/gitlab-builds/internal/glab"
	"github.com/navitronic/gitlab-builds/internal/pipeline"
	"github.com/navitronic/gitlab-builds/internal/pipeline/gitlabsvc"
	"github.com/navitronic/gitlab-builds/internal/tui"
)

func main() {
	ctx := context.Background()
	client := glab.New()
	svc := gitlabsvc.New(client)

	m := tui.New()
	m.FetchJobs = func(projectID, pipelineID string) tea.Cmd {
		return func() tea.Msg {
			jobs, err := svc.ListJobs(ctx, projectID, pipelineID)
			return tui.JobsLoadedMsg{Jobs: jobs, Err: err}
		}
	}
	m.FetchPipeline = func(projectID, pipelineID string) tea.Cmd {
		return func() tea.Msg {
			p, err := svc.GetPipeline(ctx, projectID, pipelineID)
			return tui.PipelineUpdatedMsg{Pipeline: p, Err: err}
		}
	}
	m.FetchMR = func(projectID, ref string) tea.Cmd {
		return func() tea.Msg {
			url, err := svc.GetMergeRequestURL(ctx, projectID, ref)
			return tui.MRLoadedMsg{URL: url, Err: err}
		}
	}

	var prog *tea.Program
	sendStatus := func(status string) {
		prog.Send(tui.LoadingStatusMsg{Status: status})
	}

	m.Refresh = func() tea.Cmd {
		return func() tea.Msg {
			pipelines, err := svc.ListPipelines(ctx, sendStatus)
			if err != nil && isFatalErr(err) {
				cache.Clear()
			} else if len(pipelines) > 0 {
				cache.Save(pipelines)
			}
			return tui.PipelinesLoadedMsg{Pipelines: toRows(pipelines), Err: err}
		}
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	prog = p

	go func() {
		if cached, err := cache.Load(); err == nil && len(cached) > 0 {
			p.Send(tui.PipelinesLoadedMsg{Pipelines: toRows(cached)})
		}
		pipelines, err := svc.ListPipelines(ctx, sendStatus)
		if err != nil && isFatalErr(err) {
			cache.Clear()
		} else if len(pipelines) > 0 {
			cache.Save(pipelines)
		}
		p.Send(tui.PipelinesLoadedMsg{Pipelines: toRows(pipelines), Err: err})
	}()

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
