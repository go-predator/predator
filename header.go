/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   header.go
 * @Created At:  2023-02-19 14:11:47
 * @Modified At: 2023-03-16 11:09:13
 * @Modified By: thepoy
 */

package predator

import (
	"net/http"
	"sync"
)

var headerPool = sync.Pool{
	New: func() interface{} {
		return make(http.Header)
	},
}

// NewHeader returns a new http.Header with the values in the provided map.
//
// The map is assumed to have string keys and values.
func NewHeader(header map[string]string) http.Header {
	h := headerPool.Get().(http.Header)

	for k := range h {
		delete(h, k)
	}

	for k, v := range header {
		h.Set(k, v)
	}

	return h
}

// ReleaseHeader releases the map used by NewHeader back to the pool.
func ReleaseHeader(h http.Header) {
	headerPool.Put(h)
}
