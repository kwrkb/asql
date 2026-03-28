package ui

import (
	"sort"
	"strings"
	"time"
)

// dateLayouts lists time formats to try, ordered from most specific to least.
// RFC3339 is first because dbutil.StringifyValue formats time.Time as RFC3339.
var dateLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02",
	"2006/01/02",
}

var sparkBars = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// detectDateColumn returns true if the column type string suggests a date/timestamp type.
func detectDateColumn(colType string) bool {
	lower := strings.ToLower(colType)
	return strings.Contains(lower, "date") || strings.Contains(lower, "time")
}

// parseDate attempts to parse a string value as time.Time using known layouts.
func parseDate(s string) (time.Time, bool) {
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// looksLikeDate checks the first non-null value in a column to see if it parses as a date.
// This is a heuristic: a TEXT column whose first value happens to be date-shaped
// (e.g. a code field with "2024-01-01") will trigger sparkline computation.
// The trade-off is accepted to support TEXT-typed date columns (common in SQLite).
func looksLikeDate(rows [][]string, colIdx int) bool {
	for _, row := range rows {
		if colIdx >= len(row) {
			continue
		}
		val := row[colIdx]
		if val == "NULL" {
			continue
		}
		_, ok := parseDate(val)
		return ok
	}
	return false
}

type timeGranularity int

const (
	granDay timeGranularity = iota
	granMonth
	granYear
)

const (
	dayGranularityThreshold  = 90 * 24 * time.Hour
	monthGranularityThreshold = 730 * 24 * time.Hour
)

// chooseGranularity selects day/month/year based on the time span.
func chooseGranularity(minT, maxT time.Time) (timeGranularity, string) {
	span := maxT.Sub(minT)
	switch {
	case span <= dayGranularityThreshold:
		return granDay, "by day"
	case span <= monthGranularityThreshold:
		return granMonth, "by month"
	default:
		return granYear, "by year"
	}
}

// truncateTime truncates a time to the start of its bucket based on granularity.
func truncateTime(t time.Time, gran timeGranularity) time.Time {
	switch gran {
	case granDay:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	case granMonth:
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	case granYear:
		return time.Date(t.Year(), 1, 1, 0, 0, 0, 0, t.Location())
	default:
		panic("unhandled time granularity")
	}
}

// bucketKey returns a sortable string key for a time bucket.
func bucketKey(t time.Time, gran timeGranularity) string {
	switch gran {
	case granDay:
		return t.Format("2006-01-02")
	case granMonth:
		return t.Format("2006-01")
	case granYear:
		return t.Format("2006")
	default:
		panic("unhandled time granularity")
	}
}

const maxSparklineBuckets = 20
const maxSparklineRows = 10_000

// computeSparkline scans a column for date values, buckets them by auto-detected
// granularity, and returns a sparklineData with rendered bars.
// Returns zero-value sparklineData if the column has fewer than 2 parseable dates
// or if row count exceeds maxSparklineRows.
func computeSparkline(rows [][]string, colIdx int) sparklineData {
	if len(rows) > maxSparklineRows {
		return sparklineData{Skipped: true}
	}

	times := make([]time.Time, 0, len(rows))
	for _, row := range rows {
		if colIdx >= len(row) {
			continue
		}
		val := row[colIdx]
		if val == "NULL" {
			continue
		}
		if t, ok := parseDate(val); ok {
			times = append(times, t)
		}
	}

	if len(times) < 2 {
		return sparklineData{}
	}

	// Find min/max
	minT, maxT := times[0], times[0]
	for _, t := range times[1:] {
		if t.Before(minT) {
			minT = t
		}
		if t.After(maxT) {
			maxT = t
		}
	}

	gran, label := chooseGranularity(minT, maxT)

	// Bucket by key
	counts := make(map[string]int)
	for _, t := range times {
		key := bucketKey(truncateTime(t, gran), gran)
		counts[key]++
	}

	// Sort keys
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build ordered counts
	ordered := make([]int, len(keys))
	for i, k := range keys {
		ordered[i] = counts[k]
	}

	// Downsample if too many buckets
	if len(ordered) > maxSparklineBuckets {
		ordered = downsample(ordered, maxSparklineBuckets)
	}

	if len(ordered) < 2 {
		return sparklineData{}
	}

	return sparklineData{
		Bars:  renderSparklineBars(ordered),
		Label: label,
	}
}

// downsample merges adjacent buckets to fit within maxBuckets.
func downsample(counts []int, maxBuckets int) []int {
	stride := (len(counts) + maxBuckets - 1) / maxBuckets
	result := make([]int, 0, maxBuckets)
	for i := 0; i < len(counts); i += stride {
		sum := 0
		end := min(i+stride, len(counts))
		for _, v := range counts[i:end] {
			sum += v
		}
		result = append(result, sum)
	}
	return result
}

// renderSparklineBars converts bucket counts to a string of Unicode block characters.
func renderSparklineBars(counts []int) string {
	if len(counts) == 0 {
		return ""
	}

	maxVal := 0
	for _, c := range counts {
		if c > maxVal {
			maxVal = c
		}
	}
	if maxVal == 0 {
		return ""
	}

	bars := make([]rune, len(counts))
	for i, c := range counts {
		if c == 0 {
			bars[i] = ' '
		} else {
			idx := c * (len(sparkBars) - 1) / maxVal
			bars[i] = sparkBars[idx]
		}
	}
	return string(bars)
}
