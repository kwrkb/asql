package dbutil

import "testing"

func TestHasMultipleStatements(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		dialect Dialect
		want    bool
	}{
		{"single", "SELECT 1", Dialect{}, false},
		{"trailing semicolon", "SELECT 1;", Dialect{}, false},
		{"trailing semicolon and whitespace", "SELECT 1;  \n", Dialect{}, false},
		{"trailing semicolon and comment", "SELECT 1; -- done", Dialect{}, false},
		{"two statements", "SELECT 1; DELETE FROM t", Dialect{}, true},
		{"empty statement", "SELECT 1;;", Dialect{}, true},
		{"semicolon in string", "SELECT ';'", Dialect{}, false},
		{"semicolon in line comment", "SELECT 1 -- ; DELETE\n", Dialect{}, false},
		{"semicolon in block comment", "SELECT 1 /* ; DELETE */", Dialect{}, false},
		{"semicolon in quoted identifier", `SELECT "a;b" FROM t`, Dialect{}, false},
		{"semicolon in backtick identifier", "SELECT `a;b` FROM t", Dialect{BacktickQuote: true}, false},
		{"semicolon in bracket identifier", "SELECT [a;b] FROM t", Dialect{BracketQuote: true}, false},
		{"semicolon in dollar quote", "SELECT $$a;b$$", Dialect{DollarQuote: true}, false},
		{"empty query", "", Dialect{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasMultipleStatements(tt.query, tt.dialect); got != tt.want {
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
			got, ok := CteTermKeywords(tt.query, Dialect{})
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
			got, ok := StripExplain(tt.query, Dialect{})
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
			got, ok := PragmaName(tt.query, Dialect{})
			if ok != tt.ok {
				t.Fatalf("PragmaName(%q) ok = %v, want %v", tt.query, ok, tt.ok)
			}
			if got != tt.want {
				t.Errorf("PragmaName(%q) = %q, want %q", tt.query, got, tt.want)
			}
		})
	}
}

func TestDialectFor(t *testing.T) {
	if d := DialectFor("sqlite"); !d.BracketQuote || !d.BacktickQuote || d.DollarQuote {
		t.Errorf("sqlite dialect = %+v", d)
	}
	if d := DialectFor("mysql"); !d.BacktickQuote || d.BracketQuote || d.DollarQuote {
		t.Errorf("mysql dialect = %+v", d)
	}
	if d := DialectFor("postgres"); !d.DollarQuote || d.BacktickQuote || d.BracketQuote {
		t.Errorf("postgres dialect = %+v", d)
	}
	if d := DialectFor("something-else"); d.DollarQuote || d.BacktickQuote || d.BracketQuote {
		t.Errorf("unknown dialect = %+v, want the conservative default", d)
	}
}

func TestContainsKeyword(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		keyword string
		dialect Dialect
		want    bool
	}{
		{"present", "SELECT * INTO backup FROM t", "into", Dialect{}, true},
		{"absent", "SELECT * FROM t", "into", Dialect{}, false},
		{"case insensitive", "select * into backup from t", "into", Dialect{}, true},
		{"inside string literal", "SELECT 'INTO' FROM t", "into", Dialect{}, false},
		{"inside quoted identifier", `SELECT "into" FROM t`, "into", Dialect{}, false},
		{"inside backtick identifier", "SELECT `into` FROM t", "into", Dialect{BacktickQuote: true}, false},
		{"inside bracket identifier", "SELECT [into] FROM t", "into", Dialect{BracketQuote: true}, false},
		{"inside dollar quote", "SELECT $$into$$", "into", Dialect{DollarQuote: true}, false},
		{"inside line comment", "SELECT 1 -- into backup", "into", Dialect{}, false},
		{"inside block comment", "SELECT 1 /* into backup */", "into", Dialect{}, false},
		{"prefix of a longer identifier", "SELECT * FROM into_log", "into", Dialect{}, false},
		{"suffix of a longer identifier", "SELECT * FROM log_into", "into", Dialect{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ContainsKeyword(tt.query, tt.keyword, tt.dialect); got != tt.want {
				t.Errorf("ContainsKeyword(%q, %q) = %v, want %v", tt.query, tt.keyword, got, tt.want)
			}
		})
	}
}

func TestHasAmbiguousStringEscape(t *testing.T) {
	mysql := DialectFor("mysql")
	postgres := DialectFor("postgres")
	sqlite := DialectFor("sqlite")

	tests := []struct {
		name    string
		query   string
		dialect Dialect
		want    bool
	}{
		{"plain literal", `SELECT 'abc' FROM t`, mysql, false},
		{"doubled quote", `SELECT 'it''s' FROM t`, mysql, false},
		{"escaped quote on mysql", `SELECT 'it\'s' FROM t`, mysql, true},
		{"escaped backslash on mysql", `SELECT 'a\\' FROM t`, mysql, true},
		{"escaped quote on postgres", `SELECT 'it\'s' FROM t`, postgres, true},
		{"escape string on postgres", `SELECT E'it\'s' FROM t`, postgres, false},
		{"escape string lowercase", `SELECT e'it\'s' FROM t`, postgres, false},
		{"e is part of an identifier", `SELECT tale'it\'s' FROM t`, postgres, true},
		{"backslash without a quote", `SELECT 'a\nb' FROM t`, mysql, false},
		{"sqlite has no escapes", `SELECT 'it\'s' FROM t`, sqlite, false},
		{"backslash inside a quoted identifier", "SELECT `a\\'b` FROM t", mysql, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasAmbiguousStringEscape(tt.query, tt.dialect); got != tt.want {
				t.Errorf("HasAmbiguousStringEscape(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

func TestHasMultipleStatementsHashRule(t *testing.T) {
	const query = "SELECT 1 # 2; DELETE FROM t"
	if !HasMultipleStatements(query, DialectFor("postgres")) {
		t.Error("PostgreSQL: # is the bitwise-XOR operator, so the ; still separates statements")
	}
	if HasMultipleStatements("SELECT 1 # 2; DELETE FROM t", DialectFor("mysql")) {
		t.Error("MySQL: # starts a comment, so there is only one statement")
	}
	if !HasMultipleStatements("SELECT 1 # c\n; DELETE FROM t", DialectFor("mysql")) {
		t.Error("MySQL: a # comment ends at the newline and must not hide the separator")
	}
}
