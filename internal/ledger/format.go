package ledger

import (
	"regexp"
	"strings"

	"github.com/ananthakumaran/paisa/internal/config"
)

var (
	dateLineRe     = regexp.MustCompile(`^\d{4}[/-]\d{2}[/-]\d{2}`)
	periodicLineRe = regexp.MustCompile(`^[~=]`)
	// Full posting: indent + account (lazy, stops at first double-space) + 2+space sep + prefix + amount + suffix
	// Go doesn't support lookaheads, so we use lazy matching with the 2+-space separator
	fullPostingRe = regexp.MustCompile(`^([ \t]+)((?:[*!][ \t]+)?[^; \t][^;\t]*?)([ \t]+)([^;]*?)([+-]?[.,0-9]+)(.*)$`)
	// Partial posting: indent + account only (no amount after)
	partialPostingRe = regexp.MustCompile(`^([ \t]+)((?:[*!][ \t]+)?[^; \t][^;\t]*)$`)
)

// FormatContent formats ledger file content. It is the Go equivalent of the
// TypeScript format() function in src/lib/journal.ts.
func FormatContent(text string) string {
	amountAlignmentColumn := config.GetConfig().AmountAlignmentColumn
	if amountAlignmentColumn == 0 {
		amountAlignmentColumn = 52
	}

	lines := strings.Split(text, "\n")
	result := make([]string, len(lines))
	inTransaction := false

	for i, line := range lines {
		result[i], inTransaction = formatLine(line, inTransaction, amountAlignmentColumn)
	}

	return strings.Join(result, "\n")
}

func formatLine(line string, inTransaction bool, amountAlignmentColumn int) (string, bool) {
	// Transaction start: line begins with a date or periodic marker
	if dateLineRe.MatchString(line) || periodicLineRe.MatchString(line) {
		return line, true
	}

	// Empty or non-indented line ends the current transaction
	if strings.TrimSpace(line) == "" || (len(line) > 0 && line[0] != ' ' && line[0] != '\t') {
		return line, false
	}

	if !inTransaction {
		return line, false
	}

	// Try full posting match (account + separator + amount)
	if m := fullPostingRe.FindStringSubmatch(line); m != nil {
		account := m[2]
		prefix := m[4]
		amount := m[5]
		suffix := m[6]

		totalLen := len(account) + len(prefix) + len(amount)
		if totalLen <= amountAlignmentColumn-6 {
			spaces := amountAlignmentColumn - 4 - len(account) - len(prefix) - len(amount)
			return "    " + account + strings.Repeat(" ", spaces) + prefix + amount + suffix, true
		}
	}

	// Try partial posting match (account only, no amount)
	if m := partialPostingRe.FindStringSubmatch(line); m != nil {
		account := strings.TrimRight(m[2], " \t")
		return "    " + account, true
	}

	return line, inTransaction
}
