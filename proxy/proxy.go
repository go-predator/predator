/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   proxy.go
 * @Created At:  2023-02-25 19:55:29
 * @Modified At: 2023-02-25 20:16:45
 * @Modified By: thepoy
 */

package proxy

import (
	"net/http"
	"net/url"
)

type ProxyFunc func(*http.Request) (*url.URL, error)

func Proxy(r *http.Request, proxyAddr string) ProxyFunc {
	var (
		u   *url.URL
		err error
	)
	u, err = url.Parse(proxyAddr)

	return func(r *http.Request) (*url.URL, error) {
		if err != nil {
			return nil, ProxyErr{
				Proxy: proxyAddr,
				Err:   ErrInvalidProxy,
			}
		}

		return u, nil
	}
}