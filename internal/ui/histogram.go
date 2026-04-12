package ui

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

const maxHistogramBuckets = 20
const maxHistogramRows = 10_000

// numericTypeKeywords lists SQL type keywords that indicate a numeric column.
// Matched as word prefixes to avoid false positives (e.g. "INTERVAL" matching "int").
var numericTypeKeywords = []string{
	"int", "integer", "bigint", "smallint", "tinyint", "mediumint",
	"real", "float", "double", "decimal", "numeric", "number",
}

// detectNumericColumn returns true if the column type string suggests a numeric type.
// Uses word-boundary matching to avoid false positives like INTERVAL or POINT.
func detectNumericColumn(colType string) bool {
	lower := strings.ToLower(colType)
	for _, kw := range numericTypeKeywords {
		idx := strings.Index(lower, kw)
		if idx < 0 {
			continue
		}
		// Check left boundary: start of string, space, or '('
		if idx > 0 && lower[idx-1] != ' ' && lower[idx-1] != '(' {
			continue
		}
		// Check right boundary: end of string, space, '(', or ')'
		end := idx + len(kw)
		if end < len(lower) && lower[end] != ' ' && lower[end] != '(' && lower[end] != ')' {
			continue
		}
		return true
	}
	return false
}

// looksLikeNumeric checks the first non-null value to see if it parses as float64.
// Mirrors looksLikeDate: a single-sample heuristic; computeHistogram's 50% threshold
// handles mixed-type columns downstream.
func looksLikeNumeric(rows [][]string, colIdx int) bool {
	for _, row := range rows {
		if colIdx >= len(row) {
			continue
		}
		val := row[colIdx]
		if val == "NULL" {
			continue
		}
		_, err := strconv.ParseFloat(val, 64)
		return err == nil
	}
	return false
}

// computeHistogram scans a numeric column, buckets values into equal-width bins,
// and returns a histogramData with rendered bars.
// Returns zero-value histogramData if the column has fewer than 2 parseable values
// or if row count exceeds maxHistogramRows.
func computeHistogram(rows [][]string, colIdx int) histogramData {
	if len(rows) > maxHistogramRows {
		return histogramData{Skipped: true}
	}

	values := make([]float64, 0, len(rows))
	nonNull := 0
	for _, row := range rows {
		if colIdx >= len(row) {
			continue
		}
		val := row[colIdx]
		if val == "NULL" {
			continue
		}
		nonNull++
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			continue
		}
		if math.IsNaN(f) || math.IsInf(f, 0) {
			nonNull-- // NaN/Inf are not usable for histogram; exclude from ratio
			continue
		}
		values = append(values, f)
	}

	// Suppress histogram if less than half of non-NULL values are numeric,
	// to avoid misleading distributions on mixed-type columns.
	if nonNull > 0 && len(values)*2 < nonNull {
		return histogramData{}
	}

	if len(values) < 2 {
		return histogramData{}
	}

	minVal, maxVal := values[0], values[0]
	for _, v := range values[1:] {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	if minVal == maxVal {
		return histogramData{}
	}

	numBuckets := max(min(len(values), maxHistogramBuckets), 2)

	width := (maxVal - minVal) / float64(numBuckets)
	counts := make([]int, numBuckets)
	for _, v := range values {
		idx := int((v - minVal) / width)
		if idx >= numBuckets {
			idx = numBuckets - 1
		}
		counts[idx]++
	}

	bars := renderSparklineBars(counts)
	if bars == "" {
		return histogramData{}
	}

	label := formatHistogramLabel(minVal, maxVal)

	return histogramData{
		Bars:  bars,
		Label: label,
	}
}

// formatHistogramLabel returns a compact range label for the histogram.
// Uses integer format if both values are whole numbers, otherwise %.4g.
func formatHistogramLabel(minVal, maxVal float64) string {
	if minVal == math.Trunc(minVal) && maxVal == math.Trunc(maxVal) {
		return fmt.Sprintf("%.0f–%.0f", minVal, maxVal)
	}
	return fmt.Sprintf("%.4g–%.4g", minVal, maxVal)
}
