package dbutil

import "strings"

// DialectFor returns the quoting styles recognized by the given database type
// (as reported by DBAdapter.Type). Unknown types get the conservative default:
// standard single-quoted strings and double-quoted identifiers only.
func DialectFor(dbType string) Dialect {
	switch strings.ToLower(dbType) {
	case "sqlite":
		// SQLite has no # comment and no backslash escapes: a backslash inside
		// a string literal is just a backslash.
		return Dialect{BracketQuote: true, BacktickQuote: true}
	case "mysql":
		return Dialect{
			BacktickQuote:          true,
			HashComment:            true,
			DoubleDashNeedsSpace:   true,
			ExecutableComment:      true,
			DoubleQuoteMayBeString: true,
			BackslashEscape:        true,
		}
	case "postgres", "postgresql":
		// # is the bitwise-XOR operator here, not a comment. Backslash escapes
		// apply inside E'...' always, and inside plain literals when
		// standard_conforming_strings is off.
		return Dialect{DollarQuote: true, BackslashEscape: true}
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

// skipSpace advances past whitespace and comments, recognizing # as a comment
// only where the dialect says it is one.
func (s *sqlScanner) skipSpace() { s.i = skipSpaceDialect(s.q, s.i, s.d) }

// skipSpaceDialect is skipWhitespaceAndComments with the # rule made
// dialect-dependent. Treating # as a comment on PostgreSQL hides everything
// after the bitwise-XOR operator — `SELECT 1 # 2; DELETE FROM t` would look
// like a single statement.
func skipSpaceDialect(query string, i int, d Dialect) int {
	n := len(query)
	for i < n {
		switch {
		case query[i] == ' ' || query[i] == '\t' || query[i] == '\n' || query[i] == '\r':
			i++
		case i+1 < n && query[i] == '-' && query[i+1] == '-' && lineCommentOpens(query, i, d):
			for i < n && query[i] != '\n' {
				i++
			}
		case d.HashComment && query[i] == '#':
			for i < n && query[i] != '\n' {
				i++
			}
		case i+1 < n && query[i] == '/' && query[i+1] == '*':
			i += 2
			for i < n {
				if i+1 < n && query[i] == '*' && query[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
		default:
			return i
		}
	}
	return i
}

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
		s.i, _ = skipSingleQuotedDialect(s.q, s.i, s.d)
		return true
	case c == '"':
		s.i, _ = skipDoubleQuotedDialect(s.q, s.i, s.d)
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

// skipSingleQuotedDialect advances past a single-quoted literal starting at i
// and reports whether the literal's extent is ambiguous.
//
// A literal ends at an unescaped quote, but which quotes count as escaped
// depends on the server, not on the SQL text: MySQL honors `\'` unless
// NO_BACKSLASH_ESCAPES is set, and PostgreSQL honors it inside E'...' always
// but inside a plain literal only when standard_conforming_strings is off.
// Guessing is not safe in either direction — read `\'` as an escape when the
// server does not and the scan runs past the real terminator, swallowing
// whatever follows; read it as a terminator when the server does not and the
// scan desynchronizes the same way. So the ambiguous case is reported, and
// callers that need a trustworthy scan refuse the statement instead.
func skipSingleQuotedDialect(query string, i int, d Dialect) (int, bool) {
	n := len(query)
	escaped := d.BackslashEscape && hasEscapeStringPrefix(query, i)
	ambiguous := false
	i++ // opening quote
	for i < n {
		switch query[i] {
		case '\\':
			if i+1 >= n {
				return n, ambiguous
			}
			if escaped {
				i += 2
				continue
			}
			if d.BackslashEscape && (query[i+1] == '\'' || query[i+1] == '\\') {
				// The extent depends on a server setting this scanner cannot see.
				ambiguous = true
				i += 2
				continue
			}
			i++
		case '\'':
			i++
			if i < n && query[i] == '\'' {
				i++ // doubled quote — the portable escape, never ambiguous
				continue
			}
			return i, ambiguous
		default:
			i++
		}
	}
	return i, ambiguous
}

// hasEscapeStringPrefix reports whether the quote at i opens a PostgreSQL
// escape string, E'...', where backslash escapes always apply regardless of
// standard_conforming_strings.
func hasEscapeStringPrefix(query string, i int) bool {
	if i == 0 {
		return false
	}
	if query[i-1] != 'E' && query[i-1] != 'e' {
		return false
	}
	return i < 2 || !isIdentCharByte(query[i-2])
}

// HasAmbiguousStringEscape reports whether query contains a single-quoted
// literal holding a backslash-escaped quote whose meaning depends on a server
// setting (MySQL's NO_BACKSLASH_ESCAPES, PostgreSQL's
// standard_conforming_strings). When it does, no scan of that query can be
// trusted: the literal's end — and therefore everything after it — is read
// differently depending on the setting.
func HasAmbiguousStringEscape(query string, d Dialect) bool {
	if !d.BackslashEscape {
		return false
	}
	s := &sqlScanner{q: query, d: d}
	for {
		s.skipSpace()
		if s.eof() {
			return false
		}
		if s.q[s.i] == '\'' {
			next, ambiguous := skipSingleQuotedDialect(s.q, s.i, s.d)
			if ambiguous {
				return true
			}
			s.i = next
			continue
		}
		if s.q[s.i] == '"' {
			next, ambiguous := skipDoubleQuotedDialect(s.q, s.i, s.d)
			if ambiguous {
				return true
			}
			s.i = next
			continue
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

// lineCommentOpens reports whether the -- at i opens a comment. MySQL requires
// the second dash to be followed by whitespace or a control character, so
// `1--1` is arithmetic there and only `1-- 1` is a comment. Reading it as a
// comment either way hides everything after it, including a statement
// separator.
func lineCommentOpens(query string, i int, d Dialect) bool {
	if !d.DoubleDashNeedsSpace {
		return true
	}
	if i+2 >= len(query) {
		return false
	}
	c := query[i+2]
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c < 0x20
}

// skipDoubleQuotedDialect advances past a double-quoted run starting at i and
// reports whether its extent is ambiguous.
//
// On MySQL a double-quoted run is a string, not an identifier, unless
// ANSI_QUOTES is set — and in string mode a backslash escapes the closing
// quote unless NO_BACKSLASH_ESCAPES is set. Neither setting is visible from
// the SQL text, so `"a\"b"` can end at either quote depending on the server.
// As with single quotes, the ambiguity is reported instead of guessed at.
func skipDoubleQuotedDialect(query string, i int, d Dialect) (int, bool) {
	if !d.DoubleQuoteMayBeString || !d.BackslashEscape {
		return skipDoubleQuoted(query, i), false
	}
	n := len(query)
	ambiguous := false
	i++ // opening quote
	for i < n {
		switch query[i] {
		case '\\':
			if i+1 >= n {
				return n, ambiguous
			}
			if query[i+1] == '"' || query[i+1] == '\\' {
				ambiguous = true
				i += 2
				continue
			}
			i++
		case '"':
			i++
			if i < n && query[i] == '"' {
				i++ // doubled quote — portable, never ambiguous
				continue
			}
			return i, ambiguous
		default:
			i++
		}
	}
	return i, ambiguous
}

// HasExecutableComment reports whether query contains a MySQL executable
// comment, /*! ... */ (or MariaDB's /*M! ... */).
//
// Every other comment form can be skipped because the server ignores it. This
// one the server runs: `SELECT 1; /*! DELETE FROM t */` is two statements to
// MySQL and one statement plus a comment to every scanner that does not know
// the form. Callers refuse rather than parse the contents — the payload can be
// anything, and an observation tool has no use for the construct.
func HasExecutableComment(query string, d Dialect) bool {
	if !d.ExecutableComment {
		return false
	}
	n := len(query)
	s := &sqlScanner{q: query, d: d}
	for s.i < n {
		if s.i+2 < n && s.q[s.i] == '/' && s.q[s.i+1] == '*' {
			rest := s.q[s.i+2:]
			if rest[0] == '!' || (len(rest) > 1 && (rest[0] == 'M' || rest[0] == 'm') && rest[1] == '!') {
				return true
			}
			s.skipSpace() // skips the ordinary block comment
			continue
		}
		if s.skipQuoted() {
			continue
		}
		if w := s.word(); w != "" {
			continue
		}
		s.i++
	}
	return false
}
