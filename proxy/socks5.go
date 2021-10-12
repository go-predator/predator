/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: socks5.go
 * @Created: 2021-07-23 09:22:36
 * @Modified: 2021-10-12 09:44:15
 */

package proxy

import (
	"errors"
	"net"
	"net/url"

	netProxy "golang.org/x/net/proxy"
)

var AddrIsNULL = errors.New("ip and port cannot be empty")

func Socks5Proxy(proxyAddr string, addr string) (net.Conn, error) {
	if proxyAddr == "" {
		panic(AddrIsNULL)
	}
	var (
		u      *url.URL
		err    error
		dialer netProxy.Dialer
	)
	if u, err = url.Parse(proxyAddr); err == nil {
		dialer, err = netProxy.FromURL(u, netProxy.Direct)
		if err != nil {
			panic(err)
		}
	} else {
		panic(err)
	}

	// It would be nice if we could return the error here. But we can't
	// change our API so just keep returning it in the returned Dial function.
	// Besides the implementation of proxy.SOCKS5() at the time of writing this
	// will always return nil as error.

	return dialer.Dial("tcp", addr)
}
