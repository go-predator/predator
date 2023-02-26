/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   header.go
 * @Created At:  2023-02-19 14:11:47
 * @Modified At: 2023-02-26 13:05:04
 * @Modified By: thepoy
 */

package predator

import (
	"net/http"
	"sync"
)

var headerPool = &sync.Pool{
	New: func() any {
		return make(http.Header)
	},
}

func NewHeader(header map[string]string) http.Header {
	h := make(http.Header)
	for k, v := range header {
		h.Set(k, v)
	}

	return h
}

func acquireHeader() http.Header {
	return headerPool.Get().(http.Header)
}

func releaseHeader(header http.Header) {
	ResetMap(header)
	headerPool.Put(header)
}
