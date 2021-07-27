/*
 * @Author: thepoy
 * @Email: email@example.com
 * @File Name: proxy.go
 * @Created: 2021-07-27 12:15:35
 * @Modified: 2021-07-27 12:22:07
 */

package predator

import (
	"errors"
	"net"
	"time"

	"github.com/thep0y/predator/proxy"
	"github.com/thep0y/predator/tools"
	"github.com/valyala/fasthttp"
)

func (c *Crawler) DialWithProxy() fasthttp.DialFunc {
	return c.DialWithProxyAndTimeout(0)
}

func (c *Crawler) DialWithProxyAndTimeout(timeout time.Duration) fasthttp.DialFunc {
	return func(addr string) (net.Conn, error) {
		proxyAddr := tools.Shuffle(c.proxyURLPool)[0]
		if proxyAddr[:7] == "http://" || proxyAddr[:8] == "https://" {
			return proxy.HttpProxy(proxyAddr, addr, timeout)
		} else if proxyAddr[:9] == "socks5://" {
			return proxy.Socks5Proxy(proxyAddr, addr)
		} else {
			panic(errors.New("only support http and socks5 protocol"))
		}
	}
}
