/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   header.go
 * @Created At:  2023-02-19 14:11:47
 * @Modified At: 2023-02-20 21:23:10
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

func acquireHeader() http.Header {
	return headerPool.Get().(http.Header)
}

func releaseHeader(header http.Header) {
	ResetMap(header)
	headerPool.Put(header)
}
