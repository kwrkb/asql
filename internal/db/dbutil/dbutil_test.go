package dbutil

import (
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/kwrkb/asql/internal/db"
)

func TestStringifyValue(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		value any
		want  string
	}{
		{"nil", nil, "NULL"},
		{"byte slice", []byte("hello"), "hello"},
		{"time", fixedTime, "2024-01-15T12:00:00Z"},
		{"time with microseconds", time.Date(2024, 1, 15, 12, 0, 0, 123000, time.UTC), "2024-01-15T12:00:00.000123Z"},
		{"time with nanoseconds", time.Date(2024, 1, 15, 12, 0, 0, 999999999, time.UTC), "2024-01-15T12:00:00.999999999Z"},
		{"int", 42, "42"},
		{"int64", int64(100), "100"},
		{"float64", 3.14, "3.14"},
		{"string", "world", "world"},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"binary blob", []byte{0xDE, 0xAD, 0xBE, 0xEF}, "deadbeef"},
		{"empty byte slice", []byte{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StringifyValue(tt.value)
			if got != tt.want {
				t.Errorf("StringifyValue(%v) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestLeadingKeyword(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  string
	}{
		{"plain select", "SELECT * FROM t", "select"},
		{"leading whitespace", "  INSERT INTO t VALUES (1)", "insert"},
		{"line comment", "-- comment\nSELECT 1", "select"},
		{"block comment", "/* comment */ UPDATE t SET a=1", "update"},
		{"leading semicolon", ";; SELECT 1", "select"},
		{"empty string", "", ""},
		{"only comment", "-- nothing", ""},
		{"unclosed block comment", "/* unclosed SELECT 1", ""},
		{"mixed comments", "-- line\n/* block */\nDELETE FROM t", "delete"},
		{"uppercase", "PRAGMA table_info(t)", "pragma"},
		{"hash comment", "# comment\nSELECT 1", "select"},
		{"hash comment only", "# nothing", ""},
		{"hash then block comment", "# line\n/* block */\nDELETE FROM t", "delete"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LeadingKeyword(tt.query)
			if got != tt.want {
				t.Errorf("LeadingKeyword(%q) = %q, want %q", tt.query, got, tt.want)
			}
		})
	}
}

func TestShortenTypeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"INTEGER", "int"},
		{"integer", "int"},
		{"Integer", "int"},
		{"UNSIGNED BIGINT", "ubigint"},
		{"TIMESTAMPTZ", "tstz"},
		{"TEXT", "text"},
		{"VARCHAR", "varchar"},
		{"INT4", "int4"},
		{"BOOL", "bool"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ShortenTypeName(tt.input)
			if got != tt.want {
				t.Errorf("ShortenTypeName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestContainsReturning(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		dialect Dialect
		want    bool
	}{
		// Basic cases
		{"plain returning", "INSERT INTO t VALUES (1) RETURNING id", Dialect{}, true},
		{"no returning", "INSERT INTO t VALUES (1)", Dialect{}, false},
		{"returning in string", "INSERT INTO t VALUES ('RETURNING')", Dialect{}, false},
		{"returning in double quote", `INSERT INTO t VALUES (1) WHERE "RETURNING" = 1`, Dialect{}, false},
		{"returning in line comment", "INSERT INTO t -- RETURNING\nVALUES (1)", Dialect{}, false},
		{"returning in block comment", "INSERT INTO t /* RETURNING */ VALUES (1)", Dialect{}, false},
		{"case insensitive", "insert into t values (1) returning id", Dialect{}, true},
		{"partial match", "INSERT INTO returning_table VALUES (1)", Dialect{}, false},

		// SQLite dialect: bracket + backtick
		{"bracket quoted", "INSERT INTO [RETURNING] VALUES (1)", Dialect{BracketQuote: true}, false},
		{"backtick quoted", "INSERT INTO `RETURNING` VALUES (1)", Dialect{BacktickQuote: true}, false},
		{"bracket with real returning", "INSERT INTO [t] VALUES (1) RETURNING id", Dialect{BracketQuote: true}, true},

		// PostgreSQL dialect: dollar-quoted
		{"dollar quoted", "INSERT INTO t VALUES ($$RETURNING$$)", Dialect{DollarQuote: true}, false},
		{"named dollar tag", "INSERT INTO t VALUES ($fn$RETURNING$fn$)", Dialect{DollarQuote: true}, false},
		{"dollar with real returning", "INSERT INTO t VALUES ($$hello$$) RETURNING id", Dialect{DollarQuote: true}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ContainsReturning(tt.query, tt.dialect)
			if got != tt.want {
				t.Errorf("ContainsReturning(%q, %+v) = %v, want %v", tt.query, tt.dialect, got, tt.want)
			}
		})
	}
}

func TestParseDollarTag(t *testing.T) {
	tests := []struct {
		name  string
		query string
		pos   int
		want  string
	}{
		{"empty tag", "$$hello$$", 0, "$$"},
		{"named tag", "$fn$hello$fn$", 0, "$fn$"},
		{"not dollar", "hello", 0, ""},
		{"unclosed", "$fn", 0, ""},
		{"invalid char", "$a b$", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDollarTag(tt.query, tt.pos)
			if got != tt.want {
				t.Errorf("parseDollarTag(%q, %d) = %q, want %q", tt.query, tt.pos, got, tt.want)
			}
		})
	}
}

func TestCteBodyKeyword(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  string
	}{
		{"cte select", "WITH cte AS (SELECT 1) SELECT * FROM cte", "select"},
		{"cte insert", "WITH cte AS (SELECT 1) INSERT INTO t SELECT * FROM cte", "insert"},
		{"cte update", "WITH cte AS (SELECT 1) UPDATE t SET a=1", "update"},
		{"cte delete", "WITH cte AS (SELECT 1) DELETE FROM t", "delete"},
		{"recursive cte", "WITH RECURSIVE cte AS (SELECT 1) SELECT * FROM cte", "select"},
		{"multiple ctes", "WITH a AS (SELECT 1), b AS (SELECT 2) SELECT * FROM a, b", "select"},
		{"nested parens", "WITH cte AS (SELECT (1+2) FROM t) DELETE FROM t2", "delete"},
		{"string in cte", "WITH cte AS (SELECT 'hello)world' FROM t) SELECT * FROM cte", "select"},
		{"comment in cte", "WITH cte AS (/* ) */ SELECT 1) SELECT * FROM cte", "select"},
		{"cte values", "WITH cte AS (SELECT 1) VALUES (1, 2)", "values"},
		{"cte table", "WITH cte AS (SELECT 1) TABLE users", "table"},
		{"cte explain", "WITH cte AS (SELECT 1) EXPLAIN SELECT 1", "explain"},
		{"empty", "", ""},
		{"only with", "WITH", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CteBodyKeyword(tt.query)
			if got != tt.want {
				t.Errorf("CteBodyKeyword(%q) = %q, want %q", tt.query, got, tt.want)
			}
		})
	}
}

func TestStringifyValueKind(t *testing.T) {
	ts := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		value    any
		wantStr  string
		wantKind db.Kind
	}{
		{"nil", nil, "NULL", db.KindNull},
		{"int64", int64(42), "42", db.KindInt},
		{"negative int64", int64(-7), "-7", db.KindInt},
		{"float64", float64(2.5), "2.5", db.KindFloat},
		{"float64 exponent", float64(1e21), "1e+21", db.KindFloat},
		{"string", "hello", "hello", db.KindText},
		{"literal NULL text", "NULL", "NULL", db.KindText},
		{"bool", true, "true", db.KindText},
		{"time", ts, "2026-08-11T12:00:00Z", db.KindText},
		{"utf8 bytes are text", []byte("héllo"), "héllo", db.KindText},
		{"binary bytes are blob", []byte{0xff, 0x00, 0xfe}, "ff00fe", db.KindBlob},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStr, gotKind := StringifyValueKind(tt.value)
			if gotStr != tt.wantStr {
				t.Errorf("string = %q, want %q", gotStr, tt.wantStr)
			}
			if gotKind != tt.wantKind {
				t.Errorf("kind = %d, want %d", gotKind, tt.wantKind)
			}
			// StringifyValue must keep returning exactly the same string.
			if legacy := StringifyValue(tt.value); legacy != tt.wantStr {
				t.Errorf("StringifyValue = %q, want %q", legacy, tt.wantStr)
			}
		})
	}
}

func TestStringifyValueKind_NumericStringsRoundTrip(t *testing.T) {
	// KindInt/KindFloat promise that the display string parses back exactly;
	// bring relies on that to rebuild typed values.
	for _, v := range []int64{0, 1, -1, math.MaxInt64, math.MinInt64} {
		s, k := StringifyValueKind(v)
		if k != db.KindInt {
			t.Fatalf("%d: kind = %d, want KindInt", v, k)
		}
		got, err := strconv.ParseInt(s, 10, 64)
		if err != nil || got != v {
			t.Errorf("%d: round-trip via %q gave (%d, %v)", v, s, got, err)
		}
	}
	for _, v := range []float64{0, 0.1, -2.5, 1e-9, math.MaxFloat64, math.SmallestNonzeroFloat64} {
		s, k := StringifyValueKind(v)
		if k != db.KindFloat {
			t.Fatalf("%v: kind = %d, want KindFloat", v, k)
		}
		got, err := strconv.ParseFloat(s, 64)
		if err != nil || got != v {
			t.Errorf("%v: round-trip via %q gave (%v, %v)", v, s, got, err)
		}
	}
}

func TestBinaryColumnType(t *testing.T) {
	binary := []string{"BLOB", "blob", "TINYBLOB", "MEDIUMBLOB", "LONGBLOB", "BINARY", "VARBINARY", "BYTEA", " bytea "}
	for _, name := range binary {
		if !BinaryColumnType(name) {
			t.Errorf("BinaryColumnType(%q) = false, want true", name)
		}
	}
	// TEXT is what go-sql-driver reports for a MySQL BLOB-family column with a
	// non-binary charset, so it must not be treated as binary.
	text := []string{"TEXT", "VARCHAR", "CHAR", "TINYTEXT", "LONGTEXT", "INTEGER", "JSON", "", "BINARY_FLOAT"}
	for _, name := range text {
		if BinaryColumnType(name) {
			t.Errorf("BinaryColumnType(%q) = true, want false", name)
		}
	}
}

func TestNumericColumnType(t *testing.T) {
	for _, name := range []string{"DECIMAL", "decimal", "NUMERIC", " numeric "} {
		if !NumericColumnType(name) {
			t.Errorf("NumericColumnType(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"INTEGER", "BIGINT", "DOUBLE", "TEXT", "VARCHAR", "", "DECIMAL_X"} {
		if NumericColumnType(name) {
			t.Errorf("NumericColumnType(%q) = true, want false", name)
		}
	}
}

func TestNumericKindOf(t *testing.T) {
	tests := []struct {
		in       string
		wantKind db.Kind
		wantOK   bool
	}{
		{"2", db.KindInt, true},
		{"10", db.KindInt, true},
		{"-7", db.KindInt, true},
		{"2.50", db.KindFloat, true},
		{"0.00000123", db.KindFloat, true},
		{"-1.25", db.KindFloat, true},
		// 19 significant digits: a float64 would round it, so it stays text.
		{"12345678901234567.89", db.KindText, false},
		// Exactly at the limit, and one past it.
		{"1.23456789012345", db.KindFloat, true},
		{"1.234567890123456", db.KindText, false},
		// Not numbers at all.
		{"abc", db.KindText, false},
		{"", db.KindText, false},
		{"NaN", db.KindText, false},
		{"Inf", db.KindText, false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			gotKind, gotOK := numericKindOf(tt.in)
			if gotKind != tt.wantKind || gotOK != tt.wantOK {
				t.Errorf("numericKindOf(%q) = (%d, %v), want (%d, %v)",
					tt.in, gotKind, gotOK, tt.wantKind, tt.wantOK)
			}
		})
	}
}

func TestSignificantDigits(t *testing.T) {
	tests := map[string]int{
		"2":                    1,
		"2.50":                 3, // trailing zeros counted: overstating is safe
		"0.00000123":           3,
		"-1.25":                3,
		"12345678901234567.89": 19,
		"1e10":                 1, // the exponent is not precision
		"0":                    0,
	}
	for in, want := range tests {
		if got := significantDigits(in); got != want {
			t.Errorf("significantDigits(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestClassifyValue(t *testing.T) {
	// The interactions between the three steps are what this covers: each step
	// must see the previous one's result, and the empty-value sentinel must not
	// undo either override.
	tests := []struct {
		name     string
		value    any
		hints    columnHints
		wantStr  string
		wantKind db.Kind
	}{
		{"plain text", "abc", columnHints{}, "abc", db.KindText},
		{"int", int64(7), columnHints{}, "7", db.KindInt},
		{"null", nil, columnHints{}, "NULL", db.KindNull},
		{"null in a numeric column", nil, columnHints{numeric: true}, "NULL", db.KindNull},
		{"null in a binary column", nil, columnHints{binary: true}, "NULL", db.KindNull},

		// Numbers a driver returned as text (MySQL DECIMAL).
		{"decimal integer", []byte("10"), columnHints{numeric: true}, "10", db.KindInt},
		{"decimal fraction", []byte("2.50"), columnHints{numeric: true}, "2.50", db.KindFloat},
		{"decimal beyond float64", []byte("12345678901234567.89"), columnHints{numeric: true},
			"12345678901234567.89", db.KindText},
		{"non-numeric in a numeric column", []byte("n/a"), columnHints{numeric: true}, "n/a", db.KindText},
		{"numeric hint off", []byte("10"), columnHints{}, "10", db.KindText},

		// Binary by declared column type.
		{"utf8 bytes in a binary column", []byte("abc"), columnHints{binary: true}, "616263", db.KindBlob},
		{"utf8 bytes without the hint", []byte("abc"), columnHints{}, "abc", db.KindText},
		{"non-utf8 bytes need no hint", []byte{0xff, 0xfe}, columnHints{}, "fffe", db.KindBlob},

		// Binary by Go type (SQLite).
		{"bytes are binary", []byte("abc"), columnHints{bytesAreBinary: true}, "616263", db.KindBlob},
		{"string is not binary", "abc", columnHints{bytesAreBinary: true}, "abc", db.KindText},

		// The blob override wins over the numeric one. SQLite is dynamically
		// typed, so a DECIMAL column can hold a blob whose bytes spell a
		// number; promoting it to KindInt would rewrite it as that number.
		{"numeric-looking blob in a numeric column", []byte("123"),
			columnHints{numeric: true, bytesAreBinary: true}, "313233", db.KindBlob},
		{"numeric-looking value in a binary numeric column", []byte("123"),
			columnHints{numeric: true, binary: true}, "313233", db.KindBlob},
		// ...but a driver that returns []byte for text (MySQL) must keep its
		// DECIMAL promotion, since bytesAreBinary is off there.
		{"decimal bytes stay numeric without the binary hint", []byte("123"),
			columnHints{numeric: true}, "123", db.KindInt},

		// The empty-value sentinel, and what it must not overwrite.
		{"empty string", "", columnHints{}, `""`, db.KindEmpty},
		{"empty bytes", []byte{}, columnHints{}, `""`, db.KindEmpty},
		{"zero-length blob by type", []byte{}, columnHints{binary: true}, `""`, db.KindBlob},
		{"zero-length blob by Go type", []byte{}, columnHints{bytesAreBinary: true}, `""`, db.KindBlob},
		{"empty in a numeric column", []byte{}, columnHints{numeric: true}, `""`, db.KindEmpty},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStr, gotKind := classifyValue(tt.value, tt.hints)
			if gotStr != tt.wantStr {
				t.Errorf("string = %q, want %q", gotStr, tt.wantStr)
			}
			if gotKind != tt.wantKind {
				t.Errorf("kind = %d, want %d", gotKind, tt.wantKind)
			}
		})
	}
}
