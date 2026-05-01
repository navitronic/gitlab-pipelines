package discovery

import (
	"context"
	"fmt"

	"github.com/navitronic/gitlab-builds/internal/gitlab"
	"github.com/navitronic/gitlab-builds/internal/glab"
)

// Discoverer finds pipeline candidates from user activity.
type Discoverer struct {
	client *glab.Client
}

// New creates a Discoverer with the given glab client.
func New(client *glab.Client) *Discoverer {
	return &Discoverer{client: client}
}

// Discover fetches user events and extracts deduplicated pipeline candidates.
// Results are capped at maxCandidates (0 = no limit).
func (d *Discoverer) Discover(ctx context.Context, userID int, maxPages int, maxCandidates int) ([]gitlab.PipelineCandidate, error) {
	events, err := d.client.FetchUserEvents(ctx, userID, maxPages)
	if err != nil {
		return nil, fmt.Errorf("discovering pipelines: %w", err)
	}

	candidates := ExtractCandidates(events)
	deduped := Deduplicate(candidates)

	if maxCandidates > 0 && len(deduped) > maxCandidates {
		deduped = deduped[:maxCandidates]
	}

	projectPaths := make(map[int]string)
	for i, c := range deduped {
		if _, ok := projectPaths[c.ProjectID]; !ok {
			project, err := d.client.FetchProject(ctx, c.ProjectID)
			if err == nil {
				projectPaths[c.ProjectID] = project.PathWithNamespace
			}
		}
		deduped[i].ProjectPath = projectPaths[c.ProjectID]
	}

	return deduped, nil
}

// ExtractCandidates converts events into pipeline candidates.
func ExtractCandidates(events []gitlab.Event) []gitlab.PipelineCandidate {
	var candidates []gitlab.PipelineCandidate

	for _, event := range events {
		if event.PushData == nil {
			continue
		}
		if event.PushData.CommitTo == "" {
			continue
		}

		candidate := gitlab.PipelineCandidate{
			ProjectID: event.ProjectID,
			Ref:       event.PushData.Ref,
			SHA:       event.PushData.CommitTo,
			Reason:    event.ActionName,
			EventTime: event.CreatedAt,
		}
		candidates = append(candidates, candidate)
	}

	return candidates
}

// Deduplicate removes duplicate candidates by project_id + ref + sha.
func Deduplicate(candidates []gitlab.PipelineCandidate) []gitlab.PipelineCandidate {
	type key struct {
		projectID int
		ref       string
		sha       string
	}

	seen := make(map[key]bool)
	var result []gitlab.PipelineCandidate

	for _, c := range candidates {
		k := key{projectID: c.ProjectID, ref: c.Ref, sha: c.SHA}
		if seen[k] {
			continue
		}
		seen[k] = true
		result = append(result, c)
	}

	return result
}
