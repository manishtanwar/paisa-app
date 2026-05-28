package importer

import "strings"

// dayjsToGoFormat converts a dayjs format string to a Go time layout string.
// Longer tokens are listed first to prevent partial substitutions.
func dayjsToGoFormat(s string) string {
	return strings.NewReplacer(
		"YYYY", "2006",
		"YY", "06",
		"MMMM", "January",
		"MMM", "Jan",
		"MM", "01",
		"M", "1",
		"DD", "02",
		"D", "2",
		"HH", "15",
		"hh", "03",
		"h", "3",
		"mm", "04",
		"ss", "05",
		"A", "PM",
		"a", "pm",
	).Replace(s)
}
