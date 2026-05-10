package gitlab

import "time"

// User represents a GitLab user.
type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Email    string `json:"email"`
}

// Pipeline represents a GitLab CI pipeline.
type Pipeline struct {
	ID             int       `json:"id"`
	IID            int       `json:"iid"`
	ProjectID      int       `json:"project_id"`
	SHA            string    `json:"sha"`
	Ref            string    `json:"ref"`
	Status         string    `json:"status"`
	Source         string    `json:"source"`
	WebURL         string    `json:"web_url"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Duration       float64   `json:"duration"`
	QueuedDuration float64   `json:"queued_duration"`
}

// Job represents a job within a pipeline.
type Job struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	Stage        string    `json:"stage"`
	Status       string    `json:"status"`
	AllowFailure bool      `json:"allow_failure"`
	WebURL       string    `json:"web_url"`
	CreatedAt    time.Time `json:"created_at"`
	StartedAt    time.Time `json:"started_at"`
	Duration     float64   `json:"duration"`
}

// MergeRequest represents a GitLab merge request.
type MergeRequest struct {
	IID       int       `json:"iid"`
	ProjectID int       `json:"project_id"`
	WebURL    string    `json:"web_url"`
	State     string    `json:"state"`
	Title     string    `json:"title"`
	UpdatedAt time.Time `json:"updated_at"`
}
