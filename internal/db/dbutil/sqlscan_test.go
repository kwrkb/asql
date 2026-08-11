package dbutil

import (
	"strings"
	"testing"
)

func TestUnlexableReason(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		unlexable  bool
		mentioning string
	}{
		{"plain select", "SELECT * FROM t", false, ""},
		{"doubled single quote", "SELECT 'it''s' FROM t", false, ""},
		{"doubled double quote", `SELECT "a""b" FROM t`, false, ""},
		{"doubled backtick", "SELECT `a``b` FROM t", false, ""},
		{"bracket identifier", "SELECT [a b] FROM t", false, ""},
		{"line comment with space", "SELECT 1 -- note\n", false, ""},
		{"line comment at end of input", "SELECT 1 --", false, ""},
		{"block comment", "SELECT 1 /* note */", false, ""},
		{"backslash not before a quote", `SELECT 'a\nb' FROM t`, false, ""},

		// The constructs dialects disagree about.
		{"escaped single quote", `SELECT 'it\'s' FROM t`, true, "backslash-escaped quote"},
		{"escaped double quote", `SELECT "a\"b" FROM t`, true, "backslash-escaped quote"},
		{"escaped backslash", `SELECT 'a\\' FROM t`, true, "backslash-escaped quote"},
		{"postgres escape string", `SELECT E'it\'s' FROM t`, true, "backslash-escaped quote"},
		{"hash", "SELECT 1 # 2", true, "#"},
		{"double dash without space", "SELECT 1--1", true, "no space after the dashes"},
		{"executable comment", "SELECT 1 /*! DELETE FROM t */", true, "executable comment"},
		{"mariadb executable comment", "SELECT 1 /*M! DELETE FROM t */", true, "executable comment"},
		{"versioned executable comment", "SELECT 1 /*!50110 DELETE FROM t */", true, "executable comment"},
		{"unterminated string", "SELECT 'abc FROM t", true, "unterminated"},
		{"unterminated block comment", "SELECT 1 /* note", true, "unterminated"},

		// Inside a quoted run, none of them are constructs.
		{"hash inside a literal", "SELECT '#' FROM t", false, ""},
		{"executable comment inside a literal", "SELECT '/*! x */' FROM t", false, ""},
		{"dashes inside a literal", "SELECT '--1' FROM t", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason := UnlexableReason(tt.query)
			if tt.unlexable && reason == "" {
				t.Fatalf("UnlexableReason(%q) = \"\", want a reason", tt.query)
			}
			if !tt.unlexable && reason != "" {
				t.Fatalf("UnlexableReason(%q) = %q, want it readable", tt.query, reason)
			}
			if tt.mentioning != "" && !strings.Contains(reason, tt.mentioning) {
				t.Errorf("UnlexableReason(%q) = %q, want it to mention %q", tt.query, reason, tt.mentioning)
			}
		})
	}
}

func TestHasMultipleStatements(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  bool
	}{
		{"single", "SELECT 1", false},
		{"trailing semicolon", "SELECT 1;", false},
		{"trailing semicolon and whitespace", "SELECT 1;  \n", false},
		{"trailing semicolon and comment", "SELECT 1; -- done", false},
		{"two statements", "SELECT 1; DELETE FROM t", true},
		{"empty statement", "SELECT 1;;", true},
		{"semicolon in string", "SELECT ';'", false},
		{"semicolon in line comment", "SELECT 1 -- ; DELETE\n", false},
		{"semicolon in block comment", "SELECT 1 /* ; DELETE */", false},
		{"semicolon in quoted identifier", `SELECT "a;b" FROM t`, false},
		{"semicolon in backtick identifier", "SELECT `a;b` FROM t", false},
		{"semicolon in bracket identifier", "SELECT [a;b] FROM t", false},
		{"empty query", "", false},
		// The dialect-specific comment forms are not comments here, so the
		// separator they would have hidden stays visible.
		{"semicolon after a bare double dash", "SELECT 1--1; DELETE FROM t", true},
		{"semicolon after a hash", "SELECT 1 # 2; DELETE FROM t", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasMultipleStatements(tt.query); got != tt.want {
				t.Errorf("HasMultipleStatements(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

func TestCteTermKeywords(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  []string
		ok    bool
	}{
		{"no cte", "SELECT 1", nil, true},
		{"single term", "WITH a AS (SELECT 1) SELECT * FROM a", []string{"select"}, true},
		{"two terms", "WITH a AS (SELECT 1), b AS (SELECT 2) SELECT * FROM a", []string{"select", "select"}, true},
		{
			"data-modifying term",
			"WITH gone AS (DELETE FROM t RETURNING *) SELECT * FROM gone",
			[]string{"delete"},
			true,
		},
		{
			"second term modifies",
			"WITH a AS (SELECT 1), b AS (UPDATE t SET x = 1 RETURNING *) SELECT * FROM a",
			[]string{"select", "update"},
			true,
		},
		{"materialized", "WITH a AS MATERIALIZED (SELECT 1) SELECT * FROM a", []string{"select"}, true},
		{"not materialized", "WITH a AS NOT MATERIALIZED (SELECT 1) SELECT * FROM a", []string{"select"}, true},
		{"column list is not a term", "WITH a (x, y) AS (SELECT 1, 2) SELECT * FROM a", []string{"select"}, true},
		// The outer term opens with WITH, not with its eventual body: the
		// caller has to treat a nested WITH as unclassifiable rather than
		// guess which of the inner keywords governs.
		{"nested with", "WITH a AS (WITH b AS (DELETE FROM t RETURNING *) SELECT * FROM b) SELECT * FROM a",
			[]string{"with", "delete"}, true},
		{"parenthesized term", "WITH a AS ((SELECT 1)) SELECT * FROM a", []string{"select"}, true},
		{"comment before term keyword", "WITH a AS (/* c */ SELECT 1) SELECT * FROM a", []string{"select"}, true},
		{"as in cast is not a term", "SELECT CAST(x AS INTEGER) FROM t", nil, true},
		{"as alias is not a term", "SELECT a AS b FROM t", nil, true},
		{"derived table is not a term", "SELECT * FROM (SELECT 1) AS s", nil, true},
		{"unreadable term", "WITH a AS (,) SELECT 1", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := CteTermKeywords(tt.query)
			if ok != tt.ok {
				t.Fatalf("CteTermKeywords(%q) ok = %v, want %v", tt.query, ok, tt.ok)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("CteTermKeywords(%q) = %v, want %v", tt.query, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("CteTermKeywords(%q) = %v, want %v", tt.query, got, tt.want)
				}
			}
		})
	}
}

func TestStripExplain(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  string
		ok    bool
	}{
		{"not explain", "SELECT 1", "", false},
		{"plain", "EXPLAIN SELECT 1", "SELECT 1", true},
		{"analyze", "EXPLAIN ANALYZE SELECT 1", "SELECT 1", true},
		{"analyze delete", "EXPLAIN ANALYZE DELETE FROM t", "DELETE FROM t", true},
		{"option list", "EXPLAIN (ANALYZE, BUFFERS) DELETE FROM t", "DELETE FROM t", true},
		{"format option", "EXPLAIN (FORMAT JSON) SELECT 1", "SELECT 1", true},
		{"mysql format assignment", "EXPLAIN FORMAT=JSON SELECT 1", "SELECT 1", true},
		{"sqlite query plan", "EXPLAIN QUERY PLAN SELECT 1", "SELECT 1", true},
		{"nested explain", "EXPLAIN EXPLAIN SELECT 1", "EXPLAIN SELECT 1", true},
		{"case insensitive", "explain analyze delete from t", "delete from t", true},
		{"comment between", "EXPLAIN /* c */ SELECT 1", "SELECT 1", true},
		{"nothing to explain", "EXPLAIN", "", false},
		{"only options", "EXPLAIN ANALYZE", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := StripExplain(tt.query)
			if ok != tt.ok {
				t.Fatalf("StripExplain(%q) ok = %v, want %v", tt.query, ok, tt.ok)
			}
			if got != tt.want {
				t.Errorf("StripExplain(%q) = %q, want %q", tt.query, got, tt.want)
			}
		})
	}
}

func TestPragmaName(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  string
		ok    bool
	}{
		{"not pragma", "SELECT 1", "", false},
		{"bare", "PRAGMA table_list", "table_list", true},
		{"with argument", "PRAGMA table_info(users)", "table_info", true},
		{"assignment form", "PRAGMA query_only=0", "query_only", true},
		{"function form", "PRAGMA query_only(0)", "query_only", true},
		{"schema prefix", "PRAGMA main.table_info(users)", "table_info", true},
		{"case insensitive", "pragma Table_Info(t)", "table_info", true},
		{"quoted name is unreadable", `PRAGMA "query_only"(0)`, "", false},
		{"no name", "PRAGMA", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := PragmaName(tt.query)
			if ok != tt.ok {
				t.Fatalf("PragmaName(%q) ok = %v, want %v", tt.query, ok, tt.ok)
			}
			if got != tt.want {
				t.Errorf("PragmaName(%q) = %q, want %q", tt.query, got, tt.want)
			}
		})
	}
}

func TestContainsKeyword(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		keyword string
		want    bool
	}{
		{"present", "SELECT * INTO backup FROM t", "into", true},
		{"absent", "SELECT * FROM t", "into", false},
		{"case insensitive", "select * into backup from t", "into", true},
		{"inside string literal", "SELECT 'INTO' FROM t", "into", false},
		{"inside quoted identifier", `SELECT "into" FROM t`, "into", false},
		{"inside backtick identifier", "SELECT `into` FROM t", "into", false},
		{"inside bracket identifier", "SELECT [into] FROM t", "into", false},
		{"inside line comment", "SELECT 1 -- into backup", "into", false},
		{"inside block comment", "SELECT 1 /* into backup */", "into", false},
		{"prefix of a longer identifier", "SELECT * FROM into_log", "into", false},
		{"suffix of a longer identifier", "SELECT * FROM log_into", "into", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ContainsKeyword(tt.query, tt.keyword); got != tt.want {
				t.Errorf("ContainsKeyword(%q, %q) = %v, want %v", tt.query, tt.keyword, got, tt.want)
			}
		})
	}
}
