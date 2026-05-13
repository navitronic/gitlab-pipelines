package glab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/navitronic/gitlab-pipelines/internal/gitlab"
)

// ErrGlabNotFound indicates the glab binary is not installed or not in PATH.
var ErrGlabNotFound = errors.New("glab binary not found in PATH")

// ErrAuthRequired indicates glab is not authenticated.
var ErrAuthRequired = errors.New("glab authentication required (run: glab auth login)")

// Client wraps the glab CLI for GitLab API interactions.
type Client struct {
	// BinaryPath overrides the default "glab" binary location.
	BinaryPath string
}

// New creates a new glab Client with default settings.
func New() *Client {
	return &Client{BinaryPath: "glab"}
}

// Run executes a glab command and returns the raw output.
func (c *Client) Run(ctx context.Context, args ...string) ([]byte, error) {
	bin := c.BinaryPath
	if bin == "" {
		bin = "glab"
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	out, err := cmd.Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, ErrGlabNotFound
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderr := string(exitErr.Stderr)
			if strings.Contains(stderr, "auth") || strings.Contains(stderr, "401") {
				return nil, ErrAuthRequired
			}
			return nil, fmt.Errorf("glab %s: %s", strings.Join(args, " "), stderr)
		}
		return nil, fmt.Errorf("glab %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

// CurrentUser fetches the authenticated user via `glab api user`.
func (c *Client) CurrentUser(ctx context.Context) (*gitlab.User, error) {
	out, err := c.Run(ctx, "api", "user")
	if err != nil {
		return nil, fmt.Errorf("fetching current user: %w", err)
	}

	var user gitlab.User
	if err := json.Unmarshal(out, &user); err != nil {
		return nil, fmt.Errorf("parsing user response: %w", err)
	}
	return &user, nil
}
