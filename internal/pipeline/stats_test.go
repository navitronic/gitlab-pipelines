package pipeline

import (
	"testing"
	"time"
)

func TestSummarizePipelineStats(t *testing.T) {
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)
	pipelines := []Pipeline{
		{Status: StatusPassed, CreatedAt: time.Date(2024, 3, 15, 9, 0, 0, 0, time.UTC)},
		{Status: StatusFailed, CreatedAt: time.Date(2024, 3, 15, 8, 0, 0, 0, time.UTC)},
		{Status: StatusRunning, CreatedAt: time.Date(2024, 3, 14, 23, 0, 0, 0, time.UTC)},
		{Status: StatusPassed, CreatedAt: time.Date(2024, 3, 10, 10, 0, 0, 0, time.UTC)},
		{Status: StatusPassed, CreatedAt: time.Date(2024, 3, 1, 10, 0, 0, 0, time.UTC)},
	}

	stats := SummarizePipelineStats("group/project", pipelines, 7, now)

	if stats.Project != "group/project" {
		t.Errorf("Project = %q, want group/project", stats.Project)
	}
	if len(stats.Days) != 7 {
		t.Fatalf("expected 7 days, got %d", len(stats.Days))
	}
	// Oldest day first: 9 March ... 15 March.
	if got := stats.Days[0].Date.Format("2006-01-02"); got != "2024-03-09" {
		t.Errorf("first day = %s, want 2024-03-09", got)
	}
	if got := stats.Days[6].Date.Format("2006-01-02"); got != "2024-03-15" {
		t.Errorf("last day = %s, want 2024-03-15", got)
	}

	last := stats.Days[6]
	if last.Total != 2 || last.Passed != 1 || last.Failed != 1 || last.Other != 0 {
		t.Errorf("today = %+v, want Total=2 Passed=1 Failed=1 Other=0", last)
	}
	prev := stats.Days[5]
	if prev.Total != 1 || prev.Other != 1 {
		t.Errorf("previous day = %+v, want Total=1 Other=1", prev)
	}
	if stats.Total() != 4 {
		t.Errorf("Total() = %d, want 4 (the 1 March pipeline is out of window)", stats.Total())
	}
}

func TestSummarizePipelineStats_Empty(t *testing.T) {
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)
	stats := SummarizePipelineStats("group/project", nil, 7, now)

	if len(stats.Days) != 7 {
		t.Fatalf("expected 7 days, got %d", len(stats.Days))
	}
	if stats.Total() != 0 {
		t.Errorf("Total() = %d, want 0", stats.Total())
	}
}

func TestSummarizePipelineStats_DefaultsDays(t *testing.T) {
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)
	stats := SummarizePipelineStats("group/project", nil, 0, now)
	if len(stats.Days) != 7 {
		t.Fatalf("expected 7 days when days <= 0, got %d", len(stats.Days))
	}
}

func TestSummarizePipelineStats_LocalDayBoundary(t *testing.T) {
	loc := time.FixedZone("UTC+2", 2*60*60)
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, loc)
	// 23:30 UTC on 14 March is 01:30 local on 15 March, so it belongs to today.
	pipelines := []Pipeline{
		{Status: StatusPassed, CreatedAt: time.Date(2024, 3, 14, 23, 30, 0, 0, time.UTC)},
	}

	stats := SummarizePipelineStats("group/project", pipelines, 7, now)

	today := stats.Days[6]
	if today.Total != 1 {
		t.Errorf("today Total = %d, want 1 (UTC timestamp should bucket by local day)", today.Total)
	}
}
