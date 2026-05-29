package importer

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/ananthakumaran/paisa/internal/prediction"
	"github.com/aymerick/raymond"
	"github.com/dlclark/regexp2"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

var stopWords = map[string]bool{
	"": true, "fof": true, "growth": true, "direct": true, "plan": true, "the": true,
}

// scrubAmount strips non-numeric characters, handling (amount) → -amount notation.
func scrubAmount(str string) string {
	str = strings.TrimSpace(str)
	if strings.HasPrefix(str, "(") && strings.HasSuffix(str, ")") {
		str = "-" + str[1:len(str)-1]
	}
	var b strings.Builder
	for _, r := range str {
		if unicode.IsDigit(r) || r == '-' || r == '.' {
			b.WriteRune(r)
		}
	}
	result := b.String()
	if result == "" || result == "-" {
		return ""
	}
	if _, err := strconv.ParseFloat(result, 64); err != nil {
		return ""
	}
	return result
}

// parseAmount converts various numeric types and strings to float64.
func parseAmount(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int32:
		return float64(val)
	case int64:
		return float64(val)
	case raymond.SafeString:
		n, _ := strconv.ParseFloat(scrubAmount(string(val)), 64)
		return n
	case string:
		n, _ := strconv.ParseFloat(scrubAmount(val), 64)
		return n
	}
	if v != nil {
		n, _ := strconv.ParseFloat(scrubAmount(fmt.Sprintf("%v", v)), 64)
		return n
	}
	return 0
}

// nextChar increments a spreadsheet column name (A→B, Z→AA).
func nextChar(key string) string {
	if key == "Z" {
		return "AA"
	}
	last := key[len(key)-1]
	butlast := key[:len(key)-1]
	if last == 'Z' {
		return nextChar(butlast) + "A"
	}
	return butlast + string(rune(last+1))
}

// rowFromCtx extracts the ROW map from the raymond helper options context.
func rowFromCtx(opts *raymond.Options) map[string]interface{} {
	ctx := opts.Ctx()
	if m, ok := ctx.(map[string]interface{}); ok {
		if row, ok := m["ROW"].(map[string]interface{}); ok {
			return row
		}
	}
	return nil
}

// sheetFromCtx extracts the SHEET slice from the raymond helper options context.
func sheetFromCtx(opts *raymond.Options) []map[string]interface{} {
	ctx := opts.Ctx()
	if m, ok := ctx.(map[string]interface{}); ok {
		if sheet, ok := m["SHEET"].([]map[string]interface{}); ok {
			return sheet
		}
	}
	return nil
}

func hashInt(opts *raymond.Options, key string) int {
	switch v := opts.HashProp(key).(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		n, _ := strconv.Atoi(v)
		return n
	}
	return 0
}

// buildHelpers returns all template helpers for raymond, wired to db and noPredict.
func buildHelpers(db *gorm.DB, noPredict bool) map[string]interface{} {
	return map[string]interface{}{
		// Boolean logic
		"eq":  func(a, b interface{}) bool { return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b) },
		"ne":  func(a, b interface{}) bool { return fmt.Sprintf("%v", a) != fmt.Sprintf("%v", b) },
		"not": func(v interface{}) bool { return !raymond.IsTrue(v) },
		"and": func(a, b interface{}) interface{} {
			if !raymond.IsTrue(a) {
				return false
			}
			if !raymond.IsTrue(b) {
				return false
			}
			return true
		},
		"or": func(a, b interface{}) interface{} {
			if raymond.IsTrue(a) {
				return a
			}
			return b
		},

		// Numeric comparisons
		"gte": func(a, b interface{}) bool { return parseAmount(a) >= parseAmount(b) },
		"gt":  func(a, b interface{}) bool { return parseAmount(a) > parseAmount(b) },
		"lte": func(a, b interface{}) bool { return parseAmount(a) <= parseAmount(b) },
		"lt":  func(a, b interface{}) bool { return parseAmount(a) < parseAmount(b) },

		"negate": func(v interface{}) raymond.SafeString {
			n := parseAmount(v) * -1
			return raymond.SafeString(strconv.FormatFloat(n, 'f', -1, 64))
		},
		"round": func(val interface{}, opts *raymond.Options) raymond.SafeString {
			n := parseAmount(val)
			precision := hashInt(opts, "precision")
			factor := math.Pow(10, float64(precision))
			result := math.Round(n*factor) / factor
			return raymond.SafeString(strconv.FormatFloat(result, 'f', -1, 64))
		},

		// Date helpers
		"isDate": func(str, format interface{}) bool {
			if str == nil {
				return false
			}
			s := strings.TrimSpace(fmt.Sprintf("%v", str))
			goLayout := dayjsToGoFormat(fmt.Sprintf("%v", format))
			_, err := time.Parse(goLayout, s)
			return err == nil
		},
		"date": func(str, format interface{}) raymond.SafeString {
			s := strings.TrimSpace(fmt.Sprintf("%v", str))
			goLayout := dayjsToGoFormat(fmt.Sprintf("%v", format))
			t, err := time.Parse(goLayout, s)
			if err != nil {
				return raymond.SafeString(s)
			}
			return raymond.SafeString(t.Format("2006/01/02"))
		},

		// String helpers
		"isBlank": func(str interface{}) bool {
			if str == nil {
				return true
			}
			return strings.TrimSpace(fmt.Sprintf("%v", str)) == ""
		},
		"trim":        func(s interface{}) raymond.SafeString { return raymond.SafeString(strings.TrimSpace(fmt.Sprintf("%v", s))) },
		"replace":     func(str, search, repl interface{}) raymond.SafeString { return raymond.SafeString(strings.ReplaceAll(fmt.Sprintf("%v", str), fmt.Sprintf("%v", search), fmt.Sprintf("%v", repl))) },
		"toLowerCase": func(s interface{}) raymond.SafeString { return raymond.SafeString(strings.ToLower(fmt.Sprintf("%v", s))) },
		"toUpperCase": func(s interface{}) raymond.SafeString { return raymond.SafeString(strings.ToUpper(fmt.Sprintf("%v", s))) },
		"capitalize": func(s interface{}) raymond.SafeString {
			str := fmt.Sprintf("%v", s)
			if str == "" {
				return raymond.SafeString("")
			}
			runes := []rune(strings.ToLower(str))
			runes[0] = unicode.ToUpper(runes[0])
			return raymond.SafeString(string(runes))
		},
		"acronym": func(s interface{}) raymond.SafeString {
			str := fmt.Sprintf("%v", s)
			var cleaned strings.Builder
			for _, r := range str {
				if unicode.IsLetter(r) || unicode.IsSpace(r) {
					cleaned.WriteRune(r)
				}
			}
			words := strings.Fields(cleaned.String())
			var result strings.Builder
			for _, w := range words {
				lower := strings.ToLower(w)
				if !stopWords[lower] && len(w) > 0 {
					result.WriteRune(unicode.ToUpper(rune(w[0])))
				}
			}
			return raymond.SafeString(result.String())
		},

		// Amount helper: {{amount ROW.E}} or {{amount ROW.D default="0"}}
		"amount": func(str interface{}, opts *raymond.Options) raymond.SafeString {
			a := ""
			if str != nil {
				a = scrubAmount(fmt.Sprintf("%v", str))
			}
			if a != "" {
				return raymond.SafeString(a)
			}
			return raymond.SafeString(opts.HashStr("default"))
		},

		// Regexp helpers
		"regexpTest": func(str, pattern interface{}) bool {
			s := fmt.Sprintf("%v", str)
			p := fmt.Sprintf("%v", pattern)
			re, err := regexp2.Compile(p, 0)
			if err != nil {
				log.Warnf("regexpTest: invalid pattern %q: %v", p, err)
				return false
			}
			m, _ := re.MatchString(s)
			return m
		},
		"regexpMatch": func(str, pattern interface{}, opts *raymond.Options) interface{} {
			s := fmt.Sprintf("%v", str)
			re, err := regexp2.Compile(fmt.Sprintf("%v", pattern), 0)
			if err != nil {
				return nil
			}
			m, err := re.FindStringMatch(s)
			if err != nil || m == nil {
				return nil
			}
			group := hashInt(opts, "group")
			if group >= m.GroupCount() {
				return nil
			}
			return raymond.SafeString(m.GroupByNumber(group).String())
		},
		// match: key is return value, value is regexp pattern; keys sorted for determinism
		"match": func(str interface{}, opts *raymond.Options) interface{} {
			s := fmt.Sprintf("%v", str)
			hash := opts.Hash()
			keys := make([]string, 0, len(hash))
			for k := range hash {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, value := range keys {
				re, err := regexp2.Compile(fmt.Sprintf("%v", hash[value]), 0)
				if err != nil {
					continue
				}
				if matched, _ := re.MatchString(s); matched {
					return raymond.SafeString(value)
				}
			}
			return nil
		},

		// Spreadsheet helpers (need context access via opts.Ctx())
		"textRange": func(fromCol, toCol interface{}, opts *raymond.Options) raymond.SafeString {
			from := fmt.Sprintf("%v", fromCol)
			to := fmt.Sprintf("%v", toCol)
			sep := " "
			if s := opts.HashStr("separator"); s != "" {
				sep = s
			}
			row := rowFromCtx(opts)
			if row == nil {
				return raymond.SafeString("")
			}
			var cells []string
			current := from
			for i := 0; i < 1000; i++ {
				cells = append(cells, fmt.Sprintf("%v", row[current]))
				if current == to {
					break
				}
				current = nextChar(current)
			}
			return raymond.SafeString(strings.Join(cells, sep))
		},
		"findAbove": func(col interface{}, opts *raymond.Options) interface{} {
			colStr := fmt.Sprintf("%v", col)
			pattern := opts.HashStr("regexp")
			if pattern == "" {
				pattern = ".+"
			}
			re, err := regexp2.Compile(pattern, 0)
			if err != nil {
				return nil
			}
			row := rowFromCtx(opts)
			sheet := sheetFromCtx(opts)
			if row == nil || sheet == nil {
				return nil
			}
			idx, _ := row["index"].(int)
			group := hashInt(opts, "group")
			for i := idx - 1; i >= 0; i-- {
				cell := fmt.Sprintf("%v", sheet[i][colStr])
				m, err := re.FindStringMatch(cell)
				if err == nil && m != nil {
					if group > 0 && group < m.GroupCount() {
						return raymond.SafeString(m.GroupByNumber(group).String())
					}
					return raymond.SafeString(cell)
				}
			}
			return nil
		},
		"findBelow": func(col interface{}, opts *raymond.Options) interface{} {
			colStr := fmt.Sprintf("%v", col)
			pattern := opts.HashStr("regexp")
			if pattern == "" {
				pattern = ".+"
			}
			re, err := regexp2.Compile(pattern, 0)
			if err != nil {
				return nil
			}
			row := rowFromCtx(opts)
			sheet := sheetFromCtx(opts)
			if row == nil || sheet == nil {
				return nil
			}
			idx, _ := row["index"].(int)
			group := hashInt(opts, "group")
			for i := idx + 1; i < len(sheet); i++ {
				cell := fmt.Sprintf("%v", sheet[i][colStr])
				m, err := re.FindStringMatch(cell)
				if err == nil && m != nil {
					if group > 0 && group < m.GroupCount() {
						return raymond.SafeString(m.GroupByNumber(group).String())
					}
					return raymond.SafeString(cell)
				}
			}
			return nil
		},

		// predictAccount: always receives 1 positional arg (preprocessed by importer).
		// Templates with no positional arg ({{predictAccount prefix=...}}) are
		// preprocessed to inject ROW._all as the query.
		"predictAccount": func(query interface{}, opts *raymond.Options) raymond.SafeString {
			prefix := opts.HashStr("prefix")
			if noPredict || db == nil {
				return raymond.SafeString(fallbackAccount(prefix))
			}
			q := ""
			if query != nil {
				q = fmt.Sprintf("%v", query)
			}
			return raymond.SafeString(prediction.PredictAccount(db, q, prefix))
		},
	}
}

func fallbackAccount(prefix string) string {
	if strings.HasSuffix(prefix, ":") {
		return prefix + "Unknown"
	}
	if prefix == "" {
		return "Unknown"
	}
	return prefix + ":Unknown"
}
