package dbutil

import "strings"

// This file reads the *shape* of a SQL statement — how many statements are
// there, what does EXPLAIN explain, which keyword opens each CTE term — for
// callers that must decide something about a statement before running it.
//
// It reads one portable subset of SQL rather than each dialect's own lexical
// rules, and refuses to read anything outside that subset (see
// UnlexableReason). The subset is:
//
//	'...' / "..." / `...`   quoted runs, closed by doubling the quote
//	[...]                   bracket-quoted identifier
//	-- ...                  line comment, space required after the dashes
//	/* ... */               block comment
//
// Everything else that dialects disagree about is refused, not interpreted:
// a backslash-escaped quote, a bare #, `--` without a space, an executable
// /*! ... */ comment.
//
// The alternative — teaching the scanner each dialect's rules — was tried and
// abandoned. Every rule learned revealed another: # is a comment on MySQL and
// an operator on PostgreSQL; `--` needs a following space on MySQL but not on
// PostgreSQL; `\'` closes a literal or does not depending on
// NO_BACKSLASH_ESCAPES, standard_conforming_strings and ANSI_QUOTES, which are
// server settings invisible to the text; /*! ... */ is a comment everywhere
// except MySQL, where the server runs it. Each of those was a way to read a
// write as a read. Refusing the whole ambiguous class ends the category
// instead of the instance, at the cost of refusing some valid dialect-specific
// text — a trade an observation tool can afford, since the portable spelling
// (double the quote, put a space after the dashes) is always available.

// UnlexableReason reports why query cannot be read under the portable subset,
// or "" when it can. A non-empty reason means no other function in this file
// can be trusted on that query: the extent of a quoted run or a comment is
// what decides where every later keyword falls.
func UnlexableReason(query string) string {
	i := 0
	n := len(query)
	for i < n {
		switch c := query[i]; {
		case c == '#':
			return "a # character outside a quoted string (a comment on MySQL, an operator on PostgreSQL)"
		case c == '-' && i+1 < n && query[i+1] == '-',
			c == '/' && i+1 < n && query[i+1] == '*':
			next, reason := commentRun(query, i)
			if reason != "" {
				return reason
			}
			i = next
		case c == '\'' || c == '"' || c == '`' || c == '[':
			next, reason := quoteRun(query, i)
			if reason != "" {
				return reason
			}
			i = next
		default:
			i++
		}
	}
	return ""
}

// quoteRun returns the index just past the quoted run starting at i, and a
// reason when the run cannot be read portably. i must be at a quote opener.
func quoteRun(query string, i int) (int, string) {
	open := query[i]
	closer := open
	if open == '[' {
		closer = ']'
	}
	doubling := open != '['
	n := len(query)
	j := i + 1
	for j < n {
		c := query[j]
		if c == '\\' && j+1 < n && (query[j+1] == closer || query[j+1] == '\\') {
			// MySQL honors this escape unless NO_BACKSLASH_ESCAPES is set;
			// PostgreSQL honors it inside E'...' always and inside a plain
			// literal when standard_conforming_strings is off; SQLite never
			// does. Whichever reading the scanner picks, the run ends
			// somewhere else on some server, and the text after it is read as
			// something other than what it is.
			return j, "a backslash-escaped quote inside a quoted string (double the quote instead)"
		}
		if c == closer {
			j++
			if doubling && j < n && query[j] == closer {
				j++
				continue
			}
			return j, ""
		}
		j++
	}
	return n, "an unterminated quoted string"
}

// commentRun returns the index just past the comment starting at i, and a
// reason when the comment cannot be read portably. i must be at -- or /*.
func commentRun(query string, i int) (int, string) {
	n := len(query)
	if query[i] == '-' {
		// MySQL needs whitespace after the dashes; PostgreSQL and SQLite do
		// not. `SELECT 1--1` is therefore a comment on one and arithmetic on
		// another, and reading it as a comment hides the rest of the line.
		if i+2 < n {
			c := query[i+2]
			if c != ' ' && c != '\t' && c != '\n' && c != '\r' && c >= 0x20 {
				return i, "a -- comment with no space after the dashes"
			}
		}
		j := i
		for j < n && query[j] != '\n' {
			j++
		}
		return j, ""
	}
	// Block comment. MySQL runs the contents of /*! ... */ (and MariaDB's
	// /*M! ... */) instead of ignoring them, which turns a comment into a
	// statement no other scanner would see.
	if i+2 < n && query[i+2] == '!' {
		return i, "a MySQL executable comment (/*! ... */)"
	}
	if i+3 < n && (query[i+2] == 'M' || query[i+2] == 'm') && query[i+3] == '!' {
		return i, "a MariaDB executable comment (/*M! ... */)"
	}
	j := i + 2
	for j+1 < n {
		if query[j] == '*' && query[j+1] == '/' {
			return j + 2, ""
		}
		j++
	}
	return n, "an unterminated block comment"
}

// sqlScanner walks a SQL string one significant token at a time under the
// portable subset. Constructs UnlexableReason rejects are treated as ordinary
// characters here rather than interpreted, so a caller that skipped the
// UnlexableReason check still fails toward seeing more than it should — a
// semicolon stays visible, a keyword stays a keyword.
type sqlScanner struct {
	q string
	i int
}

func (s *sqlScanner) eof() bool { return s.i >= len(s.q) }

// skipSpace advances past whitespace and portable comments.
func (s *sqlScanner) skipSpace() {
	n := len(s.q)
	for s.i < n {
		c := s.q[s.i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			s.i++
			continue
		}
		if (c == '-' && s.i+1 < n && s.q[s.i+1] == '-') || (c == '/' && s.i+1 < n && s.q[s.i+1] == '*') {
			next, reason := commentRun(s.q, s.i)
			if reason != "" {
				return // not a portable comment: leave it for the caller to read
			}
			s.i = next
			continue
		}
		return
	}
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

// skipQuoted advances past a quoted run when the scanner is positioned at one,
// and reports whether it advanced.
func (s *sqlScanner) skipQuoted() bool {
	if s.eof() {
		return false
	}
	switch s.q[s.i] {
	case '\'', '"', '`', '[':
		next, _ := quoteRun(s.q, s.i)
		s.i = next
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
// other than a single trailing semicolon.
//
// LeadingKeyword only inspects the first statement, so "SELECT 1; DELETE FROM t"
// looks like a SELECT to it. Callers that classify a statement by its leading
// keyword must reject multi-statement input first. Whether the driver would
// actually execute the tail is dialect- and DSN-dependent (MySQL needs
// multiStatements, PostgreSQL allows it over the simple protocol), which is
// why this does not consult either.
func HasMultipleStatements(query string) bool {
	s := &sqlScanner{q: query}
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
func CteTermKeywords(query string) ([]string, bool) {
	s := &sqlScanner{q: query}
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
func StripExplain(query string) (string, bool) {
	s := &sqlScanner{q: query}
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
		if w == "" {
			return "", false
		}
		if !explainOptionWords[w] {
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
func PragmaName(query string) (string, bool) {
	s := &sqlScanner{q: query}
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

// ContainsKeyword reports whether keyword appears in query as a bare word —
// not inside a quoted run, not inside a comment, and not as part of a longer
// identifier. keyword must be lowercase; the match is case-insensitive.
func ContainsKeyword(query string, keyword string) bool {
	s := &sqlScanner{q: query}
	for {
		s.skipSpace()
		if s.eof() {
			return false
		}
		if s.skipQuoted() {
			continue
		}
		start := s.i
		if w := s.word(); w != "" {
			if w == keyword && start == s.i-len(keyword) {
				return true
			}
			continue
		}
		s.i++
	}
}
