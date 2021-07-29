/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: socks5.go
 * @Created: 2021-07-23 09:22:36
 * @Modified: 2021-07-29 14:12:27
 */

package proxy

import (
	"net"
	"net/url"
	"strings"

	netProxy "golang.org/x/net/proxy"
)

func Socks5Proxy(proxyAddr string, addr string) (net.Conn, error) {
	proxyAddr = strings.Split(proxyAddr, "//")[1]
	var (
		u      *url.URL
		err    error
		dialer netProxy.Dialer
	)
	if u, err = url.Parse(proxyAddr); err == nil {
		dialer, err = netProxy.FromURL(u, netProxy.Direct)
	}
	// It would be nice if we could return the error here. But we can't
	// change our API so just keep returning it in the returned Dial function.
	// Besides the implementation of proxy.SOCKS5() at the time of writing this
	// will always return nil as error.

	if err != nil {
		return nil, err
	}

	return dialer.Dial("tcp", addr)
}
