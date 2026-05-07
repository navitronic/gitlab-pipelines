package glab

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/navitronic/gitlab-builds/internal/gitlab"
)

// FetchUserEventsSince retrieves user events after the given date.
// The GitLab API accepts date-level granularity (YYYY-MM-DD) for the "after" parameter.
func (c *Client) FetchUserEventsSince(ctx context.Context, userID int, after time.Time) ([]gitlab.Event, error) {
	afterDate := after.Format("2006-01-02")

	var allEvents []gitlab.Event
	for page := 1; ; page++ {
		endpoint := fmt.Sprintf("users/%d/events?per_page=100&after=%s&page=%d", userID, afterDate, page)
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
		if len(events) < 100 {
			break
		}
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
