package glab

import (
	"context"
	"testing"
	"time"
)

func TestFetchUserEventsSince(t *testing.T) {
	dir := t.TempDir()
	response := `[{"id":1,"action_name":"pushed to","project_id":42,"push_data":{"commit_count":1,"ref":"main","ref_type":"branch","commit_to":"abc123"}}]`
	script := fakeGlabScript(t, dir, response)

	c := &Client{BinaryPath: script}
	events, err := c.FetchUserEventsSince(context.Background(), 1, time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].ProjectID != 42 {
		t.Errorf("expected project ID 42, got %d", events[0].ProjectID)
	}
	if events[0].PushData == nil {
		t.Fatal("expected push data to be present")
	}
	if events[0].PushData.CommitTo != "abc123" {
		t.Errorf("expected commit_to abc123, got %q", events[0].PushData.CommitTo)
	}
}

func TestFetchUserEventsSince_Empty(t *testing.T) {
	dir := t.TempDir()
	script := fakeGlabScript(t, dir, "[]")

	c := &Client{BinaryPath: script}
	events, err := c.FetchUserEventsSince(context.Background(), 1, time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
}

func TestFetchProject(t *testing.T) {
	dir := t.TempDir()
	response := `{"id":42,"path_with_namespace":"group/project","name":"project"}`
	script := fakeGlabScript(t, dir, response)

	c := &Client{BinaryPath: script}
	project, err := c.FetchProject(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if project.PathWithNamespace != "group/project" {
		t.Errorf("expected path group/project, got %q", project.PathWithNamespace)
	}
}

func TestFetchProject_ParseError(t *testing.T) {
	dir := t.TempDir()
	script := fakeGlabScript(t, dir, "not json")

	c := &Client{BinaryPath: script}
	_, err := c.FetchProject(context.Background(), 42)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestFetchPipelinesByUser(t *testing.T) {
	dir := t.TempDir()
	response := `[{"id":10,"status":"success"},{"id":9,"status":"failed"}]`
	script := fakeGlabScript(t, dir, response)

	c := &Client{BinaryPath: script}
	pipelines, err := c.FetchPipelinesByUser(context.Background(), 42, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pipelines) != 2 {
		t.Fatalf("expected 2 pipelines, got %d", len(pipelines))
	}
}

func TestCurrentUser_ParseError(t *testing.T) {
	dir := t.TempDir()
	script := fakeGlabScript(t, dir, "not json")

	c := &Client{BinaryPath: script}
	_, err := c.CurrentUser(context.Background())
	if err == nil {
		t.Fatal("expected parse error")
	}
}
