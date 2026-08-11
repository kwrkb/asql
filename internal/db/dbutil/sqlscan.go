package dbutil

import "strings"

// DialectFor returns the quoting styles recognized by the given database type
// (as reported by DBAdapter.Type). Unknown types get the conservative default:
// standard single-quoted strings and double-quoted identifiers only.
func DialectFor(dbType string) Dialect {
	switch strings.ToLower(dbType) {
	case "sqlite":
		return Dialect{BracketQuote: true, BacktickQuote: true}
	case "mysql":
		return Dialect{BacktickQuote: true}
	case "postgres", "postgresql":
		return Dialect{DollarQuote: true}
	default:
		return Dialect{}
	}
}

// sqlScanner walks a SQL string one significant token at a time, skipping
// comments, string literals and quoted identifiers. It backs the shape-level
// questions callers ask about a statement: how many statements are there, what
// does EXPLAIN explain, which keyword opens each CTE term. It answers nothing
// about policy — that belongs to the caller.
type sqlScanner struct {
	q string
	d Dialect
	i int
}

func (s *sqlScanner) eof() bool { return s.i >= len(s.q) }

// skipSpace advances past whitespace and comments.
func (s *sqlScanner) skipSpace() { s.i = skipWhitespaceAndComments(s.q, s.i) }

// skipLeadingSemicolons mirrors LeadingKeyword, which treats a statement that
// begins with separators as starting at the first real token.
func (s *sqlScanner) skipLeadingSemicolons() {
	s.skipSpace()
	for !s.eof() && s.q[s.i] == ';' {
		s.i++
		s.skipSpace()
	}
}

// skipQuoted advances past a string literal or quoted identifier when the
// scanner is positioned at one, and reports whether it advanced.
func (s *sqlScanner) skipQuoted() bool {
	if s.eof() {
		return false
	}
	switch c := s.q[s.i]; {
	case c == '\'':
		s.i = skipSingleQuoted(s.q, s.i)
		return true
	case c == '"':
		s.i = skipDoubleQuoted(s.q, s.i)
		return true
	case s.d.BacktickQuote && c == '`':
		s.i = skipBacktickQuoted(s.q, s.i)
		return true
	case s.d.BracketQuote && c == '[':
		s.i++
		for !s.eof() && s.q[s.i] != ']' {
			s.i++
		}
		if !s.eof() {
			s.i++
		}
		return true
	case s.d.DollarQuote && c == '$' && s.i+1 < len(s.q):
		s.i = skipDollarQuoted(s.q, s.i)
		return true
	}
	return false
}

// word reads the identifier at the current position and returns it lowercased.
// It returns "" (without advancing) when the scanner is not on an identifier.
func (s *sqlScanner) word() string {
	start := s.i
	for !s.eof() && isIdentCharByte(s.q[s.i]) {
		s.i++
	}
	if s.i == start {
		return ""
	}
	return strings.ToLower(s.q[start:s.i])
}

// HasMultipleStatements reports whether query carries a statement separator
// other than a single trailing semicolon. Semicolons inside string literals,
// quoted identifiers and comments do not count.
//
// LeadingKeyword only inspects the first statement, so "SELECT 1; DELETE FROM t"
// looks like a SELECT to it. Callers that classify a statement by its leading
// keyword must reject multi-statement input first. Whether the driver would
// actually execute the tail is dialect- and DSN-dependent (MySQL needs
// multiStatements, PostgreSQL allows it over the simple protocol), so this is
// deliberately answered without consulting the dialect.
func HasMultipleStatements(query string, d Dialect) bool {
	s := &sqlScanner{q: query, d: d}
	seenSemicolon := false
	for {
		s.skipSpace()
		if s.eof() {
			return false
		}
		if s.q[s.i] == ';' {
			if seenSemicolon {
				return true
			}
			seenSemicolon = true
			s.i++
			continue
		}
		if seenSemicolon {
			return true
		}
		if s.skipQuoted() {
			continue
		}
		if w := s.word(); w != "" {
			continue
		}
		s.i++
	}
}

// CteTermKeywords returns the leading keyword of every `AS (...)` group in
// query, lowercased, in the order they appear. The second return value is
// false when a group could not be read, in which case the caller must treat
// the statement as unclassifiable rather than assume there were no terms.
//
// CteBodyKeyword answers what a WITH statement's body does, which is not
// enough on its own: PostgreSQL evaluates data-modifying CTEs as part of the
// same execution, so `WITH gone AS (DELETE FROM t RETURNING *) SELECT * FROM gone`
// has a body keyword of "select" while deleting rows. Every `AS (` group is
// inspected — including nested ones — because a CTE body may itself be a WITH.
func CteTermKeywords(query string, d Dialect) ([]string, bool) {
	s := &sqlScanner{q: query, d: d}
	var terms []string
	last := ""
	for {
		s.skipSpace()
		if s.eof() {
			return terms, true
		}
		if s.q[s.i] == '(' && (last == "as" || last == "materialized") {
			s.i++
			s.skipSpace()
			// A term may be parenthesized again: AS ((SELECT 1)).
			for !s.eof() && s.q[s.i] == '(' {
				s.i++
				s.skipSpace()
			}
			kw := s.word()
			if kw == "" {
				return nil, false
			}
			terms = append(terms, kw)
			last = ""
			continue
		}
		if s.skipQuoted() {
			last = ""
			continue
		}
		if w := s.word(); w != "" {
			last = w
			continue
		}
		s.i++
		last = ""
	}
}

// explainOptionWords are the words that may appear between EXPLAIN and the
// statement it explains, plus the values those words take. Anything outside
// this set is the first keyword of the explained statement.
var explainOptionWords = map[string]bool{
	"analyze": true, "analyse": true, "verbose": true, "costs": true,
	"settings": true, "generic_plan": true, "buffers": true, "wal": true,
	"timing": true, "summary": true, "format": true, "memory": true,
	"query": true, "plan": true, // SQLite: EXPLAIN QUERY PLAN
	"on": true, "off": true, "true": true, "false": true,
	"text": true, "xml": true, "json": true, "yaml": true, "traditional": true, "tree": true,
}

// StripExplain returns the statement that an EXPLAIN applies to. It reports
// false when query does not start with EXPLAIN or when the target could not be
// isolated.
//
// This matters because PostgreSQL's EXPLAIN ANALYZE *executes* its target:
// `EXPLAIN ANALYZE DELETE FROM t` deletes rows while presenting a leading
// keyword of "explain". Returning the inner statement lets the caller apply
// the same classification recursively instead of special-casing ANALYZE.
func StripExplain(query string, d Dialect) (string, bool) {
	s := &sqlScanner{q: query, d: d}
	s.skipLeadingSemicolons()
	if w := s.word(); w != "explain" {
		return "", false
	}
	for {
		s.skipSpace()
		if s.eof() {
			return "", false
		}
		if s.q[s.i] == '(' {
			if !s.skipBalancedParens() {
				return "", false
			}
			continue
		}
		start := s.i
		w := s.word()
		if w == "" || !explainOptionWords[w] {
			if w == "" {
				return "", false
			}
			return query[start:], true
		}
		// MySQL spells options as FORMAT=JSON; consume the assigned value.
		s.skipSpace()
		if !s.eof() && s.q[s.i] == '=' {
			s.i++
			s.skipSpace()
			if s.word() == "" {
				return "", false
			}
		}
	}
}

// skipBalancedParens advances past a parenthesized group starting at the
// current position, and reports whether the group was closed.
func (s *sqlScanner) skipBalancedParens() bool {
	depth := 0
	for {
		s.skipSpace()
		if s.eof() {
			return false
		}
		if s.skipQuoted() {
			continue
		}
		switch s.q[s.i] {
		case '(':
			depth++
			s.i++
		case ')':
			depth--
			s.i++
			if depth == 0 {
				return true
			}
		default:
			s.i++
		}
	}
}

// PragmaName returns the name of a PRAGMA statement, lowercased and without
// any schema prefix (`PRAGMA main.table_info(t)` yields "table_info"). It
// reports false when query is not a PRAGMA or the name could not be read —
// including when the name is quoted, which callers should treat as unknown
// rather than resolve.
func PragmaName(query string, d Dialect) (string, bool) {
	s := &sqlScanner{q: query, d: d}
	s.skipLeadingSemicolons()
	if w := s.word(); w != "pragma" {
		return "", false
	}
	s.skipSpace()
	name := s.word()
	if name == "" {
		return "", false
	}
	s.skipSpace()
	if !s.eof() && s.q[s.i] == '.' {
		s.i++
		s.skipSpace()
		name = s.word()
		if name == "" {
			return "", false
		}
	}
	return name, true
}
