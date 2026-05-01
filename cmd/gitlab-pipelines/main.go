package main

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/navitronic/gitlab-builds/internal/discovery"
	"github.com/navitronic/gitlab-builds/internal/glab"
	"github.com/navitronic/gitlab-builds/internal/tui"
)

func main() {
	ctx := context.Background()
	client := glab.New()

	m := tui.New()
	m.FetchJobs = func(projectID, pipelineID int) tea.Cmd {
		return func() tea.Msg {
			jobs, err := client.FetchPipelineJobs(ctx, projectID, pipelineID)
			return tui.JobsLoadedMsg{Jobs: jobs, Err: err}
		}
	}
	m.Refresh = func() tea.Cmd {
		return func() tea.Msg {
			rows, err := loadPipelines(ctx, client)
			return tui.PipelinesLoadedMsg{Pipelines: rows, Err: err}
		}
	}
	p := tea.NewProgram(m, tea.WithAltScreen())

	go func() {
		rows, err := loadPipelines(ctx, client)
		p.Send(tui.PipelinesLoadedMsg{Pipelines: rows, Err: err})
	}()

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func loadPipelines(ctx context.Context, client *glab.Client) ([]tui.PipelineRow, error) {
	user, err := client.CurrentUser(ctx)
	if err != nil {
		return nil, err
	}

	disc := discovery.New(client)
	candidates, err := disc.Discover(ctx, user.ID, 3)
	if err != nil {
		return nil, err
	}

	var rows []tui.PipelineRow
	var lastErr error
	for _, c := range candidates {
		pipelines, err := client.FetchPipelinesBySHA(ctx, c.ProjectID, user.ID, c.SHA)
		if err != nil {
			lastErr = err
			continue
		}
		if len(pipelines) == 0 {
			pipelines, err = client.FetchPipelinesByRef(ctx, c.ProjectID, user.ID, c.Ref)
			if err != nil {
				lastErr = err
				continue
			}
		}
		if len(pipelines) == 0 {
			pipelines, err = client.FetchPipelinesFallback(ctx, c.ProjectID, user.ID)
			if err != nil {
				lastErr = err
				continue
			}
		}
		for _, pipeline := range pipelines {
			jobSummary := ""
			jobs, jobErr := client.FetchPipelineJobs(ctx, c.ProjectID, pipeline.ID)
			if jobErr == nil && len(jobs) > 0 {
				passed, failed, total := 0, 0, len(jobs)
				for _, j := range jobs {
					switch j.Status {
					case "success":
						passed++
					case "failed":
						failed++
					}
				}
				if failed > 0 {
					jobSummary = fmt.Sprintf("%d/%d fail", failed, total)
				} else {
					jobSummary = fmt.Sprintf("%d/%d pass", passed, total)
				}
			}
			rows = append(rows, tui.PipelineRow{
				Pipeline:    pipeline,
				ProjectPath: c.ProjectPath,
				JobSummary:  jobSummary,
			})
		}
	}
	if len(rows) == 0 && lastErr != nil {
		return nil, fmt.Errorf("fetching pipelines: %w", lastErr)
	}
	return rows, nil
}
