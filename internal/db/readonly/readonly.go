// Package readonly refuses statements that would write, before asql sends
// them to a database.
//
// It is not a sandbox. It defends against the accidental DELETE typed at a
// production connection, not against a user who is determined to write. Trying
// to guarantee the latter means chasing every construct of every dialect,
// which costs more than the protection is worth. That line is what keeps this
// package small.
//
// The guard is an allow-list: a statement is refused unless it is recognized
// as read-only. Unknown keywords are refused, so a dialect asql has never seen
// fails closed.
package readonly

import (
	"errors"
	"strings"

	"github.com/kwrkb/asql/internal/db/dbutil"
)

// allowedLeading are the statement keywords that cannot write.
//
// "with", "explain" and "pragma" are deliberately absent: each needs to be
// checked further (see Check) because each can carry a writing statement
// inside it.
var allowedLeading = map[string]bool{
	"select":   true,
	"values":   true,
	"table":    true,
	"show":     true,
	"describe": true,
	"desc":     true,
}

// allowedPragmas are the SQLite pragmas that only report schema information.
//
// A pragma is not read-only just because it takes no argument or has no `=`:
// SQLite accepts the function form as a setter, so `PRAGMA query_only(0)`
// writes despite looking like a getter. Membership here therefore means the
// name has no side effect in *any* of its syntactic forms. Getters like
// journal_mode are left out rather than parsed for argument shape, because
// reading them is worth little while classifying them correctly costs a lot.
var allowedPragmas = map[string]bool{
	"table_info":       true,
	"table_xinfo":      true,
	"table_list":       true,
	"index_list":       true,
	"index_info":       true,
	"index_xinfo":      true,
	"foreign_key_list": true,
	"database_list":    true,
	"collation_list":   true,
	"function_list":    true,
	"module_list":      true,
	"pragma_list":      true,
}

// Error reports a statement refused by the guard.
type Error struct {
	// Subject names what was refused — a SQL keyword ("DELETE", "PRAGMA
	// journal_mode") or a shape ("multiple statements").
	Subject string
	// Detail explains why, when the subject alone does not.
	Detail string
}

func (e *Error) Error() string {
	msg := "readonly: " + e.Subject + " is not allowed"
	if e.Detail == "" {
		return msg + " (asql --readonly)"
	}
	return msg + " (" + e.Detail + ")"
}

// IsRefused reports whether err came from the guard rather than the database.
// It unwraps, so a refusal stays recognizable if a caller ever wraps it on the
// way to the status bar.
func IsRefused(err error) bool {
	var refusal *Error
	return errors.As(err, &refusal)
}

// Check reports whether query may run against a read-only connection.
// dbType is the value reported by db.DBAdapter.Type; it selects the quoting
// styles the scanner recognizes.
func Check(query string, dbType string) error {
	d := dbutil.DialectFor(dbType)
	// Before classifying anything, make sure the statement can be read at all.
	// A backslash-escaped quote means the literal's extent — and so where every
	// later keyword falls — depends on a server setting the guard cannot see.
	if dbutil.HasAmbiguousStringEscape(query, d) {
		return &Error{
			Subject: "a backslash-escaped quote in a string literal",
			Detail:  "its extent depends on a server setting; write it as '' instead",
		}
	}
	if dbutil.HasMultipleStatements(query, d) {
		return &Error{
			Subject: "multiple statements",
			Detail:  "only the first statement would be classified",
		}
	}
	return classify(query, d, 0)
}

// maxExplainDepth bounds the EXPLAIN recursion. One level covers every real
// use; deeper nesting is refused rather than followed.
const maxExplainDepth = 1

func classify(query string, d dbutil.Dialect, depth int) error {
	keyword := dbutil.LeadingKeyword(query)
	switch {
	case keyword == "":
		return &Error{Subject: "this statement", Detail: "no SQL keyword found"}

	case allowedLeading[keyword]:
		return checkIntoTarget(query, d)

	case keyword == "with":
		return classifyWith(query, d)

	case keyword == "explain":
		if depth >= maxExplainDepth {
			return &Error{Subject: "nested EXPLAIN", Detail: "cannot be classified"}
		}
		target, ok := dbutil.StripExplain(query, d)
		if !ok {
			return &Error{Subject: "EXPLAIN", Detail: "its target could not be read"}
		}
		// EXPLAIN ANALYZE runs the statement it explains, so the target is
		// classified exactly as if it had been submitted on its own.
		return classify(target, d, depth+1)

	case keyword == "pragma":
		name, ok := dbutil.PragmaName(query, d)
		if !ok {
			return &Error{Subject: "PRAGMA", Detail: "its name could not be read"}
		}
		if !allowedPragmas[name] {
			return &Error{Subject: "PRAGMA " + name, Detail: "not a schema-inspection pragma"}
		}
		return nil

	default:
		return &Error{Subject: strings.ToUpper(keyword)}
	}
}

// classifyWith accepts a WITH statement only when its body and every CTE term
// are themselves allowed. Checking the body alone would let a data-modifying
// CTE through, since those run as part of the same statement while leaving the
// body keyword a plain SELECT.
func classifyWith(query string, d dbutil.Dialect) error {
	terms, ok := dbutil.CteTermKeywords(query, d)
	if !ok {
		return &Error{Subject: "WITH", Detail: "its CTE terms could not be read"}
	}
	for _, term := range terms {
		// A CTE term may itself be a WITH or an EXPLAIN; both are refused
		// here rather than parsed further, because the term's text is not
		// isolated and re-classifying it would be guesswork.
		//
		// The scan covers every `AS (` group in the statement, not only the
		// CTE definitions, because PostgreSQL allows a WITH inside a subquery
		// in the body. The cost is that a column definition list in the body
		// — `SELECT * FROM f() AS (x int)` — is read as a term and refused.
		// That is the allow-list failing closed on syntax it cannot classify,
		// and the workaround (name the alias: `AS r(x int)`) is small, so the
		// scan is left wide rather than narrowed to the CTE region.
		if !allowedLeading[term] {
			return &Error{
				Subject: strings.ToUpper(term),
				Detail:  "an AS (...) group must open with a read-only statement",
			}
		}
	}
	body := dbutil.CteBodyKeyword(query)
	if body == "" {
		return &Error{Subject: "WITH", Detail: "its body could not be read"}
	}
	if !allowedLeading[body] {
		return &Error{Subject: strings.ToUpper(body), Detail: "as the body of WITH"}
	}
	return checkIntoTarget(query, d)
}

// checkIntoTarget refuses a statement that would send its result somewhere
// instead of returning it.
//
// A read-only leading keyword is not enough: PostgreSQL's
// `SELECT * INTO backup FROM t` creates and fills a table, and MySQL's
// `SELECT ... INTO OUTFILE '/path'` writes a file on the server. Both start
// with SELECT, and neither MySQL nor PostgreSQL has a verified connection-level
// layer here, so the statement would reach the database and write for real.
//
// INTO is reserved in every dialect asql speaks, so a bare INTO outside string
// literals and quoted identifiers is always the clause and never a column name.
// MySQL's `SELECT a INTO @var` only assigns a session variable and is refused
// along with the rest — separating it would mean parsing the target, which buys
// nothing for an observation tool.
func checkIntoTarget(query string, d dbutil.Dialect) error {
	if dbutil.ContainsKeyword(query, "into", d) {
		return &Error{
			Subject: "SELECT ... INTO",
			Detail:  "it writes its result to a table or a file",
		}
	}
	return nil
}
