package ui

import (
	"testing"
	"time"

	"github.com/kwrkb/asql/internal/db"
)

func TestDetectDateColumn(t *testing.T) {
	tests := []struct {
		colType string
		want    bool
	}{
		{"DATE", true},
		{"DATETIME", true},
		{"TIMESTAMP", true},
		{"timestamp with time zone", true},
		{"timestamptz", true},
		{"INTEGER", false},
		{"TEXT", false},
		{"VARCHAR", false},
		{"", false},
	}
	for _, tt := range tests {
		got := detectDateColumn(tt.colType)
		if got != tt.want {
			t.Errorf("detectDateColumn(%q) = %v, want %v", tt.colType, got, tt.want)
		}
	}
}

func TestParseDate(t *testing.T) {
	tests := []struct {
		input string
		ok    bool
	}{
		{"2024-01-15T10:30:00Z", true},          // RFC3339
		{"2024-01-15T10:30:00+09:00", true},      // RFC3339 with offset
		{"2024-01-15T10:30:00", true},             // ISO 8601 no TZ
		{"2024-01-15 10:30:00", true},             // datetime
		{"2024-01-15", true},                      // date only
		{"2024/01/15", true},                      // slash
		{"not-a-date", false},
		{"12345", false},
		{"", false},
	}
	for _, tt := range tests {
		_, ok := parseDate(tt.input)
		if ok != tt.ok {
			t.Errorf("parseDate(%q) ok = %v, want %v", tt.input, ok, tt.ok)
		}
	}
}

func TestLooksLikeDate(t *testing.T) {
	rows := [][]string{
		{"NULL", "hello"},
		{"2024-01-01", "world"},
	}
	if !looksLikeDate(rows, 0) {
		t.Error("column 0 should look like date")
	}
	if looksLikeDate(rows, 1) {
		t.Error("column 1 should not look like date")
	}
}

func TestLooksLikeDate_AllNull(t *testing.T) {
	rows := [][]string{{"NULL"}, {"NULL"}}
	if looksLikeDate(rows, 0) {
		t.Error("all-null column should not look like date")
	}
}

func TestChooseGranularity(t *testing.T) {
	base, _ := time.Parse("2006-01-02", "2024-01-01")

	tests := []struct {
		name     string
		max      string
		wantGran timeGranularity
		wantLbl  string
	}{
		{"30 days", "2024-01-31", granDay, "by day"},
		{"90 days", "2024-03-31", granDay, "by day"},
		{"6 months", "2024-07-01", granMonth, "by month"},
		{"18 months", "2025-07-01", granMonth, "by month"},
		{"3 years", "2027-01-01", granYear, "by year"},
		{"10 years", "2034-01-01", granYear, "by year"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			maxT, _ := time.Parse("2006-01-02", tt.max)
			gran, lbl := chooseGranularity(base, maxT)
			if gran != tt.wantGran {
				t.Errorf("granularity = %d, want %d", gran, tt.wantGran)
			}
			if lbl != tt.wantLbl {
				t.Errorf("label = %q, want %q", lbl, tt.wantLbl)
			}
		})
	}
}

func TestRenderSparklineBars(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		bars := renderSparklineBars([]int{1, 4, 8, 4, 1})
		if len([]rune(bars)) != 5 {
			t.Errorf("expected 5 bars, got %d", len([]rune(bars)))
		}
		// Middle should be highest
		runes := []rune(bars)
		if runes[2] != '█' {
			t.Errorf("max bucket should be █, got %c", runes[2])
		}
	})

	t.Run("all equal", func(t *testing.T) {
		bars := renderSparklineBars([]int{5, 5, 5})
		runes := []rune(bars)
		for i, r := range runes {
			if r != '█' {
				t.Errorf("equal buckets: bar[%d] = %c, want █", i, r)
			}
		}
	})

	t.Run("with zero", func(t *testing.T) {
		bars := renderSparklineBars([]int{0, 5, 0})
		runes := []rune(bars)
		if runes[0] != ' ' {
			t.Errorf("zero bucket should be space, got %c", runes[0])
		}
		if runes[1] != '█' {
			t.Errorf("max bucket should be █, got %c", runes[1])
		}
	})

	t.Run("empty", func(t *testing.T) {
		if bars := renderSparklineBars(nil); bars != "" {
			t.Errorf("nil counts should return empty, got %q", bars)
		}
	})

	t.Run("all zero", func(t *testing.T) {
		if bars := renderSparklineBars([]int{0, 0, 0}); bars != "" {
			t.Errorf("all-zero counts should return empty, got %q", bars)
		}
	})
}

func TestDownsample(t *testing.T) {
	counts := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	result := downsample(counts, 5)
	if len(result) != 5 {
		t.Fatalf("len = %d, want 5", len(result))
	}
	// Pairs: (1+2), (3+4), (5+6), (7+8), (9+10)
	expected := []int{3, 7, 11, 15, 19}
	for i, v := range result {
		if v != expected[i] {
			t.Errorf("result[%d] = %d, want %d", i, v, expected[i])
		}
	}
}

func TestComputeSparkline_DateColumn(t *testing.T) {
	rows := [][]string{
		{"2024-01-15"},
		{"2024-01-20"},
		{"2024-02-10"},
		{"2024-02-15"},
		{"2024-02-20"},
		{"2024-03-01"},
	}
	sd := computeSparkline(rows, 0)
	if sd.Bars == "" {
		t.Fatal("expected sparkline bars for date column")
	}
	if sd.Label == "" {
		t.Fatal("expected sparkline label")
	}
}

func TestComputeSparkline_NonDateColumn(t *testing.T) {
	rows := [][]string{{"hello"}, {"world"}, {"foo"}}
	sd := computeSparkline(rows, 0)
	if sd.Bars != "" {
		t.Errorf("non-date column should have empty bars, got %q", sd.Bars)
	}
}

func TestComputeSparkline_AllNull(t *testing.T) {
	rows := [][]string{{"NULL"}, {"NULL"}, {"NULL"}}
	sd := computeSparkline(rows, 0)
	if sd.Bars != "" {
		t.Errorf("all-null should have empty bars, got %q", sd.Bars)
	}
}

func TestComputeSparkline_SingleDate(t *testing.T) {
	rows := [][]string{{"2024-01-01"}, {"NULL"}}
	sd := computeSparkline(rows, 0)
	if sd.Bars != "" {
		t.Errorf("single date should have empty bars, got %q", sd.Bars)
	}
}

func TestComputeSparkline_SameDateAllRows(t *testing.T) {
	rows := [][]string{{"2024-01-01"}, {"2024-01-01"}, {"2024-01-01"}}
	sd := computeSparkline(rows, 0)
	if sd.Bars != "" {
		t.Errorf("same date for all rows should have empty bars (1 bucket), got %q", sd.Bars)
	}
}

func TestComputeColumnStats_DateSparkline(t *testing.T) {
	result := db.QueryResult{
		Columns:     []string{"created_at", "name"},
		ColumnTypes: []string{"DATE", "TEXT"},
		Rows: [][]string{
			{"2024-01-15", "alice"},
			{"2024-02-10", "bob"},
			{"2024-03-01", "carol"},
		},
	}
	stats := computeColumnStats(result)
	if stats[0].Sparkline.Bars == "" {
		t.Error("DATE column should have sparkline bars")
	}
	if stats[1].Sparkline.Bars != "" {
		t.Error("TEXT column should not have sparkline bars")
	}
}

func TestComputeColumnStats_TextLooksLikeDate(t *testing.T) {
	result := db.QueryResult{
		Columns:     []string{"created"},
		ColumnTypes: []string{"TEXT"},
		Rows: [][]string{
			{"2024-01-01"},
			{"2024-02-01"},
			{"2024-03-01"},
		},
	}
	stats := computeColumnStats(result)
	if stats[0].Sparkline.Bars == "" {
		t.Error("TEXT column with date values should have sparkline bars")
	}
}

