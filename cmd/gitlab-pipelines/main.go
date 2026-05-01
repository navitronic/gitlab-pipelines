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
	for _, c := range candidates {
		pipelines, err := client.FetchPipelinesBySHA(ctx, c.ProjectID, user.ID, c.SHA)
		if err != nil {
			continue
		}
		if len(pipelines) == 0 {
			pipelines, err = client.FetchPipelinesByRef(ctx, c.ProjectID, user.ID, c.Ref)
			if err != nil {
				continue
			}
		}
		for _, pipeline := range pipelines {
			rows = append(rows, tui.PipelineRow{
				Pipeline:    pipeline,
				ProjectPath: c.ProjectPath,
			})
		}
	}
	return rows, nil
}
