package pipeline

import "time"

// DailyPipelineStats holds the pipeline run counts for a single calendar day.
type DailyPipelineStats struct {
	Date   time.Time
	Total  int
	Passed int
	Failed int
	Other  int
}

// PipelineStats holds per-day pipeline run counts for a project, oldest day
// first.
type PipelineStats struct {
	Project string
	Days    []DailyPipelineStats
}

// Total returns the number of pipeline runs across all days.
func (s PipelineStats) Total() int {
	total := 0
	for _, d := range s.Days {
		total += d.Total
	}
	return total
}

// SummarizePipelineStats buckets pipelines into per-day run counts for the
// given number of days ending on now, oldest day first. Days are calendar days
// in now's location. Pipelines outside the window are ignored.
func SummarizePipelineStats(project string, pipelines []Pipeline, days int, now time.Time) PipelineStats {
	if days <= 0 {
		days = 7
	}
	loc := now.Location()
	start := dayStart(now, loc).AddDate(0, 0, -(days - 1))

	index := make(map[string]int, days)
	stats := PipelineStats{Project: project, Days: make([]DailyPipelineStats, days)}
	for i := 0; i < days; i++ {
		d := start.AddDate(0, 0, i)
		stats.Days[i].Date = d
		index[d.Format("2006-01-02")] = i
	}

	for _, p := range pipelines {
		i, ok := index[dayStart(p.CreatedAt, loc).Format("2006-01-02")]
		if !ok {
			continue
		}
		day := &stats.Days[i]
		day.Total++
		switch p.Status {
		case StatusPassed:
			day.Passed++
		case StatusFailed:
			day.Failed++
		default:
			day.Other++
		}
	}
	return stats
}

func dayStart(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}
