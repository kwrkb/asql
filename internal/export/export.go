package export

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// FormatCSV formats query results as CSV.
func FormatCSV(headers []string, rows [][]string) (string, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(headers); err != nil {
		return "", fmt.Errorf("writing CSV header: %w", err)
	}
	for _, row := range rows {
		if err := w.Write(row); err != nil {
			return "", fmt.Errorf("writing CSV row: %w", err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return "", fmt.Errorf("flushing CSV: %w", err)
	}
	return buf.String(), nil
}

// FormatJSON formats query results as a JSON array of objects.
func FormatJSON(headers []string, rows [][]string) (string, error) {
	records := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		record := make(map[string]string, len(headers))
		for i, h := range headers {
			if i < len(row) {
				record[h] = row[i]
			} else {
				record[h] = ""
			}
		}
		records = append(records, record)
	}
	b, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling JSON: %w", err)
	}
	return string(b), nil
}

// FormatMarkdown formats query results as a GitHub Flavored Markdown table.
func FormatMarkdown(headers []string, rows [][]string) string {
	escape := func(s string) string {
		return strings.ReplaceAll(s, "|", "\\|")
	}

	var b strings.Builder

	// Header row
	b.WriteByte('|')
	for _, h := range headers {
		b.WriteByte(' ')
		b.WriteString(escape(h))
		b.WriteString(" |")
	}
	b.WriteByte('\n')

	// Separator row
	b.WriteByte('|')
	for range headers {
		b.WriteString(" --- |")
	}
	b.WriteByte('\n')

	// Data rows
	for _, row := range rows {
		b.WriteByte('|')
		for i := range headers {
			b.WriteByte(' ')
			if i < len(row) {
				b.WriteString(escape(row[i]))
			}
			b.WriteString(" |")
		}
		b.WriteByte('\n')
	}

	return b.String()
}

// SaveCSVFile writes query results to a CSV file with a timestamped name
// in the current directory. Returns the filename.
func SaveCSVFile(headers []string, rows [][]string) (string, error) {
	content, err := FormatCSV(headers, rows)
	if err != nil {
		return "", err
	}
	filename := fmt.Sprintf("result_%s.csv", time.Now().Format("20060102_150405"))
	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("writing file %s: %w", filename, err)
	}
	return filename, nil
}
