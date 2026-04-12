package ui

import (
	"strings"
	"testing"
)

func TestDetectNumericColumn(t *testing.T) {
	tests := []struct {
		colType string
		want    bool
	}{
		{"INTEGER", true},
		{"INT", true},
		{"BIGINT", true},
		{"SMALLINT", true},
		{"TINYINT", true},
		{"REAL", true},
		{"FLOAT", true},
		{"DOUBLE", true},
		{"DECIMAL", true},
		{"NUMERIC", true},
		{"NUMBER", true},
		{"integer", true},    // lowercase
		{"int unsigned", true},
		{"double precision", true},
		{"TEXT", false},
		{"VARCHAR(255)", false},
		{"DATE", false},
		{"TIMESTAMP", false},
		{"BLOB", false},
		{"INTERVAL", false},   // must not match "int" substring
		{"POINT", false},      // must not match "int" substring
		{"interval", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.colType, func(t *testing.T) {
			got := detectNumericColumn(tt.colType)
			if got != tt.want {
				t.Errorf("detectNumericColumn(%q) = %v, want %v", tt.colType, got, tt.want)
			}
		})
	}
}

func TestLooksLikeNumeric(t *testing.T) {
	tests := []struct {
		name   string
		rows   [][]string
		colIdx int
		want   bool
	}{
		{
			name:   "integer values",
			rows:   [][]string{{"1"}, {"2"}, {"3"}},
			colIdx: 0,
			want:   true,
		},
		{
			name:   "float values",
			rows:   [][]string{{"1.5"}, {"2.7"}, {"3.14"}},
			colIdx: 0,
			want:   true,
		},
		{
			name:   "negative values",
			rows:   [][]string{{"-1"}, {"-2"}, {"3"}},
			colIdx: 0,
			want:   true,
		},
		{
			name:   "non-numeric text",
			rows:   [][]string{{"alice"}, {"bob"}},
			colIdx: 0,
			want:   false,
		},
		{
			name:   "date string not numeric",
			rows:   [][]string{{"2024-01-01"}, {"2024-01-02"}},
			colIdx: 0,
			want:   false,
		},
		{
			name:   "all NULL",
			rows:   [][]string{{"NULL"}, {"NULL"}},
			colIdx: 0,
			want:   false,
		},
		{
			name:   "NULL then numeric",
			rows:   [][]string{{"NULL"}, {"42"}, {"99"}},
			colIdx: 0,
			want:   true,
		},
		{
			name:   "empty rows",
			rows:   [][]string{},
			colIdx: 0,
			want:   false,
		},
		{
			name:   "out of bounds colIdx",
			rows:   [][]string{{"1", "2"}},
			colIdx: 5,
			want:   false,
		},
		{
			name:   "first is numeric even if later values are not",
			rows:   [][]string{{"42"}, {"N/A"}, {"100"}},
			colIdx: 0,
			want:   true,
		},
		{
			name:   "first non-NULL is not numeric",
			rows:   [][]string{{"N/A"}, {"42"}, {"100"}},
			colIdx: 0,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := looksLikeNumeric(tt.rows, tt.colIdx)
			if got != tt.want {
				t.Errorf("looksLikeNumeric() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComputeHistogram(t *testing.T) {
	tests := []struct {
		name        string
		rows        [][]string
		colIdx      int
		wantEmpty   bool
		wantSkipped bool
		wantBars    string // if non-empty, exact match; if "*", just non-empty
		wantLabel   string // if non-empty, check contains
	}{
		{
			name: "basic uniform distribution",
			rows: func() [][]string {
				r := make([][]string, 0, 100)
				for i := 0; i < 100; i++ {
					r = append(r, []string{string(rune('0'+i%10))})
				}
				// Use explicit values instead
				return [][]string{
					{"0"}, {"10"}, {"20"}, {"30"}, {"40"},
					{"50"}, {"60"}, {"70"}, {"80"}, {"90"},
				}
			}(),
			colIdx:    0,
			wantEmpty: false,
			wantBars:  "*",
			wantLabel: "0–90",
		},
		{
			name: "integer range label",
			rows: [][]string{
				{"1"}, {"2"}, {"3"}, {"4"}, {"5"},
				{"6"}, {"7"}, {"8"}, {"9"}, {"10"},
			},
			colIdx:    0,
			wantEmpty: false,
			wantBars:  "*",
			wantLabel: "1–10",
		},
		{
			name: "float range label",
			rows: [][]string{
				{"1.5"}, {"2.5"}, {"3.5"},
			},
			colIdx:    0,
			wantEmpty: false,
			wantBars:  "*",
			wantLabel: "1.5–3.5",
		},
		{
			name: "all same value",
			rows: [][]string{
				{"42"}, {"42"}, {"42"},
			},
			colIdx:    0,
			wantEmpty: true,
		},
		{
			name: "single value",
			rows: [][]string{
				{"42"},
			},
			colIdx:    0,
			wantEmpty: true,
		},
		{
			name: "all NULL",
			rows: [][]string{
				{"NULL"}, {"NULL"}, {"NULL"},
			},
			colIdx:    0,
			wantEmpty: true,
		},
		{
			name: "NULL mixed with values",
			rows: [][]string{
				{"NULL"}, {"10"}, {"NULL"}, {"20"}, {"30"},
			},
			colIdx:    0,
			wantEmpty: false,
			wantBars:  "*",
		},
		{
			name: "negative values",
			rows: [][]string{
				{"-10"}, {"-5"}, {"0"}, {"5"}, {"10"},
			},
			colIdx:    0,
			wantEmpty: false,
			wantBars:  "*",
			wantLabel: "-10–10",
		},
		{
			name: "too many rows skipped",
			rows: func() [][]string {
				r := make([][]string, maxHistogramRows+1)
				for i := range maxHistogramRows + 1 {
					r[i] = []string{"1"}
				}
				return r
			}(),
			colIdx:      0,
			wantSkipped: true,
		},
		{
			name: "out of bounds colIdx",
			rows: [][]string{
				{"1", "2"},
			},
			colIdx:    5,
			wantEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeHistogram(tt.rows, tt.colIdx)

			if tt.wantSkipped {
				if !got.Skipped {
					t.Errorf("expected Skipped=true, got false")
				}
				return
			}

			if tt.wantEmpty {
				if got.Bars != "" || got.Label != "" || got.Skipped {
					t.Errorf("expected empty histogramData, got Bars=%q Label=%q Skipped=%v", got.Bars, got.Label, got.Skipped)
				}
				return
			}

			if tt.wantBars == "*" {
				if got.Bars == "" {
					t.Errorf("expected non-empty Bars, got empty")
				}
			} else if tt.wantBars != "" {
				if got.Bars != tt.wantBars {
					t.Errorf("Bars = %q, want %q", got.Bars, tt.wantBars)
				}
			}

			if tt.wantLabel != "" {
				if !strings.Contains(got.Label, tt.wantLabel) {
					t.Errorf("Label = %q, want to contain %q", got.Label, tt.wantLabel)
				}
			}
		})
	}
}

func TestComputeHistogram_MixedColumnSuppressed(t *testing.T) {
	// When more than half of non-NULL values are non-numeric, suppress histogram.
	rows := [][]string{
		{"42"}, {"N/A"}, {"100"}, {"N/A"}, {"N/A"}, {"N/A"},
	}
	got := computeHistogram(rows, 0)
	if got.Bars != "" || got.Label != "" {
		t.Errorf("expected empty histogram for mixed column, got Bars=%q Label=%q", got.Bars, got.Label)
	}
}

func TestComputeHistogram_NaNInf(t *testing.T) {
	// NaN and Inf should be skipped; the remaining valid values should produce a histogram
	rows := [][]string{
		{"NaN"}, {"Inf"}, {"-Inf"},
		{"1"}, {"2"}, {"3"},
	}
	got := computeHistogram(rows, 0)
	if got.Bars == "" {
		t.Error("expected histogram from valid values after skipping NaN/Inf, got empty")
	}
}

func TestFormatHistogramLabel(t *testing.T) {
	tests := []struct {
		min, max float64
		want     string
	}{
		{0, 100, "0–100"},
		{-50, 50, "-50–50"},
		{1.5, 3.5, "1.5–3.5"},
		{0.001, 0.999, "0.001–0.999"},
		{1000000, 9999999, "1000000–9999999"},
		// BIGINT UNSIGNED range — float64 can't represent MaxUint64 exactly; %.0f rounds up by 1
		{0, 18446744073709551615, "0–18446744073709551616"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatHistogramLabel(tt.min, tt.max)
			if got != tt.want {
				t.Errorf("formatHistogramLabel(%v, %v) = %q, want %q", tt.min, tt.max, got, tt.want)
			}
		})
	}
}
