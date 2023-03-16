/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   cookie.go
 * @Created At:  2023-02-20 21:35:40
 * @Modified At: 2023-03-16 09:41:45
 * @Modified By: thepoy
 */

package predator

import (
	"strings"

	"golang.org/x/net/http/httpguts"
)

// isNotToken returns true if the given rune is not a valid token character.
func isNotToken(r rune) bool {
	return !httpguts.IsTokenRune(r)
}

// isCookieNameValid returns true if the given string can be a valid cookie name.
// A cookie name must be non-empty and contain only valid token characters.
func isCookieNameValid(raw string) bool {
	if raw == "" {
		return false
	}
	return strings.IndexFunc(raw, isNotToken) < 0
}

// validCookieValueByte returns true if the given byte is a valid character for a cookie value.
// A valid character must be printable, not a double quote, a semicolon or a backslash.
func validCookieValueByte(b byte) bool {
	return 0x20 <= b && b < 0x7f && b != '"' && b != ';' && b != '\\'
}

// parseCookieValue parses the cookie value from a raw string and returns it along with a boolean
// indicating if the value is valid. If allowDoubleQuote is true, the function will strip the quotes
// from the raw string before parsing.
func parseCookieValue(raw string, allowDoubleQuote bool) (string, bool) {
	// Strip the quotes, if present.
	if allowDoubleQuote && len(raw) > 1 && raw[0] == '"' && raw[len(raw)-1] == '"' {
		raw = raw[1 : len(raw)-1]
	}
	for i := 0; i < len(raw); i++ {
		if !validCookieValueByte(raw[i]) {
			return "", false
		}
	}
	return raw, true
}
