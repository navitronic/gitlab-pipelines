package gitlab

import "time"

// Event represents a GitLab user event from the events API.
type Event struct {
	ID         int       `json:"id"`
	ActionName string    `json:"action_name"`
	TargetType string    `json:"target_type"`
	ProjectID  int       `json:"project_id"`
	CreatedAt  time.Time `json:"created_at"`
	PushData   *PushData `json:"push_data,omitempty"`
	Note       *Note     `json:"note,omitempty"`
}

// PushData contains push event details.
type PushData struct {
	CommitCount int    `json:"commit_count"`
	Ref         string `json:"ref"`
	RefType     string `json:"ref_type"`
	CommitTo    string `json:"commit_to"`
}

// Note contains note/comment event details.
type Note struct {
	NoteableType string `json:"noteable_type"`
}

// Project represents a GitLab project (minimal fields needed).
type Project struct {
	ID                int    `json:"id"`
	PathWithNamespace string `json:"path_with_namespace"`
}
