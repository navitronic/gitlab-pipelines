package main

import (
	"context"
	"fmt"
	"os"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/navitronic/gitlab-builds/internal/discovery"
	"github.com/navitronic/gitlab-builds/internal/gitlab"
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
	candidates, err := disc.Discover(ctx, user.ID, 3, 10)
	if err != nil {
		return nil, err
	}

	type result struct {
		rows []tui.PipelineRow
		err  error
	}
	ch := make(chan result, len(candidates))

	for _, c := range candidates {
		go func(c gitlab.PipelineCandidate) {
			pipelines, err := client.FetchPipelinesBySHA(ctx, c.ProjectID, user.ID, c.SHA)
			if err != nil {
				ch <- result{err: err}
				return
			}
			if len(pipelines) == 0 {
				pipelines, err = client.FetchPipelinesByRef(ctx, c.ProjectID, user.ID, c.Ref)
				if err != nil {
					ch <- result{err: err}
					return
				}
			}
			if len(pipelines) == 0 {
				pipelines, err = client.FetchPipelinesFallback(ctx, c.ProjectID, user.ID)
				if err != nil {
					ch <- result{err: err}
					return
				}
			}
			var rows []tui.PipelineRow
			for _, pipeline := range pipelines {
				rows = append(rows, tui.PipelineRow{
					Pipeline:    pipeline,
					ProjectPath: c.ProjectPath,
				})
			}
			ch <- result{rows: rows}
		}(c)
	}

	var rows []tui.PipelineRow
	var lastErr error
	for range candidates {
		r := <-ch
		if r.err != nil {
			lastErr = r.err
			continue
		}
		rows = append(rows, r.rows...)
	}
	if len(rows) == 0 && lastErr != nil {
		return nil, fmt.Errorf("fetching pipelines: %w", lastErr)
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Pipeline.UpdatedAt.After(rows[j].Pipeline.UpdatedAt)
	})
	return rows, nil
}
