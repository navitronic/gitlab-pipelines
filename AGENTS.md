# AGENTS.md

## Project

GitLab Pipelines TUI — a Go terminal application using Bubble Tea that displays GitLab pipelines via `glab` CLI.

## Language & Build

- Go 1.22+
- Build: `go build ./cmd/gitlab-pipelines/`
- Run: `go run ./cmd/gitlab-pipelines/`
- Test: `go test ./...`
- Lint: `golangci-lint run` (if available)

## Dependencies

- `charmbracelet/bubbletea` — TUI framework
- `charmbracelet/bubbles` — TUI components (table, spinner, etc.)
- `charmbracelet/lipgloss` — Terminal styling
- `glab` CLI — runtime dependency, must be installed and authenticated

## Architecture

```
cmd/gitlab-pipelines/main.go    # Entry point
internal/glab/                   # glab CLI wrapper (exec + JSON parsing)
internal/discovery/              # Pipeline candidate extraction from user events
internal/gitlab/                 # Data models (Pipeline, Job, User, Event)
internal/tui/                    # Bubble Tea models and views
internal/config/                 # Configuration loading
```

## Key Constraints

- **No net/http to GitLab.** All GitLab data comes via `glab api <endpoint>`.
- **No direct token usage.** Assume `glab auth login` is pre-configured.
- **Pagination is manual.** Use `per_page` and `page` query params with repeated calls.

## Coding Style

- Standard Go conventions (`gofmt`, `goimports`)
- Error wrapping with `fmt.Errorf("context: %w", err)`
- Context propagation for cancellation
- Table-driven tests
- Interfaces for testability (mock glab execution in tests)

## File Naming

- One type per file when the type is substantial
- Test files: `*_test.go` adjacent to source
- Package names match directory names

## glab Wrapper Pattern

```go
// internal/glab/client.go
type Client struct {
    // optional: custom binary path, timeout
}

func (c *Client) Run(ctx context.Context, args ...string) ([]byte, error) {
    cmd := exec.CommandContext(ctx, "glab", args...)
    out, err := cmd.Output()
    if err != nil {
        var exitErr *exec.ExitError
        if errors.As(err, &exitErr) {
            return nil, fmt.Errorf("glab %s: %s", strings.Join(args, " "), exitErr.Stderr)
        }
        return nil, fmt.Errorf("glab %s: %w", strings.Join(args, " "), err)
    }
    return out, nil
}
```

## TUI Patterns

- Each view is its own Bubble Tea `Model` with `Init`, `Update`, `View`
- Parent model routes between views (pipeline list ↔ pipeline detail)
- Use `tea.Cmd` for async data fetching (never block in Update)
- Loading states via `bubbles/spinner`

## Testing Strategy

- Unit test the glab wrapper with fake exec (or interface mock)
- Unit test discovery logic with fixture JSON
- Integration tests optional (require live glab auth)

## Common Tasks

### Add a new glab endpoint

1. Add a method to `internal/glab/client.go`
2. Define response struct in `internal/gitlab/models.go`
3. Parse JSON in the client method
4. Wire into discovery or TUI as needed

### Add a new TUI view

1. Create model in `internal/tui/`
2. Implement `Init`, `Update`, `View`
3. Add navigation case in the parent model's `Update`

## Plan Reference

See `README.md` for the full implementation checklist with progress tracking.
