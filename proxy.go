/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: proxy.go
 * @Created: 2021-07-27 12:15:35
 * @Modified:  2021-11-08 08:39:49
 */

package predator

import (
	"time"

	"github.com/go-predator/predator/proxy"
	"github.com/valyala/fasthttp"
)

// 可以从一些代理网站的 api 中请求指定数量的代理 ip
type AcquireProxies func(n int) []string

func (c *Crawler) ProxyDialerWithTimeout(proxyAddr string, timeout time.Duration) fasthttp.DialFunc {
	if proxyAddr[:7] == "http://" || proxyAddr[:8] == "https://" {
		return proxy.HttpProxyDialerWithTimeout(proxyAddr, timeout)
	} else if proxyAddr[:9] == "socks5://" {
		return proxy.Socks5ProxyDialer(proxyAddr)
	} else {
		err := proxy.ProxyErr{
			Code: proxy.ErrUnknownProtocolCode,
			Args: map[string]string{
				"proxy_addr": proxyAddr,
			},
			Msg: "only support http and socks5 protocol, but the incoming proxy address uses an unknown protocol",
		}
		if c.log != nil {
			c.log.Fatal().
				Caller().
				Err(err).
				Str("proxy", proxyAddr).
				Send()
		} else {
			panic(err)
		}
		return nil
	}
}
