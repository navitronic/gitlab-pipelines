package glab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/navitronic/gitlab-pipelines/internal/gitlab"
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
	return c.fetchProject(ctx, strconv.Itoa(projectID))
}

// FetchProjectByPath retrieves project details by full path, for example "group/project".
func (c *Client) FetchProjectByPath(ctx context.Context, projectPath string) (*gitlab.Project, error) {
	return c.fetchProject(ctx, url.PathEscape(projectPath))
}

func (c *Client) fetchProject(ctx context.Context, project string) (*gitlab.Project, error) {
	endpoint := "projects/" + project
	out, err := c.Run(ctx, "api", endpoint)
	if err != nil {
		return nil, fmt.Errorf("fetching project %s: %w", project, err)
	}

	var projectResponse gitlab.Project
	if err := json.Unmarshal(out, &projectResponse); err != nil {
		return nil, fmt.Errorf("parsing project response: %w", err)
	}
	return &projectResponse, nil
}
