package dbutil

import (
	"database/sql"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kwrkb/asql/internal/db"
)

// StringifyValue converts a database value to its string representation.
func StringifyValue(value any) string {
	s, _ := StringifyValueKind(value)
	return s
}

// StringifyValueKind converts a database value to its string representation and
// reports what that string stands for. The string is identical to what
// StringifyValue returns; the kind is the only extra information, and it is
// available here because this is the last point at which the driver's typed
// value exists (Rows is [][]string from here on).
//
// Integer and float values are formatted with fmt.Sprint, which uses the
// shortest round-tripping representation for floats, so KindInt/KindFloat
// strings parse back to the original value exactly.
func StringifyValueKind(value any) (string, db.Kind) {
	switch v := value.(type) {
	case nil:
		return db.NullSentinel, db.KindNull
	case []byte:
		// A []byte that is valid UTF-8 is treated as text here, because drivers
		// return []byte for ordinary string columns too — the MySQL driver does
		// it for every VARCHAR/TEXT — and classifying those as blobs would
		// mistype most MySQL string data. Callers that know the column is
		// declared binary should use BinaryColumnType to override this; see
		// ScanRowsLimit.
		if utf8.Valid(v) {
			return string(v), db.KindText
		}
		return fmt.Sprintf("%x", v), db.KindBlob
	case time.Time:
		return v.Format(time.RFC3339), db.KindText
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprint(v), db.KindInt
	case float32, float64:
		return fmt.Sprint(v), db.KindFloat
	default:
		// bool, and anything else a driver returns, keeps its fmt.Sprint form
		// and is treated as text — that is exactly what is displayed.
		return fmt.Sprint(v), db.KindText
	}
}

// binaryColumnTypes are the column type names, as drivers report them through
// ColumnType.DatabaseTypeName, whose values are binary rather than text.
//
// The list is an exact-match set on purpose: substring matching on "BINARY"
// or "BLOB" is the kind of rule that quietly catches an unrelated type later.
// It covers the three databases asql supports. Both MySQL cases are safe —
// go-sql-driver reports a BLOB-family column as "TEXT" when its charset is not
// binary, so ordinary MySQL text never lands here.
var binaryColumnTypes = map[string]bool{
	"BLOB": true, "TINYBLOB": true, "MEDIUMBLOB": true, "LONGBLOB": true, // SQLite, MySQL
	"BINARY": true, "VARBINARY": true, // MySQL
	"BYTEA": true, // PostgreSQL
}

// BinaryColumnType reports whether a driver-reported column type name denotes
// binary data.
func BinaryColumnType(name string) bool {
	return binaryColumnTypes[strings.ToUpper(strings.TrimSpace(name))]
}

// numericColumnTypes are the column type names whose values some drivers hand
// back as text rather than as a Go number. MySQL does this for DECIMAL: the
// driver leaves the digits alone rather than routing them through a float64 and
// losing precision, so they arrive here as a []byte that happens to be valid
// UTF-8 and would otherwise be classified as text.
//
// Both MySQL and PostgreSQL report the SQL standard names for these.
var numericColumnTypes = map[string]bool{
	"DECIMAL": true, "NUMERIC": true,
}

// NumericColumnType reports whether a driver-reported column type name denotes
// a number that the driver may return as text.
func NumericColumnType(name string) bool {
	return numericColumnTypes[strings.ToUpper(strings.TrimSpace(name))]
}

// maxExactFloatDigits is how many significant decimal digits a float64 always
// represents exactly. Beyond it, a decimal cannot survive the trip through a
// float64 and is better left as text than silently rounded.
const maxExactFloatDigits = 15

// numericKindOf classifies a decimal written out as text. It reports false when
// the value is not a number, or has more precision than a float64 can hold —
// SQLite has no decimal type, so such a value stays text rather than being
// quietly rounded on the way into a brought table.
func numericKindOf(s string) (db.Kind, bool) {
	if s == "" {
		return db.KindText, false
	}
	if _, err := strconv.ParseInt(s, 10, 64); err == nil {
		return db.KindInt, true
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return db.KindText, false
	}
	if significantDigits(s) > maxExactFloatDigits {
		return db.KindText, false
	}
	return db.KindFloat, true
}

// significantDigits counts the decimal digits in s, ignoring a sign, the
// decimal point and leading zeros. Trailing zeros are counted, which overstates
// the precision of a value like "2.50" — an overstatement is the safe
// direction, since it can only keep a value as text that would have converted
// cleanly.
func significantDigits(s string) int {
	n := 0
	seenNonZero := false
	for _, r := range s {
		switch {
		case r == 'e' || r == 'E':
			return n
		case r >= '1' && r <= '9':
			seenNonZero = true
			n++
		case r == '0':
			if seenNonZero {
				n++
			}
		}
	}
	return n
}

// DefaultRowLimit is the maximum number of rows ScanRows will read.
// Use ScanRowsOpts to override this default.
const DefaultRowLimit = 10_000

// ScanOptions describes how the calling driver represents values, so the
// scanner can classify them correctly (see db.Kind).
type ScanOptions struct {
	// Limit caps how many rows are read. 0 means no limit.
	Limit int

	// BytesAreBinary says this driver returns []byte only for binary values and
	// never for text, so the Go type alone identifies a blob.
	//
	// modernc.org/sqlite behaves this way: it returns a Go string for every TEXT
	// storage class and []byte for every BLOB one, whatever the column was
	// declared as. That is strictly better information than the declared type,
	// because SQLite is dynamically typed — a TEXT column can hold a blob, a BLOB
	// column can hold a string, and an expression such as a blob literal or
	// randomblob(4) carries no declared type at all.
	//
	// Leave it false for drivers that also use []byte for strings — the MySQL
	// driver returns []byte for every VARCHAR — and the scanner falls back to the
	// driver-reported column type instead.
	BytesAreBinary bool
}

// columnHints carries what the driver said about a column, for classifyValue.
type columnHints struct {
	binary         bool // the column type names a binary type
	numeric        bool // the column type names a number the driver may return as text
	bytesAreBinary bool // this driver uses []byte for binary values only
}

// classifyValue turns a scanned driver value into the string Rows will carry
// and the Kind that says what that string stands for.
//
// The order of the steps is load-bearing and has been wrong twice: the
// column-type overrides must run before the empty-value sentinel, and the
// sentinel must not undo them. Keep the steps here, in one place, rather than
// spreading them back through the scan loop.
func classifyValue(value any, hints columnHints) (string, db.Kind) {
	s, k := StringifyValueKind(value)

	// A number the driver handed back as text — MySQL does this for DECIMAL —
	// would otherwise sort and join as a string.
	if k == db.KindText && hints.numeric {
		if nk, ok := numericKindOf(s); ok {
			k = nk
		}
	}

	// Binary data that happens to be valid UTF-8 is indistinguishable from text
	// by value alone, so trust what the driver says about the column (or, where
	// the driver only uses []byte for binary, about the Go type). Show the hex
	// form for every binary value, not just the ones that failed the UTF-8
	// check, so a binary column reads consistently and round-trips as a blob.
	if k == db.KindText && (hints.bytesAreBinary || hints.binary) {
		if b, ok := value.([]byte); ok {
			s, k = fmt.Sprintf("%x", b), db.KindBlob
		}
	}

	// Zero-length values are displayed as the `""` sentinel so they stay
	// visually distinct from NULL and from a blank cell. Record what the
	// sentinel stands for rather than for those two characters — but keep a
	// zero-length blob a blob, or it comes back as an empty string and stops
	// matching an empty blob literal.
	if s == "" {
		s = db.EmptySentinel
		if k != db.KindBlob {
			k = db.KindEmpty
		}
	}

	return s, k
}

// ScanRows reads rows from *sql.Rows up to DefaultRowLimit and returns a QueryResult.
// The caller is responsible for closing rows.
func ScanRows(rows *sql.Rows) (db.QueryResult, error) {
	return ScanRowsOpts(rows, ScanOptions{Limit: DefaultRowLimit})
}

// ScanRowsLimit reads rows from *sql.Rows up to the given limit.
// A limit of 0 means no limit.
func ScanRowsLimit(rows *sql.Rows, limit int) (db.QueryResult, error) {
	return ScanRowsOpts(rows, ScanOptions{Limit: limit})
}

// ScanRowsOpts reads rows from *sql.Rows under the given options.
func ScanRowsOpts(rows *sql.Rows, opts ScanOptions) (db.QueryResult, error) {
	limit := opts.Limit
	columns, err := rows.Columns()
	if err != nil {
		return db.QueryResult{}, err
	}

	// Retrieve column type names (best-effort; driver-dependent).
	var colTypes []string
	if cts, err := rows.ColumnTypes(); err == nil {
		colTypes = make([]string, len(cts))
		for i, ct := range cts {
			colTypes[i] = ct.DatabaseTypeName()
		}
	}

	// Columns the driver declares as binary. StringifyValueKind cannot tell a
	// binary value from a string one when both arrive as []byte, so a blob that
	// happens to hold valid UTF-8 would otherwise be brought over as TEXT and
	// stop matching a comparison against the original bytes. Skipped entirely
	// when the driver already distinguishes them by Go type.
	var binaryCols []bool
	if !opts.BytesAreBinary {
		binaryCols = make([]bool, len(columns))
		for i, name := range colTypes {
			binaryCols[i] = BinaryColumnType(name)
		}
	}

	// Columns the driver declares as numeric but may hand back as text, so their
	// values are not left to sort and join as strings.
	numericCols := make([]bool, len(columns))
	for i, name := range colTypes {
		numericCols[i] = NumericColumnType(name)
	}

	values := make([]any, len(columns))
	ptrs := make([]any, len(columns))
	for i := range values {
		ptrs[i] = &values[i]
	}

	resultRows := make([][]string, 0)
	// Kinds are accumulated into one flat buffer and sliced per row afterwards,
	// so scanning costs one amortized allocation for the whole result instead of
	// a second per-row allocation next to each record.
	var flatKinds []db.Kind
	truncated := false
	for rows.Next() {
		if limit > 0 && len(resultRows) >= limit {
			truncated = true
			break
		}
		if err := rows.Scan(ptrs...); err != nil {
			return db.QueryResult{}, err
		}
		record := make([]string, len(columns))
		for i, value := range values {
			s, k := classifyValue(value, columnHints{
				binary:         binaryCols != nil && binaryCols[i],
				numeric:        numericCols[i],
				bytesAreBinary: opts.BytesAreBinary,
			})
			record[i] = s
			flatKinds = append(flatKinds, k)
		}
		resultRows = append(resultRows, record)
	}
	if err := rows.Err(); err != nil {
		return db.QueryResult{}, err
	}

	var kinds [][]db.Kind
	if len(columns) > 0 && len(resultRows) > 0 {
		kinds = make([][]db.Kind, len(resultRows))
		for i := range kinds {
			start, end := i*len(columns), (i+1)*len(columns)
			kinds[i] = flatKinds[start:end:end]
		}
	}

	msg := fmt.Sprintf("%d row(s) returned", len(resultRows))
	if truncated {
		msg = fmt.Sprintf("%d row(s) returned (truncated at %d)", len(resultRows), limit)
	}

	return db.QueryResult{
		Columns:     columns,
		ColumnTypes: colTypes,
		Rows:        resultRows,
		Kinds:       kinds,
		Message:     msg,
		Truncated:   truncated,
	}, nil
}

// LeadingKeyword returns the first SQL keyword from query, skipping comments
// and leading semicolons. The result is always lowercase.
func LeadingKeyword(query string) string {
	trimmed := strings.TrimSpace(query)

	for trimmed != "" {
		switch {
		case strings.HasPrefix(trimmed, "--"), strings.HasPrefix(trimmed, "#"):
			if idx := strings.Index(trimmed, "\n"); idx >= 0 {
				trimmed = strings.TrimSpace(trimmed[idx+1:])
				continue
			}
			return ""
		case strings.HasPrefix(trimmed, "/*"):
			if idx := strings.Index(trimmed, "*/"); idx >= 0 {
				trimmed = strings.TrimSpace(trimmed[idx+2:])
				continue
			}
			return ""
		case strings.HasPrefix(trimmed, ";"):
			trimmed = strings.TrimSpace(trimmed[1:])
			continue
		}
		break
	}

	fields := strings.Fields(strings.ToLower(trimmed))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// CteBodyKeyword extracts the leading keyword of the body statement in a
// WITH (CTE) query. It skips the WITH keyword, then tracks parenthesis depth
// to skip over CTE definitions, and returns the first keyword of the body
// (e.g. "select", "insert", "delete"). Returns "" if parsing fails.
// It correctly handles string literals, comments, and parentheses.
func CteBodyKeyword(query string) string {
	i := 0
	n := len(query)

	// Skip to past the WITH keyword (LeadingKeyword already confirmed it's "with")
	i = skipWhitespaceAndComments(query, i)
	if i+4 > n {
		return ""
	}
	// Advance past "WITH"
	i += 4
	// Skip optional RECURSIVE
	j := skipWhitespaceAndComments(query, i)
	if j+9 <= n && strings.EqualFold(query[j:j+9], "recursive") {
		after := j + 9
		if after >= n || !isIdentCharByte(query[after]) {
			i = after
		}
	}

	// Now skip CTE definitions by tracking parenthesis depth.
	// We need to find the body keyword after all CTE definitions end.
	depth := 0
	for i < n {
		i = skipWhitespaceAndComments(query, i)
		if i >= n {
			break
		}
		c := query[i]
		switch {
		case c == '(':
			depth++
			i++
		case c == ')':
			depth--
			i++
		case c == '\'':
			i = skipSingleQuoted(query, i)
		case c == '"':
			i = skipDoubleQuoted(query, i)
		case c == '`':
			i = skipBacktickQuoted(query, i)
		case c == '$' && i+1 < n:
			i = skipDollarQuoted(query, i)
		default:
			if depth == 0 && c != ',' {
				// We're outside all CTE parentheses and it's not a comma separator.
				// Check if this is a SQL keyword (the body statement).
				if isIdentCharByte(c) {
					end := i
					for end < n && isIdentCharByte(query[end]) {
						end++
					}
					word := strings.ToLower(query[i:end])
					// CTE name or AS keyword — keep scanning
					if word == "as" {
						i = end
						continue
					}
					// Could be a CTE name followed by column list or AS
					// Check if this is a known body keyword
					switch word {
					case "select", "insert", "update", "delete", "merge",
						"values", "table", "show", "describe", "desc",
						"explain", "pragma":
						return word
					default:
						// CTE alias — skip past it
						i = end
						continue
					}
				}
				i++
			} else {
				i++
			}
		}
	}
	return ""
}

func isIdentCharByte(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

func skipWhitespaceAndComments(query string, i int) int {
	n := len(query)
	for i < n {
		// Skip whitespace
		if query[i] == ' ' || query[i] == '\t' || query[i] == '\n' || query[i] == '\r' {
			i++
			continue
		}
		// Line comment --
		if i+1 < n && query[i] == '-' && query[i+1] == '-' {
			for i < n && query[i] != '\n' {
				i++
			}
			continue
		}
		// MySQL # comment
		if query[i] == '#' {
			for i < n && query[i] != '\n' {
				i++
			}
			continue
		}
		// Block comment
		if i+1 < n && query[i] == '/' && query[i+1] == '*' {
			i += 2
			for i < n {
				if i+1 < n && query[i] == '*' && query[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
			continue
		}
		break
	}
	return i
}

func skipSingleQuoted(query string, i int) int {
	n := len(query)
	i++ // skip opening '
	for i < n {
		if query[i] == '\'' {
			i++
			if i < n && query[i] == '\'' {
				i++
				continue
			}
			return i
		}
		i++
	}
	return i
}

func skipDoubleQuoted(query string, i int) int {
	n := len(query)
	i++ // skip opening "
	for i < n {
		if query[i] == '"' {
			i++
			if i < n && query[i] == '"' {
				i++
				continue
			}
			return i
		}
		i++
	}
	return i
}

func skipBacktickQuoted(query string, i int) int {
	n := len(query)
	i++ // skip opening `
	for i < n && query[i] != '`' {
		i++
	}
	if i < n {
		i++ // skip closing `
	}
	return i
}

func skipDollarQuoted(query string, i int) int {
	n := len(query)
	// Try to parse a dollar-quote tag
	tag := parseDollarTag(query, i)
	if tag != "" {
		i += len(tag)
		for i+len(tag) <= n {
			if query[i:i+len(tag)] == tag {
				i += len(tag)
				return i
			}
			i++
		}
		return i
	}
	return i + 1
}

// typeShortNames maps verbose type names to shorter display forms.
var typeShortNames = map[string]string{
	"integer":            "int",
	"unsigned int":       "uint",
	"unsigned bigint":    "ubigint",
	"unsigned mediumint": "umedint",
	"unsigned smallint":  "usmallint",
	"unsigned tinyint":   "utinyint",
	"timestamptz":        "tstz",
}

// ShortenTypeName returns a shortened display name for a database column type.
// Unknown types are returned lower-cased as-is.
func ShortenTypeName(typeName string) string {
	lower := strings.ToLower(typeName)
	if short, ok := typeShortNames[lower]; ok {
		return short
	}
	return lower
}

// Dialect controls which quoting styles the SQL scanner recognizes.
//
// It describes only what ContainsReturning needs. The statement guard does not
// take a dialect at all — see the note at the top of sqlscan.go on why reading
// one portable subset beats teaching a scanner every dialect's lexical rules.
type Dialect struct {
	BracketQuote  bool // SQLite/MSSQL [identifier] style
	DollarQuote   bool // PostgreSQL $$string$$ style
	BacktickQuote bool // SQLite/MySQL `identifier` style
}

// ContainsReturning scans query for the RETURNING keyword, correctly skipping
// string literals, quoted identifiers, comments, and dialect-specific quoting.
func ContainsReturning(query string, dialect Dialect) bool {
	const kw = "returning"
	i := 0
	n := len(query)
	for i < n {
		switch {
		case query[i] == '-' && i+1 < n && query[i+1] == '-':
			for i < n && query[i] != '\n' {
				i++
			}
		case query[i] == '/' && i+1 < n && query[i+1] == '*':
			i += 2
			for i < n {
				if query[i] == '*' && i+1 < n && query[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
		case query[i] == '\'':
			i = skipSingleQuoted(query, i)
		case query[i] == '"':
			i = skipDoubleQuoted(query, i)
		case dialect.BacktickQuote && query[i] == '`':
			i = skipBacktickQuoted(query, i)
		case dialect.BracketQuote && query[i] == '[':
			i++
			for i < n && query[i] != ']' {
				i++
			}
			if i < n {
				i++
			}
		case dialect.DollarQuote && query[i] == '$' && i+1 < n:
			i = skipDollarQuoted(query, i)
		default:
			if i+len(kw) <= n && strings.EqualFold(query[i:i+len(kw)], kw) {
				before := i == 0 || !isIdentCharByte(query[i-1])
				after := i+len(kw) >= n || !isIdentCharByte(query[i+len(kw)])
				if before && after {
					return true
				}
			}
			i++
		}
	}
	return false
}

func parseDollarTag(query string, i int) string {
	n := len(query)
	if i >= n || query[i] != '$' {
		return ""
	}
	j := i + 1
	for j < n && query[j] != '$' {
		c := query[j]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return ""
		}
		j++
	}
	if j >= n {
		return ""
	}
	return query[i : j+1]
}
