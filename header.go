/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   header.go
 * @Created At:  2023-02-19 14:11:47
 * @Modified At: 2023-02-27 11:48:35
 * @Modified By: thepoy
 */

package predator

import (
	"net/http"
)

func NewHeader(header map[string]string) http.Header {
	h := make(http.Header)
	for k, v := range header {
		h.Set(k, v)
	}

	return h
}
