package glab

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/navitronic/gitlab-builds/internal/gitlab"
)

// FetchUserEvents retrieves user events with pagination.
// Returns all events across pages up to maxPages (0 = single page).
func (c *Client) FetchUserEvents(ctx context.Context, userID int, maxPages int) ([]gitlab.Event, error) {
	if maxPages <= 0 {
		maxPages = 1
	}

	var allEvents []gitlab.Event
	for page := 1; page <= maxPages; page++ {
		endpoint := fmt.Sprintf("users/%d/events?per_page=100&page=%d", userID, page)
		out, err := c.Run(ctx, "api", endpoint)
		if err != nil {
			return nil, fmt.Errorf("fetching user events (page %d): %w", page, err)
		}

		var events []gitlab.Event
		if err := json.Unmarshal(out, &events); err != nil {
			return nil, fmt.Errorf("parsing events response (page %d): %w", page, err)
		}

		if len(events) == 0 {
			break
		}
		allEvents = append(allEvents, events...)
	}
	return allEvents, nil
}

// FetchProject retrieves project details by ID.
func (c *Client) FetchProject(ctx context.Context, projectID int) (*gitlab.Project, error) {
	endpoint := "projects/" + strconv.Itoa(projectID)
	out, err := c.Run(ctx, "api", endpoint)
	if err != nil {
		return nil, fmt.Errorf("fetching project %d: %w", projectID, err)
	}

	var project gitlab.Project
	if err := json.Unmarshal(out, &project); err != nil {
		return nil, fmt.Errorf("parsing project response: %w", err)
	}
	return &project, nil
}
