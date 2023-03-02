/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   options.go
 * @Created At:  2021-07-23 08:58:31
 * @Modified At: 2023-03-02 09:38:43
 * @Modified By: thepoy
 */

package predator

import (
	"crypto/tls"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/go-predator/log"
)

type CrawlerOption func(*Crawler)

// SkipVerification will skip verifying the certificate when
// you access the `https` protocol
func SkipVerification() CrawlerOption {
	return func(c *Crawler) {
		c.client.Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
}

func WithLogger(logger *log.Logger) CrawlerOption {
	if logger == nil {
		logger = log.NewLogger(log.WARNING, log.ToConsole(), 2)
	}

	return func(c *Crawler) {
		c.log = log.NewLogger(log.Level(logger.L.GetLevel()), logger.Out(), 2)
	}
}

func WithConsoleLogger(level log.Level) CrawlerOption {
	return func(c *Crawler) {
		c.log = log.NewLogger(level, log.ToConsole(), 2)
	}
}

func WithFileLogger(level log.Level, filename string) CrawlerOption {
	return func(c *Crawler) {
		c.log = log.NewLogger(level, log.MustToFile(filename, -1), 2)
	}
}

func WithConsoleAndFileLogger(level log.Level, filename string) CrawlerOption {
	return func(c *Crawler) {
		c.log = log.NewLogger(level, log.MustToConsoleAndFile(filename, -1), 2)
	}
}

func WithDefaultLogger() CrawlerOption {
	return WithLogger(nil)
}

func WithUserAgent(ua string) CrawlerOption {
	return func(c *Crawler) {
		c.UserAgent = ua
	}
}

func RecordRemoteAddr() CrawlerOption {
	return func(c *Crawler) {
		c.recordRemoteAddr = true
	}
}

func WithRawCookie(cookies string) CrawlerOption {
	return func(c *Crawler) {
		c.rawCookies = cookies
	}
}

func WithCookies(cookies map[string]string) CrawlerOption {
	return func(c *Crawler) {
		for k, v := range cookies {
			v, ok := parseCookieValue(v, true)
			if !ok {
				continue
			}

			cookies[k] = v
		}

		c.cookies = cookies
	}
}

// WithConcurrency 使用并发，参数为要创建的协程池数量
func WithConcurrency(count uint64, blockPanic bool) CrawlerOption {
	return func(c *Crawler) {
		p, err := NewPool(count)
		if err != nil {
			panic(err)
		}
		p.blockPanic = blockPanic

		c.goPool = p
		c.wg = new(sync.WaitGroup)
	}
}

type RetryCondition func(r *Response) bool

// WithRetry 请求失败时重试多少次，什么条件的响应是请求失败
func WithRetry(count uint32, cond RetryCondition) CrawlerOption {
	return func(c *Crawler) {
		c.retryCount = count
		c.retryCondition = cond
	}
}

// WithProxy 使用一个代理
func WithProxy(proxyURL string) CrawlerOption {
	return func(c *Crawler) {
		c.proxyURLPool = []string{proxyURL}
	}
}

// WithProxyPool 使用一个代理池
func WithProxyPool(proxyURLs []string) CrawlerOption {
	return func(c *Crawler) {
		c.proxyURLPool = proxyURLs
	}
}

// WithTimeout 使用超时控制。
//
// 此处的超时时间将作用于整个 request 请求，如果你使用了代理，
// 客户端与代理服务器建立连接的时间也会被算在内，这是 http 标准
// 库的问题。
//
// 同时，这也意味着，一旦发生超时错误，无法判断这个错误是由代理
// 引发还是由代理向服务器发送请求引发的，需要用户自行判断。
func WithTimeout(timeout time.Duration) CrawlerOption {
	return func(c *Crawler) {
		c.timeout = timeout
	}
}

// WithComplementProxyPool replenishes the proxy pool when the proxy pool is empty
func WithComplementProxyPool(f ComplementProxyPool) CrawlerOption {
	return func(c *Crawler) {
		c.complementProxyPool = f
	}
}

// WithCache 使用缓存，可以选择是否压缩缓存的响应。
// 使用缓存时，如果发出的是 POST 请求，最好传入能
// 代表请求体的唯一性的缓存字段，可以是零个、一个或多个。
//
// 注意：当不传入缓存字段时，将会默认采用整个请求体作为
// 缓存标识，但由于 map 无序，同一个请求体生成的 key 很
// 难保证相同，所以可能会有同一个请求缓存多次，或者无法
// 从缓存中读取已请求过的请求的响应的情况出现。
func WithCache(cc Cache, compressed bool, cacheCondition CacheCondition, cacheFileds ...CacheField) CrawlerOption {
	return func(c *Crawler) {
		cc.Compressed(compressed)
		err := cc.Init()
		if err != nil {
			panic(err)
		}
		c.cache = cc
		if cacheCondition == nil {
			cacheCondition = func(r *Response) bool {
				return r.StatusCode/100 == 2
			}
		}
		c.cacheCondition = cacheCondition
		if len(cacheFileds) > 0 {
			c.cacheFields = cacheFileds
		}
	}
}

func DisableIPv6() CrawlerOption {
	return func(c *Crawler) {
		c.client.Transport.(*http.Transport).DialContext = (&net.Dialer{
			Timeout:       30 * time.Second,
			KeepAlive:     30 * time.Second,
			FallbackDelay: -1,
		}).DialContext
	}
}
