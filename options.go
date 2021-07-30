/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: options.go (c) 2021
 * @Created: 2021-07-23 08:58:31
 * @Modified: 2021-07-30 18:10:38
 */

package predator

import "github.com/thep0y/predator/cache"

type CrawlerOption func(*Crawler)

func WithUserAgent(ua string) CrawlerOption {
	return func(c *Crawler) {
		c.UserAgent = ua
	}
}

func WithCookies(cookies map[string]string) CrawlerOption {
	return func(c *Crawler) {
		c.cookies = cookies
	}
}

// WithConcurrency 使用并发，参数为要创建的协程池数量
func WithConcurrency(count uint64) CrawlerOption {
	return func(c *Crawler) {
		p, err := NewPool(count)
		if err != nil {
			panic(err)
		}
		c.goPool = p
	}
}

type RetryConditions func(r Response) bool

// WithRetry 请求失败时重试多少次，什么条件的响应是请求失败
func WithRetry(count uint32, cond RetryConditions) CrawlerOption {
	return func(c *Crawler) {
		c.retryCount = count
		c.retryConditions = cond
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

// WithCache 使用缓存，可以选择是否压缩缓存的响应
func WithCache(cc cache.Cache, compressed bool) CrawlerOption {
	return func(c *Crawler) {
		if cc == nil {
			cc = &cache.SqliteCache{}
		}
		cc.Compressed(compressed)
		err := cc.Init()
		if err != nil {
			panic(err)
		}
		c.cache = cc
	}
}
