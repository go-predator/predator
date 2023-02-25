/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   cookie.go
 * @Created At:  2023-02-20 21:35:40
 * @Modified At: 2023-02-20 21:43:04
 * @Modified By: thepoy
 */

package predator

import (
	"strings"

	"golang.org/x/net/http/httpguts"
)

func isNotToken(r rune) bool {
	return !httpguts.IsTokenRune(r)
}

func isCookieNameValid(raw string) bool {
	if raw == "" {
		return false
	}
	return strings.IndexFunc(raw, isNotToken) < 0
}

func validCookieValueByte(b byte) bool {
	return 0x20 <= b && b < 0x7f && b != '"' && b != ';' && b != '\\'
}

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
