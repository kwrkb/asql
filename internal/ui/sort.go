package ui

import (
	"cmp"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kwrkb/asql/internal/db"
)

type sortOrder int

const (
	sortNone sortOrder = iota
	sortAsc
	sortDesc
)

// compareValues orders two cell values with no NULL handling at all: they are
// compared numerically when both parse as numbers, chronologically when both
// parse as timestamps, and lexically otherwise.
//
// Callers that have already separated out the NULLs — computeColumnStats skips
// them before computing min/max — must use this rather than smartCompare, or a
// value whose text is literally "NULL" gets ordered as though it were one.
func compareValues(a, b string) int {
	// Integers are compared as integers. Going through float64 folds every
	// integer past 2^53 onto a neighbour, so 9007199254740993 and
	// 9007199254740992 compared equal: the sort left them in query order and
	// the column min and max came out the same value, while the display
	// string — the database's own int64 — still showed the difference.
	ai, aErr := strconv.ParseInt(a, 10, 64)
	bi, bErr := strconv.ParseInt(b, 10, 64)
	if aErr == nil && bErr == nil {
		return cmp.Compare(ai, bi)
	}

	af, aErr := strconv.ParseFloat(a, 64)
	bf, bErr := strconv.ParseFloat(b, 64)
	if aErr == nil && bErr == nil {
		switch {
		case af < bf:
			return -1
		case af > bf:
			return 1
		}
		// Equal as float64 is not equal: a BIGINT UNSIGNED past int64, a
		// DECIMAL with more digits than a double keeps, or an integer against
		// a fraction, all round to the same double. Only ties pay for the
		// exact comparison, which is what keeps it off the common path.
		if ar, ok := new(big.Rat).SetString(a); ok {
			if br, ok := new(big.Rat).SetString(b); ok {
				return ar.Cmp(br)
			}
		}
		return 0 // NaN or Inf: no exact form, and they were equal as floats
	}

	// Timestamps are compared as instants rather than as text. Lexical order
	// happens to be chronological for most date shapes, but not for the one
	// the database hands back: dbutil formats time.Time with RFC3339Nano,
	// which trims trailing zeros from the fraction and drops it entirely on a
	// whole second, so "…:00Z" and "…:00.1Z" put '.' (0x2E) against 'Z'
	// (0x5A) and sort the later value first — in the result table and in the
	// column min/max alike.
	if at, ok := parseTimestamp(a); ok {
		if bt, ok := parseTimestamp(b); ok {
			return at.Compare(bt)
		}
	}

	return strings.Compare(a, b)
}

// parseTimestamp is parseDate behind a shape check, so an ordinary text column
// does not pay for a run of failed time.Parse calls on every comparison. Every
// layout parseDate knows carries a separator at index 4.
func parseTimestamp(s string) (time.Time, bool) {
	if len(s) < len("2006-01-02") || (s[4] != '-' && s[4] != '/') {
		return time.Time{}, false
	}
	return parseDate(s)
}

// smartCompare compares two display strings, ordering the NULL sentinel after
// every other value.
//
// It matches on the display string, so a value whose text is literally "NULL"
// sorts with the real NULLs. That is intended for the result table, where the
// two are drawn identically and separating them would look like a bug; it is
// wrong anywhere the caller can tell them apart, which is what compareValues
// is for.
func smartCompare(a, b string) int {
	aNULL := a == db.NullSentinel
	bNULL := b == db.NullSentinel
	if aNULL && bNULL {
		return 0
	}
	if aNULL {
		return 1
	}
	if bNULL {
		return -1
	}
	return compareValues(a, b)
}

// sortedRows returns a sorted copy of rows by the given column index and order.
// The original slice is not modified.
func sortedRows(rows [][]string, col int, dir sortOrder) [][]string {
	if dir == sortNone || len(rows) == 0 {
		return rows
	}

	indices := make([]int, len(rows))
	for i := range indices {
		indices[i] = i
	}

	sort.SliceStable(indices, func(i, j int) bool {
		ai, bi := indices[i], indices[j]
		var a, b string
		if col < len(rows[ai]) {
			a = rows[ai][col]
		}
		if col < len(rows[bi]) {
			b = rows[bi][col]
		}
		// NULL always sorts last, regardless of direction.
		aNULL := a == db.NullSentinel
		bNULL := b == db.NullSentinel
		if aNULL != bNULL {
			return bNULL
		}
		if aNULL && bNULL {
			return false
		}
		cmp := smartCompare(a, b)
		if dir == sortDesc {
			cmp = -cmp
		}
		return cmp < 0
	})

	result := make([][]string, len(rows))
	for i, idx := range indices {
		result[i] = rows[idx]
	}
	return result
}

// sortIndicator returns the sort direction symbol for a column header.
func sortIndicator(dir sortOrder) string {
	switch dir {
	case sortAsc:
		return " ▲"
	case sortDesc:
		return " ▼"
	default:
		return ""
	}
}
