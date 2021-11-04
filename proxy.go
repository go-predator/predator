/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: proxy.go
 * @Created: 2021-07-27 12:15:35
 * @Modified: 2021-11-04 19:53:58
 */

package predator

import (
	"net"
	"time"

	"github.com/go-predator/predator/proxy"
	"github.com/go-predator/predator/tools"
	"github.com/valyala/fasthttp"
)

// 可以从一些代理网站的 api 中请求指定数量的代理 ip
type AcquireProxies func(n int) []string

func (c *Crawler) DialWithProxy() fasthttp.DialFunc {
	return c.DialWithProxyAndTimeout(0)
}

func (c *Crawler) DialWithProxyAndTimeout(timeout time.Duration) fasthttp.DialFunc {
	return func(addr string) (net.Conn, error) {
		proxyAddr := tools.Shuffle(c.proxyURLPool)[0]

		if c.log != nil {
			c.log.Debug().
				Str("proxy_ip", proxyAddr).
				Msg("an proxy ip is selected from the proxy pool")
		}

		if proxyAddr[:7] == "http://" || proxyAddr[:8] == "https://" {
			return proxy.HttpProxy(proxyAddr, addr, timeout)
		} else if proxyAddr[:9] == "socks5://" {
			return proxy.Socks5Proxy(proxyAddr, addr)
		} else {
			if c.log != nil {
				c.log.Fatal().
					Caller().
					Err(ErrUnknownProtocol).
					Str("proxy", proxyAddr).
					Send()
			} else {
				panic(ErrUnknownProtocol)
			}
			return nil, nil
		}
	}
}
