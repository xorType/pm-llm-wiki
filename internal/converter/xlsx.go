package converter

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

// fromXLSX handles both .xlsx and .csv, returning markdown table(s).
func fromXLSX(path string) (string, error) {
	if strings.ToLower(filepath.Ext(path)) == ".csv" {
		return fromCSV(path)
	}
	return fromExcel(path)
}

func fromCSV(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	rows, err := csv.NewReader(bufio.NewReader(f)).ReadAll()
	if err != nil {
		return "", fmt.Errorf("parsing CSV %q: %w", path, err)
	}
	return rowsToMarkdown(rows), nil
}

func fromExcel(path string) (string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return "", fmt.Errorf("opening xlsx %q: %w", path, err)
	}
	defer f.Close()

	var sb strings.Builder
	for _, sheet := range f.GetSheetList() {
		rows, err := f.GetRows(sheet)
		if err != nil || len(rows) == 0 {
			continue
		}
		fmt.Fprintf(&sb, "## Sheet: %s\n\n", sheet)
		sb.WriteString(rowsToMarkdown(rows))
		sb.WriteString("\n\n")
	}
	return sb.String(), nil
}

// rowsToMarkdown converts a 2-D string slice to a GFM table.
func rowsToMarkdown(rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}

	// Find max column count.
	maxCols := 0
	for _, row := range rows {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}

	pad := func(row []string) []string {
		for len(row) < maxCols {
			row = append(row, "")
		}
		return row
	}

	escape := func(s string) string {
		return strings.ReplaceAll(strings.TrimSpace(s), "|", "\\|")
	}

	rowLine := func(row []string) string {
		cells := make([]string, len(row))
		for i, c := range row {
			cells[i] = escape(c)
		}
		return "| " + strings.Join(cells, " | ") + " |"
	}

	var sb strings.Builder
	sb.WriteString(rowLine(pad(rows[0])) + "\n")

	sep := make([]string, maxCols)
	for i := range sep {
		sep[i] = "---"
	}
	sb.WriteString("| " + strings.Join(sep, " | ") + " |\n")

	for _, row := range rows[1:] {
		sb.WriteString(rowLine(pad(row)) + "\n")
	}
	return sb.String()
}
