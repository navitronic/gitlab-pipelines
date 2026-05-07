package glab

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestNew(t *testing.T) {
	c := New()
	if c == nil {
		t.Fatal("New() returned nil")
	}
	if c.BinaryPath != "glab" {
		t.Errorf("BinaryPath = %q, want \"glab\"", c.BinaryPath)
	}
}

func TestRun_BinaryNotFound(t *testing.T) {
	c := &Client{BinaryPath: "/nonexistent/path/glab-fake"}
	_, err := c.Run(context.Background(), "api", "user")
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
}

func TestRun_EmptyBinaryPath(t *testing.T) {
	c := &Client{BinaryPath: ""}
	_, err := c.Run(context.Background(), "version")
	_ = err
}

func TestRun_ExitError(t *testing.T) {
	dir := t.TempDir()
	var script string
	if runtime.GOOS == "windows" {
		script = filepath.Join(dir, "glab.bat")
		os.WriteFile(script, []byte("@echo off\necho something went wrong >&2\nexit /b 1\n"), 0o755)
	} else {
		script = filepath.Join(dir, "glab")
		os.WriteFile(script, []byte("#!/bin/sh\necho 'something went wrong' >&2\nexit 1\n"), 0o755)
	}

	c := &Client{BinaryPath: script}
	_, err := c.Run(context.Background(), "api", "user")
	if err == nil {
		t.Fatal("expected error for non-zero exit")
	}
}

func TestRun_AuthError(t *testing.T) {
	dir := t.TempDir()
	var script string
	if runtime.GOOS == "windows" {
		script = filepath.Join(dir, "glab.bat")
		os.WriteFile(script, []byte("@echo off\necho 401 auth required >&2\nexit /b 1\n"), 0o755)
	} else {
		script = filepath.Join(dir, "glab")
		os.WriteFile(script, []byte("#!/bin/sh\necho '401 auth required' >&2\nexit 1\n"), 0o755)
	}

	c := &Client{BinaryPath: script}
	_, err := c.Run(context.Background(), "api", "user")
	if err == nil {
		t.Fatal("expected error for auth failure")
	}
	if err != ErrAuthRequired {
		t.Fatalf("expected ErrAuthRequired, got: %v", err)
	}
}

func TestCurrentUser_ParsesJSON(t *testing.T) {
	dir := t.TempDir()
	var script string
	response := `{"id":123,"username":"testuser","name":"Test User","email":"test@example.com"}`
	if runtime.GOOS == "windows" {
		script = filepath.Join(dir, "glab.bat")
		os.WriteFile(script, []byte("@echo off\necho "+response+"\n"), 0o755)
	} else {
		script = filepath.Join(dir, "glab")
		os.WriteFile(script, []byte("#!/bin/sh\necho '"+response+"'\n"), 0o755)
	}

	c := &Client{BinaryPath: script}
	user, err := c.CurrentUser(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != 123 {
		t.Errorf("expected user ID 123, got %d", user.ID)
	}
	if user.Username != "testuser" {
		t.Errorf("expected username 'testuser', got %q", user.Username)
	}
}
