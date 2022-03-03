/*
 * @Author:    thepoy
 * @Email:     thepoy@163.com
 * @File Name: options.go
 * @Created:   2021-07-23 08:58:31
 * @Modified:  2022-03-03 13:20:15
 */

package predator

import (
	"crypto/tls"
	"strings"
	"sync"

	"github.com/go-predator/cache"
	"github.com/go-predator/predator/log"
)

type CrawlerOption func(*Crawler)

// SkipVerification will skip verifying the certificate when
// you access the `https` protocol
func SkipVerification() CrawlerOption {
	return func(c *Crawler) {
		c.client.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	}
}

func WithLogger(logger *log.Logger) CrawlerOption {
	if logger == nil {
		logger = log.NewLogger(log.WARNING, log.ToConsole(), 1)
	}

	return func(c *Crawler) {
		c.log = logger
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

func WithRawCookie(cookie string) CrawlerOption {
	cookies := make(map[string]string)
	cookieSlice := strings.Split(cookie, "; ")
	for _, c := range cookieSlice {
		temp := strings.SplitN(c, "=", 2)
		cookies[temp[0]] = temp[1]
	}
	return WithCookies(cookies)
}

func WithCookies(cookies map[string]string) CrawlerOption {
	return func(c *Crawler) {
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

type RetryCondition func(r Response) bool

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
			cacheCondition = func(r Response) bool {
				return r.StatusCode/100 == 2
			}
		}
		c.cacheCondition = cacheCondition
		if len(cacheFileds) > 0 {
			c.cacheFields = cacheFileds
		}
	}
}

// WithDefaultCache 默认缓存为 sqlite3，不压缩
func WithDefaultCache() CrawlerOption {
	return WithCache(&cache.SQLiteCache{}, false, nil)
}

func EnableIPv6() CrawlerOption {
	return func(c *Crawler) {
		c.client.DialDualStack = true
	}
}
