// Package csvsafe protects text fields exported to spreadsheet-readable CSV.
package csvsafe

import (
	"strings"
	"unicode"
)

func Text(value string) string {
	trimmed := strings.TrimLeftFunc(value, unicode.IsSpace)
	if len(value) > 0 && strings.ContainsRune("\t\r\n", rune(value[0])) || len(trimmed) > 0 && strings.ContainsRune("=+-@", rune(trimmed[0])) {
		return "'" + value
	}
	return value
}
