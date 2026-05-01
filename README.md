# GitLab Pipelines TUI — Implementation Plan

## Overview

A Go-based terminal UI (TUI) application that displays GitLab pipelines relevant to the current user's activity. All communication with GitLab is performed via the `glab` CLI installed on the host — no direct HTTP requests to GitLab APIs.

---

## Tech Stack

- Go (1.22+)
- Charm libraries: bubbletea, bubbles, lipgloss
- External dependency: glab CLI (required at runtime)

---

## Architecture

```
TUI (Bubble Tea)
  ↓
Application State
  ↓
Discovery Layer
  ↓
Glab Client (shells out)
  ↓
glab api → GitLab
```

---

## Project Structure

```
cmd/gitlab-pipelines/main.go

internal/glab/       # glab CLI wrapper
internal/discovery/  # Pipeline candidate discovery
internal/gitlab/     # GitLab data models
internal/tui/        # Bubble Tea UI components
internal/config/     # Configuration
```

---

## Implementation Checklist

### Phase 1: Foundation

- [x] Project scaffolding (go.mod, directory structure, main.go)
- [x] glab client wrapper (`internal/glab/`)
  - [x] `runGlab(ctx, args...)` exec helper
  - [x] JSON response parsing
  - [x] Error handling (missing binary, auth failure)
- [x] Fetch current user via `glab api user`

### Phase 2: Discovery

- [x] Fetch user events via `glab api "users/:id/events?per_page=100"`
- [x] Extract pipeline candidates (project ID, ref, SHA, event type, timestamp)
- [x] Deduplicate candidates by `project_id + ref + sha`
- [x] Pagination support for events endpoint

### Phase 3: Pipeline Retrieval

- [x] Fetch pipelines by SHA: `glab api "projects/:id/pipelines?sha=$SHA&user_id=$USER_ID"`
- [x] Fetch pipelines by ref: `glab api "projects/:id/pipelines?ref=$REF&user_id=$USER_ID"`
- [x] Fallback fetch: `glab api "projects/:id/pipelines?user_id=$USER_ID&order_by=updated_at&sort=desc"`
- [x] Fetch pipeline jobs: `glab api "projects/:id/pipelines/:pipeline_id/jobs"`
- [x] Pagination support for pipeline endpoints

### Phase 4: TUI — Pipeline List

- [ ] Bubble Tea model setup
- [ ] Pipeline list table view
  - [ ] Columns: Status | Project | Ref | Commit | Pipeline | Jobs | Updated | Source
- [ ] Keyboard navigation (up/down, enter to select)
- [ ] Status indicators with lipgloss styling
- [ ] Loading state while fetching data

### Phase 5: TUI — Pipeline Details

- [ ] Pipeline detail view (metadata + job list)
- [ ] Job status display
- [ ] Navigation back to pipeline list
- [ ] Keyboard shortcut help

### Phase 6: Polish

- [ ] Graceful handling of missing/unauthenticated glab
- [ ] Error display in TUI (not panic)
- [ ] Auto-refresh / manual refresh
- [ ] Responsive layout for different terminal sizes

---

## Data Flow

1. Fetch current user → `glab api user`
2. Fetch user events → `glab api "users/$USER_ID/events?per_page=100"`
3. Build pipeline candidates (extract + deduplicate)
4. Fetch pipelines per candidate (SHA → ref → fallback)
5. Render pipeline list in TUI
6. On selection: fetch jobs → render detail view

---

## Constraints

- MUST use `glab` for all GitLab interaction
- MUST NOT use `net/http` to call GitLab directly
- MUST NOT require GitLab tokens directly (assume `glab auth login` done)
- MUST support pagination via repeated `glab api` calls

---

## glab Execution Pattern

```go
func runGlab(ctx context.Context, args ...string) ([]byte, error) {
    cmd := exec.CommandContext(ctx, "glab", args...)
    return cmd.Output()
}
```

---

## Key Models

```go
type PipelineCandidate struct {
    ProjectID   string
    ProjectPath string
    Ref         string
    SHA         string
    Reason      string
    EventTime   time.Time
}
```
