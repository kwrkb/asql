package ui

import (
	"sort"
	"strconv"
	"strings"
)

type sortOrder int

const (
	sortNone sortOrder = iota
	sortAsc
	sortDesc
)

// smartCompare compares two cell values.
// NULL is always sorted last. Numeric values are compared numerically.
func smartCompare(a, b string) int {
	aNULL := a == "NULL"
	bNULL := b == "NULL"
	if aNULL && bNULL {
		return 0
	}
	if aNULL {
		return 1
	}
	if bNULL {
		return -1
	}

	af, aErr := strconv.ParseFloat(a, 64)
	bf, bErr := strconv.ParseFloat(b, 64)
	if aErr == nil && bErr == nil {
		switch {
		case af < bf:
			return -1
		case af > bf:
			return 1
		default:
			return 0
		}
	}

	return strings.Compare(a, b)
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
		aNULL := a == "NULL"
		bNULL := b == "NULL"
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
