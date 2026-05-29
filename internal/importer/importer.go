package importer

import (
	"encoding/csv"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unsafe"

	"github.com/ananthakumaran/paisa/internal/ledger"
	"github.com/ananthakumaran/paisa/internal/model/template"
	"github.com/aymerick/raymond"
	"github.com/extrame/xls"
	log "github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// predictNoArgRe matches {{predictAccount hash=...}} calls with no positional arg.
// Transforms to {{predictAccount ROW._all hash=...}} so the helper always gets 1 arg.
var predictNoArgRe = regexp.MustCompile(`\{\{predictAccount\s+([a-zA-Z_][a-zA-Z0-9_]*=)`)

// inTransactionRe matches a date line starting a ledger transaction.
var inTransactionRe = regexp.MustCompile(`^\d{4}[/-]\d{2}[/-]\d{2}`)

// Run parses filePath, applies the named template, and returns formatted ledger transactions.
func Run(filePath, templateName string, db *gorm.DB, noPredict bool) (string, error) {
	data, err := parseFile(filePath)
	if err != nil {
		return "", err
	}

	tplContent, err := findTemplate(templateName)
	if err != nil {
		return "", err
	}

	// Inject ROW._all for predictAccount calls with no positional arg.
	tplContent = predictNoArgRe.ReplaceAllString(tplContent, "{{predictAccount ROW._all $1")

	tpl, err := raymond.Parse(tplContent)
	if err != nil {
		return "", fmt.Errorf("template parse error: %w", err)
	}
	tpl.RegisterHelpers(buildHelpers(db, noPredict))

	allRows := asRows(data)
	log.Debugf("parsed %d rows from %s", len(allRows), filePath)

	var outputs []string
	for _, row := range allRows {
		ctx := buildContext(row, allRows)
		out, err := tpl.Exec(ctx)
		if err != nil {
			return "", fmt.Errorf("template render error: %w", err)
		}
		out = html.UnescapeString(out)
		// Remove whitespace-only lines from raymond's standalone block handling.
		// raymond emits lines of pure whitespace (e.g. "    ") when a conditional
		// block is false, which would reset FormatContent's inTransaction state.
		rawLines := strings.Split(out, "\n")
		var kept []string
		for _, l := range rawLines {
			if strings.TrimSpace(l) != "" {
				kept = append(kept, l)
			}
		}
		out = strings.Join(kept, "\n")
		out = strings.TrimSpace(out)
		if out != "" {
			outputs = append(outputs, out)
		}
	}
	log.Debugf("%d of %d rows produced output", len(outputs), len(allRows))

	result := strings.Join(outputs, "\n\n")
	result = ledger.FormatContent(result)
	// Normalize excessive indentation caused by raymond's standalone block whitespace bug.
	// raymond sometimes adds extra leading whitespace (matching the outer block's indent)
	// to lines that follow a true block + false block sequence at the same nesting level.
	result = normalizeIndentation(result)
	return result, nil
}

// normalizeIndentation reduces any run of 5+ leading spaces to 4 spaces for lines
// inside a transaction. This is a workaround for raymond's standalone block handling
// which can add extra indentation within nested {{#if}} sequences.
func normalizeIndentation(text string) string {
	lines := strings.Split(text, "\n")
	inTransaction := false
	for i, line := range lines {
		if inTransactionRe.MatchString(line) {
			inTransaction = true
			continue
		}
		if strings.TrimSpace(line) == "" || (len(line) > 0 && line[0] != ' ' && line[0] != '\t') {
			inTransaction = false
			continue
		}
		if inTransaction {
			trimmed := strings.TrimLeft(line, " \t")
			if len(line)-len(trimmed) > 4 {
				lines[i] = "    " + trimmed
			}
		}
	}
	return strings.Join(lines, "\n")
}

// findTemplate looks up a template by name (case-sensitive, then case-insensitive).
func findTemplate(name string) (string, error) {
	templates := template.All()
	for _, t := range templates {
		if t.Name == name {
			return t.Content, nil
		}
	}
	nameLower := strings.ToLower(name)
	for _, t := range templates {
		if strings.ToLower(t.Name) == nameLower {
			return t.Content, nil
		}
	}
	return "", fmt.Errorf("template %q not found; use --list-templates to see available templates", name)
}

// asRows converts a 2D string slice to a slice of row maps.
// Each map has column letter keys (A, B, ...), an "index" int, and
// an "_all" key with all cell values joined (for predictAccount with no query arg).
func asRows(data [][]string) []map[string]interface{} {
	rows := make([]map[string]interface{}, len(data))
	for i, row := range data {
		m := make(map[string]interface{}, len(row)+2)
		var allParts []string
		for j, cell := range row {
			var key string
			if j < 26 {
				key = string(rune('A' + j))
			} else {
				key = nextChar(string(rune('A' + j - 1)))
			}
			m[key] = cell
			allParts = append(allParts, cell)
		}
		m["index"] = i
		m["_all"] = strings.Join(allParts, " ")
		rows[i] = m
	}
	return rows
}

// buildContext builds the raymond render context for a single row.
func buildContext(row map[string]interface{}, allRows []map[string]interface{}) map[string]interface{} {
	ctx := map[string]interface{}{
		"ROW":   row,
		"SHEET": allRows,
	}
	for c := 'A'; c <= 'Z'; c++ {
		ctx[string(c)] = string(c)
	}
	return ctx
}

// parseFile routes to the appropriate parser based on file extension.
func parseFile(filePath string) ([][]string, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".csv", ".txt":
		return parseCSV(filePath)
	case ".xlsx", ".xls":
		return parseXLSX(filePath)
	case ".pdf":
		return nil, fmt.Errorf("PDF import is not supported in the CLI; use 'paisa serve' to import PDF files via the web UI")
	default:
		return nil, fmt.Errorf("unsupported file type %q; supported formats: csv, txt, xlsx, xls", ext)
	}
}

// parseCSV reads a CSV/TXT file, auto-detecting the delimiter.
func parseCSV(filePath string) ([][]string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	delim := detectDelimiter(string(content))
	r := csv.NewReader(strings.NewReader(string(content)))
	r.Comma = delim
	r.LazyQuotes = true
	r.FieldsPerRecord = -1

	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("CSV parse error: %w", err)
	}
	return filterBlankRows(records), nil
}

// detectDelimiter picks the most common delimiter candidate from the first line.
func detectDelimiter(content string) rune {
	firstLine := content
	if idx := strings.IndexByte(content, '\n'); idx >= 0 {
		firstLine = content[:idx]
	}
	candidates := []rune{',', '\t', '|', ';', '^'}
	best := ','
	bestCount := 0
	for _, c := range candidates {
		count := strings.Count(firstLine, string(c))
		if count > bestCount {
			bestCount = count
			best = c
		}
	}
	return best
}

// parseXLSX reads an XLSX file and returns the first sheet as a 2D string slice.
func parseXLSX(filePath string) ([][]string, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		// Fall back to legacy XLS parser for old binary format.
		return parseXLS(filePath)
	}
	defer f.Close()

	sheetName := f.GetSheetName(0)
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("XLSX read error: %w", err)
	}
	// excelize returns all columns from A regardless of the sheet's used range.
	// xlsx.js (used by the web UI) starts from the sheet's first used column, so
	// templates are written expecting that offset. Strip leading empty columns to match.
	rows = stripLeadingEmptyCols(rows)
	return filterBlankRows(rows), nil
}

// parseXLS reads an old binary XLS file (OLE2/BIFF) via github.com/extrame/xls.
// It applies Excel number formats to numeric cells using reflection so that values
// match what xlsx.js (the web UI's XLS library) returns.
func parseXLS(filePath string) (rows [][]string, err error) {
	wb, xlsErr := xls.Open(filePath, "utf-8")
	if xlsErr != nil {
		return nil, fmt.Errorf("XLS parse error: %w", xlsErr)
	}
	sheet := wb.GetSheet(0)
	if sheet == nil {
		return nil, fmt.Errorf("XLS file has no sheets")
	}
	for rowIdx := 0; rowIdx <= int(sheet.MaxRow); rowIdx++ {
		var cells []string
		func() {
			defer func() { recover() }() //nolint extrame/xls panics on sparse rows
			row := sheet.Row(rowIdx)
			if row == nil {
				return
			}
			for colIdx := 0; colIdx < row.LastCol(); colIdx++ {
				cell := xlsFormattedCell(wb, row, colIdx)
				// extrame/xls returns Excel date serials as "2006-01-02T15:04:05Z" — strip time.
				if len(cell) > 10 && cell[4] == '-' && cell[7] == '-' && cell[10] == 'T' {
					cell = cell[:10]
				}
				cells = append(cells, cell)
			}
		}()
		if len(cells) > 0 {
			rows = append(rows, cells)
		}
	}
	return filterBlankRows(rows), nil
}

// xlsFormattedCell returns the cell value with the Excel number format applied.
// For numeric cells, extrame/xls ignores the XF format and returns raw float precision.
// This function uses reflection+unsafe to access the cell's XF index and apply the
// corresponding format string. Falls back to row.Col(colIdx) on any failure.
func xlsFormattedCell(wb *xls.WorkBook, row *xls.Row, colIdx int) string {
	defer func() { recover() }()

	// Access unexported Row.cols via unsafe to bypass flagRO on reflect.Interface() calls.
	rv := reflect.ValueOf(row).Elem()
	colsFv := rv.FieldByName("cols")
	if !colsFv.IsValid() {
		return row.Col(colIdx)
	}
	exportedCols := reflect.NewAt(colsFv.Type(), unsafe.Pointer(colsFv.UnsafeAddr())).Elem()

	// Try direct key lookup first, then range scan for multi-column types (MulrkCol).
	serial := uint16(colIdx)
	cellFv := exportedCols.MapIndex(reflect.ValueOf(serial))
	if !cellFv.IsValid() {
		for _, k := range exportedCols.MapKeys() {
			v := exportedCols.MapIndex(k).Elem()
			if !v.IsValid() {
				continue
			}
			// Use FirstCol/LastCol via reflection to check the range.
			firstColFn := v.MethodByName("FirstCol")
			lastColFn := v.MethodByName("LastCol")
			if !firstColFn.IsValid() || !lastColFn.IsValid() {
				continue
			}
			first := uint16(firstColFn.Call(nil)[0].Uint())
			last := uint16(lastColFn.Call(nil)[0].Uint())
			if first <= serial && serial <= last {
				cellFv = exportedCols.MapIndex(k)
				break
			}
		}
		if !cellFv.IsValid() {
			return row.Col(colIdx)
		}
	}

	concrete := cellFv.Elem()
	if !concrete.IsValid() {
		return row.Col(colIdx)
	}

	// Determine XF index and numeric value from the concrete cell type.
	var xfIdx uint16
	var cellFloat float64
	switch cell := concrete.Interface().(type) {
	case *xls.NumberCol:
		xfIdx = cell.Index
		cellFloat = cell.Float
	case *xls.RkCol:
		xfIdx = cell.Xfrk.Index
		if f, err := cell.Xfrk.Rk.Float(); err == nil {
			cellFloat = f
		} else {
			return row.Col(colIdx)
		}
	case *xls.MulrkCol:
		offset := int(serial - uint16(cell.FirstCol()))
		if offset >= len(cell.Xfrks) {
			return row.Col(colIdx)
		}
		xfrk := cell.Xfrks[offset]
		xfIdx = xfrk.Index
		if f, err := xfrk.Rk.Float(); err == nil {
			cellFloat = f
		} else {
			return row.Col(colIdx)
		}
	default:
		return row.Col(colIdx)
	}

	// Look up the XF record for this cell's format number.
	xfsSlice := reflect.ValueOf(wb).Elem().FieldByName("Xfs")
	if !xfsSlice.IsValid() || int(xfIdx) >= xfsSlice.Len() {
		return row.Col(colIdx)
	}
	xfElem := xfsSlice.Index(int(xfIdx)).Elem().Elem() // Interface → Ptr → Struct
	formatField := xfElem.FieldByName("Format")
	if !formatField.IsValid() {
		return row.Col(colIdx)
	}
	fNo := uint16(formatField.Uint())

	// Date format numbers — let the library's own date handling produce the RFC3339 string.
	if fNo >= 164 || (14 <= fNo && fNo <= 22) || (27 <= fNo && fNo <= 36) || (50 <= fNo && fNo <= 58) {
		return row.Col(colIdx)
	}

	var fmtStr string
	if format, ok := wb.Formats[fNo]; ok {
		// Access unexported Format.str field via reflect.String() which works without flagRO.
		fmtStr = reflect.ValueOf(format).Elem().FieldByName("str").String()
	} else if s, ok := xlsBuiltinFormats[fNo]; ok {
		fmtStr = s
	} else {
		return row.Col(colIdx)
	}

	return applyExcelNumFmt(cellFloat, fmtStr)
}

// applyExcelNumFmt formats f using a simplified Excel number format string.
// It counts '0' and '#' characters after the first '.' to determine decimal places.
func applyExcelNumFmt(f float64, fmtStr string) string {
	dotIdx := strings.Index(fmtStr, ".")
	if dotIdx < 0 {
		return strconv.FormatFloat(f, 'f', 0, 64)
	}
	decimals := 0
	for i := dotIdx + 1; i < len(fmtStr); i++ {
		ch := fmtStr[i]
		if ch == '0' || ch == '#' {
			decimals++
		} else {
			break
		}
	}
	return strconv.FormatFloat(f, 'f', decimals, 64)
}

// xlsBuiltinFormats maps Excel built-in format numbers (0–49) to format strings.
// Only the numeric formats that affect decimal places are included.
// See https://support.microsoft.com/en-us/office/number-format-codes-5026bbd6-04bc-48cd-bf33-80f18b4eae68
var xlsBuiltinFormats = map[uint16]string{
	1: "0",
	2: "0.00",
	3: "#,##0",
	4: "#,##0.00",
	9: "0%",
	10: "0.00%",
	11: "0.00E+00",
	37: "#,##0;(#,##0)",
	38: "#,##0;[Red](#,##0)",
	39: "#,##0.00;(#,##0.00)",
	40: "#,##0.00;[Red](#,##0.00)",
	48: "##0.0E+0",
}

// stripLeadingEmptyCols removes the leading empty columns that XLSX files often
// have when the sheet range doesn't start at column A. excelize always starts
// from column A; xlsx.js (the web UI) starts from the sheet's first used column,
// so templates are written expecting that offset.
func stripLeadingEmptyCols(rows [][]string) [][]string {
	minCol := -1
	for _, row := range rows {
		for j, cell := range row {
			if strings.TrimSpace(cell) != "" {
				if minCol == -1 || j < minCol {
					minCol = j
				}
				break
			}
		}
	}
	if minCol <= 0 {
		return rows
	}
	result := make([][]string, len(rows))
	for i, row := range rows {
		if len(row) > minCol {
			result[i] = row[minCol:]
		}
	}
	return result
}

// filterBlankRows removes rows where every cell is empty or whitespace-only.
func filterBlankRows(rows [][]string) [][]string {
	var result [][]string
	for _, row := range rows {
		for _, cell := range row {
			if strings.TrimSpace(cell) != "" {
				result = append(result, row)
				break
			}
		}
	}
	return result
}
