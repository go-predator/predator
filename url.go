/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   url.go
 * @Created At:  2023-02-20 20:06:37
 * @Modified At: 2023-03-16 10:56:05
 * @Modified By: thepoy
 */

package predator

import (
	"net/url"
	"sync"
)

// urlPool is a global sync.Pool that contains *url.URL instances.
var urlPool = &sync.Pool{
	New: func() any {
		return &url.URL{}
	},
}

// AcquireURL returns an empty *url.URL instance from the urlPool.
//
// It reduces GC load by recycling the *url.URL instances.
// Release the *url.URL with ReleaseURL after it's no longer needed.
func AcquireURL() *url.URL {
	return urlPool.Get().(*url.URL)
}

// ReleaseURL releases the *url.URL acquired via AcquireURL back to the urlPool.
//
// The released *url.URL mustn't be used after releasing it to avoid data races.
func ReleaseURL(u *url.URL) {
	resetURL(u)
	urlPool.Put(u)
}

// resetURL resets the given *url.URL to its initial state.
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
