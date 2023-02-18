/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   socks5.go
 * @Created At:  2021-07-23 09:22:36
 * @Modified At: 2023-02-18 22:32:01
 * @Modified By: thepoy
 */

package proxy

import (
	"net"
	"net/url"

	"github.com/valyala/fasthttp"
	netProxy "golang.org/x/net/proxy"
)

func Socks5ProxyDialer(proxyAddr string) fasthttp.DialFunc {
	var (
		u      *url.URL
		err    error
		dialer netProxy.Dialer
	)

	if proxyAddr == "" {
		err = ProxyErr{
			Code: ErrInvalidProxyCode,
			Args: map[string]string{"addr": proxyAddr},
			Msg:  "ip and port cannot be empty",
		}
	} else {
		if u, err = url.Parse(proxyAddr); err == nil {
			dialer, err = netProxy.FromURL(u, netProxy.Direct)
		}
	}

	// It would be nice if we could return the error here. But we can't
	// change our API so just keep returning it in the returned Dial function.
	// Besides the implementation of proxy.SOCKS5() at the time of writing this
	// will always return nil as error.

	return func(addr string) (net.Conn, error) {
		if err != nil {
			return nil, err
		}
		return dialer.Dial("tcp", addr)
	}
}
