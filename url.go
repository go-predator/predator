/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   url.go
 * @Created At:  2023-02-20 20:06:37
 * @Modified At: 2023-02-20 21:11:54
 * @Modified By: thepoy
 */

package predator

import (
	"net/url"
	"sync"
)

var urlPool = &sync.Pool{
	New: func() any {
		return &url.URL{}
	},
}

// AcquireURL returns an empty URL instance from the pool.
//
// Release the URL with ReleaseURL after the URL is no longer needed.
// This allows reducing GC load.
func AcquireURL() *url.URL {
	return urlPool.Get().(*url.URL)
}

// ReleaseURL releases the URL acquired via AcquireURL.
//
// The released URL mustn't be used after releasing it, otherwise data races
// may occur.
func ReleaseURL(u *url.URL) {
	resetURL(u)
	urlPool.Put(u)
}

func resetURL(u *url.URL) {
	u.Scheme = ""
	u.Opaque = ""
	u.User = nil
	u.Host = ""
	u.Path = ""
	u.RawPath = ""
	u.OmitHost = false
	u.ForceQuery = false
	u.RawQuery = ""
	u.Fragment = ""
	u.RawFragment = ""
}
