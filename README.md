# GitLab Pipelines TUI — Implementation Plan

## Overview

Build a terminal UI (TUI) application in Go that displays GitLab pipelines relevant to the current user’s activity.

All communication with GitLab must be performed via the `glab` CLI installed on the host. No direct HTTP requests to GitLab APIs are permitted.

---

## Core Requirements

### Functional

- Retrieve current user via `glab`
- Fetch recent user activity
- Infer relevant pipelines from that activity
- Display pipelines in a table
- Allow selection of a pipeline to view job-level details

### Non-Functional

- Must use Charm TUI ecosystem
- Must shell out to `glab` for all GitLab data
- Must gracefully handle missing or unauthenticated `glab`
- Must support pagination via repeated `glab api` calls

---

## Tech Stack

- Go (1.22+)
- Charm libraries:
  - bubbletea
  - bubbles
  - lipgloss
- External dependency:
  - glab CLI (required at runtime)

---

## Architecture

TUI (Bubble Tea)
  ↓
Application State
  ↓
Discovery Layer
  ↓
Glab Client (shells out)
  ↓
glab api
  ↓
GitLab

---

## glab Integration (MANDATORY)

All GitLab access must use:

    glab api <endpoint>

### Example Commands

    glab api user
    glab api "users/:id/events?per_page=100"
    glab api "projects/:id/pipelines?per_page=100"
    glab api "projects/:id/pipelines/:pipeline_id/jobs"

### Go Execution Pattern

```go
func runGlab(ctx context.Context, args ...string) ([]byte, error) {
    cmd := exec.CommandContext(ctx, "glab", args...)
    return cmd.Output()
}
```

### Constraints

- Do NOT use net/http to call GitLab
- Do NOT require GitLab tokens directly
- Assume `glab auth login` has already been completed

---

## Data Flow

1. Fetch current user
2. Fetch user events
3. Extract pipeline candidates
4. Fetch pipelines per candidate
5. Fetch jobs for selected pipeline
6. Render UI

---

## Step 1: Fetch Current User

    glab api user

---

## Step 2: Fetch User Activity

    glab api "users/$USER_ID/events?per_page=100"

Extract:
- project ID/path
- branch (ref)
- commit SHA
- event type
- timestamp

---

## Step 3: Build Pipeline Candidates

### Candidate Model

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

### Deduplication

project_id + ref + sha

---

## Step 4: Fetch Pipelines

### By SHA

    glab api "projects/:id/pipelines?sha=$SHA&user_id=$USER_ID"

### By ref

    glab api "projects/:id/pipelines?ref=$REF&user_id=$USER_ID"

### Fallback

    glab api "projects/:id/pipelines?user_id=$USER_ID&order_by=updated_at&sort=desc"

---

## Step 5: Fetch Pipeline Jobs

    glab api "projects/:id/pipelines/:pipeline_id/jobs"

---

## TUI Design

### Pipeline List

Columns:
Status | Project | Ref | Commit | Pipeline | Jobs | Updated | Source

### Pipeline Details

Displays pipeline metadata and job list.

---

## Project Structure

cmd/gitlab-pipelines/main.go

internal/glab/
internal/discovery/
internal/gitlab/
internal/tui/
internal/config/

---

## Milestones

1. CLI Prototype
2. Basic TUI
3. Pipeline Details
4. Improved Discovery
5. Packaging

---

## Constraints

- MUST use glab for all GitLab interaction
- MUST NOT use direct HTTP calls
- MUST support pagination manually

---

## MVP Scope

- user retrieval
- recent events
- pipeline discovery
- pipeline list UI
- pipeline details UI

---

## Summary

A Go-based TUI that uses glab exclusively to surface relevant GitLab pipelines and jobs based on user activity.
