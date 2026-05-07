package discovery

import (
	"context"
	"fmt"
	"time"

	"github.com/navitronic/gitlab-builds/internal/gitlab"
	"github.com/navitronic/gitlab-builds/internal/glab"
)

// Discoverer finds active repositories from user activity.
type Discoverer struct {
	client *glab.Client
}

func New(client *glab.Client) *Discoverer {
	return &Discoverer{client: client}
}

// ActiveRepo represents a project the user has been active in recently.
type ActiveRepo struct {
	ProjectID   int
	ProjectPath string
	LastActive  time.Time
}

// DiscoverSince fetches user events after the given time and returns active repositories.
func (d *Discoverer) DiscoverSince(ctx context.Context, userID int, since time.Time) ([]gitlab.Event, []ActiveRepo, error) {
	events, err := d.client.FetchUserEventsSince(ctx, userID, since)
	if err != nil {
		return nil, nil, fmt.Errorf("discovering activity: %w", err)
	}

	repos := ExtractActiveRepos(events)
	return events, repos, nil
}

// ExtractActiveRepos extracts unique active repositories from events.
// Any event with a project_id marks that project as active.
func ExtractActiveRepos(events []gitlab.Event) []ActiveRepo {
	type repoState struct {
		lastActive time.Time
	}
	seen := make(map[int]*repoState)
	var order []int

	for _, e := range events {
		if e.ProjectID == 0 {
			continue
		}
		if s, ok := seen[e.ProjectID]; ok {
			if e.CreatedAt.After(s.lastActive) {
				s.lastActive = e.CreatedAt
			}
		} else {
			seen[e.ProjectID] = &repoState{lastActive: e.CreatedAt}
			order = append(order, e.ProjectID)
		}
	}

	repos := make([]ActiveRepo, 0, len(order))
	for _, id := range order {
		repos = append(repos, ActiveRepo{
			ProjectID:  id,
			LastActive: seen[id].lastActive,
		})
	}
	return repos
}
