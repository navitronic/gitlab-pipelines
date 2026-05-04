# Plan: Separate Data and UI Layers

## Goals

1. Make it easier to reason about UI code and make updates to it.
2. Support multiple CI/CD platforms (GitLab, GitHub Actions, etc.).
3. Add caching to improve perceived performance.

## Current State

The codebase has these coupling points between data and UI:

| Issue | Where | Impact |
|-------|-------|--------|
| TUI imports `internal/gitlab` models directly | `internal/tui/model.go`, `detail.go` | UI is bound to GitLab's data shape |
| TUI imports `internal/glab` for error sentinels | `internal/tui/model.go` (formatError) | UI knows about the transport layer |
| `PipelineRow` uses `gitlab.Pipeline` | `internal/tui/model.go` | Can't swap backend without changing UI |
| `loadPipelines` in `main.go` builds `tui.PipelineRow` | `cmd/gitlab-pipelines/main.go` | Orchestration logic mixed with data assembly |
| `FetchJobs`/`FetchPipeline`/`Refresh` are closures on the model | `internal/tui/model.go` | UI owns the contract for data fetching |
| Discovery logic is GitLab-specific (push events → candidates) | `internal/discovery/` | No equivalent abstraction for other platforms |

## Proposed Architecture

```
cmd/
  pipelines/main.go              # Entry point, wires provider + cache + TUI

internal/
  provider/                      # Platform-agnostic data interface
    provider.go                  # Interface definitions
    types.go                     # Platform-neutral models (Pipeline, Job, etc.)
  
  provider/gitlab/               # GitLab implementation
    gitlab.go                    # Implements provider.Provider using glab CLI
  
  provider/github/               # GitHub implementation (future)
    github.go                    # Implements provider.Provider using gh CLI
  
  cache/                         # Caching layer
    cache.go                     # In-memory TTL cache wrapping any Provider
  
  tui/                           # UI layer (no platform imports)
    model.go                     # Bubble Tea model, uses provider.types only
    detail.go                    # Detail view
    styles.go                    # All style definitions

  discovery/                     # (removed — folded into provider implementations)
  gitlab/                        # (removed — replaced by provider/gitlab)
  glab/                          # (kept as low-level CLI wrapper, used by provider/gitlab)
```

## Steps

### Step 1: Define platform-neutral types (`internal/provider/types.go`)

```go
type Pipeline struct {
    ID          string
    Project     string        // "org/repo" or "group/project"
    Ref         string        // branch or tag
    SHA         string
    Status      Status        // enum: Running, Passed, Failed, Pending, Canceled
    Source      string
    WebURL      string
    CreatedAt   time.Time
    UpdatedAt   time.Time
    Duration    time.Duration
}

type Job struct {
    ID        string
    Name      string
    Stage     string
    Status    Status
    WebURL    string
    CreatedAt time.Time
    StartedAt time.Time
    Duration  time.Duration
}

type Status int
const (
    StatusPending Status = iota
    StatusRunning
    StatusPassed
    StatusFailed
    StatusCanceled
    StatusSkipped
)
```

Key difference from current: `ID` is string (GitHub uses int64, GitLab uses int), `Duration` is `time.Duration` not float64, `Status` is a typed enum not a raw string.

### Step 2: Define the Provider interface (`internal/provider/provider.go`)

```go
type Provider interface {
    // CurrentUser returns the authenticated user identifier.
    CurrentUser(ctx context.Context) (string, error)

    // ListPipelines returns recent pipelines for the current user.
    // The provider handles discovery logic internally.
    ListPipelines(ctx context.Context, opts ListOptions) ([]Pipeline, error)

    // GetPipeline returns a single pipeline by project and ID.
    GetPipeline(ctx context.Context, project string, id string) (Pipeline, error)

    // ListJobs returns jobs for a pipeline.
    ListJobs(ctx context.Context, project string, pipelineID string) ([]Job, error)
}

type ListOptions struct {
    MaxProjects int
    MaxPages    int
}

// ProgressReporter allows providers to report loading status.
type ProgressReporter interface {
    Report(status string)
}
```

This moves the discovery concept inside the provider — GitLab's implementation scans push events, GitHub's could scan recent workflow runs.

### Step 3: Implement GitLab provider (`internal/provider/gitlab/`)

- Wraps existing `glab.Client` and `discovery.Discoverer`
- Translates `gitlab.Pipeline` / `gitlab.Job` → `provider.Pipeline` / `provider.Job`
- Keeps the existing fetch-by-SHA → fetch-by-ref → fallback cascade
- Accepts a `ProgressReporter` for status updates

### Step 4: Add caching layer (`internal/cache/`)

```go
type CachedProvider struct {
    inner    provider.Provider
    pipes    *ttlCache[[]provider.Pipeline]
    jobs     *ttlCache[[]provider.Job]
    pipeTTL  time.Duration
    jobTTL   time.Duration
}
```

Behaviour:
- `ListPipelines`: return cached immediately (stale-while-revalidate), refresh in background
- `GetPipeline`: short TTL (5s) for live status updates
- `ListJobs`: short TTL (5s) for active pipelines, longer (60s) for completed
- First load: no cache, fetch directly (cold start)
- Cache invalidation: on manual refresh (`r` key), clear all

This gives instant UI on subsequent views while keeping data fresh.

### Step 5: Update TUI to use provider types only

- Replace `gitlab.Pipeline` / `gitlab.Job` imports with `provider.Pipeline` / `provider.Job`
- Replace `glab.ErrGlabNotFound` / `glab.ErrAuthRequired` with provider-level sentinel errors
- `PipelineRow.Pipeline` becomes `provider.Pipeline`
- Status rendering maps from `provider.Status` enum instead of string matching
- `FetchJobs` / `FetchPipeline` / `Refresh` signatures change to use provider types

### Step 6: Update main.go wiring

```go
func main() {
    // Select provider based on config or auto-detection
    var p provider.Provider
    p = gitlab.New(glab.New())
    p = cache.Wrap(p, cache.DefaultTTLs())

    m := tui.New(p)
    // ... tea.NewProgram
}
```

The TUI model takes a `Provider` directly. No more closure injection for `FetchJobs`/`FetchPipeline`/`Refresh` — the model calls the provider interface methods via `tea.Cmd` internally.

### Step 7 (future): GitHub provider

```go
// internal/provider/github/github.go
type Provider struct {
    // Uses `gh` CLI, similar pattern to glab wrapper
}
```

Discovers pipelines from recent workflow runs via `gh api` or `gh run list`. Maps GitHub Actions concepts (workflow → pipeline, job → job) to the provider types.

## Migration Path

1. Steps 1–2 can land without changing any existing behaviour (new package, unused yet).
2. Step 3 wraps existing code — tests pass by comparing old output to new provider output.
3. Step 4 is additive — wrap the provider, no UI changes.
4. Step 5 is the breaking change — the TUI switches from `gitlab.*` to `provider.*`. Do in one commit.
5. Step 6 simplifies main.go — lands same PR as step 5.
6. Old packages (`internal/gitlab/`, `internal/discovery/`) can be removed once the provider is wired.

## Risks and Considerations

- **Status mapping**: GitHub Actions has different statuses (queued, in_progress, completed + conclusion). The `Status` enum needs to cover the union cleanly.
- **Discovery divergence**: GitLab discovers via push events; GitHub might use `gh run list --user`. The interface abstracts this but implementations will differ significantly.
- **Cache staleness**: Stale-while-revalidate means the UI might briefly show old statuses. Acceptable tradeoff for perceived speed.
- **ID types**: GitLab uses int, GitHub uses int64. String ID in the provider type handles both without loss.
