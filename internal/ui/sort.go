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
	return compareKeys(newSortKey(a), newSortKey(b))
}

// sortKey is a cell value parsed once for every way compareKeys may compare
// it. Parsing inside the comparator repeated the work for every comparison —
// on the order of n·log n ParseInt/ParseFloat calls per column sort, run in
// the key handler.
type sortKey struct {
	s       string
	i       int64
	f       float64
	t       time.Time
	isInt   bool
	isFloat bool
	isTime  bool
}

func newSortKey(s string) sortKey {
	k := sortKey{s: s}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		// float64(i) is what ParseFloat would return — both round to the
		// nearest double — so an integer skips the second parse. It still
		// needs the float form for a comparison against a fraction.
		k.i, k.isInt = i, true
		k.f, k.isFloat = float64(i), true
		return k
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		k.f, k.isFloat = f, true
	}
	if !k.isFloat {
		k.t, k.isTime = parseTimestamp(s)
	}
	return k
}

// compareKeys decides pair by pair, so a column mixing numbers and text
// orders each pair the way the two values allow.
func compareKeys(a, b sortKey) int {
	// Integers are compared as integers. Going through float64 folds every
	// integer past 2^53 onto a neighbour, so 9007199254740993 and
	// 9007199254740992 compared equal: the sort left them in query order and
	// the column min and max came out the same value, while the display
	// string — the database's own int64 — still showed the difference.
	if a.isInt && b.isInt {
		return cmp.Compare(a.i, b.i)
	}

	if a.isFloat && b.isFloat {
		switch {
		case a.f < b.f:
			return -1
		case a.f > b.f:
			return 1
		}
		// Equal as float64 is not equal: a BIGINT UNSIGNED past int64, a
		// DECIMAL with more digits than a double keeps, or an integer against
		// a fraction, all round to the same double. Only ties pay for the
		// exact comparison, which is what keeps it off the common path.
		if ar, ok := new(big.Rat).SetString(a.s); ok {
			if br, ok := new(big.Rat).SetString(b.s); ok {
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
	if a.isTime && b.isTime {
		return a.t.Compare(b.t)
	}

	return strings.Compare(a.s, b.s)
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
// every other value — the result table's rule, which sortedRows applies
// itself on its pre-parsed keys. Only the tests call this now: it pins that
// rule down in one comparable function.
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

	// Decorate: parse each cell once, not once per comparison.
	type decorated struct {
		idx  int
		null bool
		key  sortKey
	}
	keys := make([]decorated, len(rows))
	for i, row := range rows {
		var v string
		if col < len(row) {
			v = row[col]
		}
		keys[i] = decorated{idx: i, null: v == db.NullSentinel}
		if !keys[i].null {
			keys[i].key = newSortKey(v)
		}
	}

	sort.SliceStable(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		// NULL always sorts last, regardless of direction.
		if a.null != b.null {
			return b.null
		}
		if a.null {
			return false
		}
		c := compareKeys(a.key, b.key)
		if dir == sortDesc {
			c = -c
		}
		return c < 0
	})

	result := make([][]string, len(rows))
	for i, k := range keys {
		result[i] = rows[k.idx]
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
