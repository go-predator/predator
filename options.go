/*
 * @Author: Ryan Wong
 * @Email: thepoy@163.com
 * @File Name: options.go
 * @Created: 2021-07-23 08:58:31
 * @Modified: 2021-07-25 10:15:08
 */

package predator

import (
	"strings"

	"github.com/thep0y/go-logger/log"
)

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

// WithConcurrent 使用多少个协程，用于创建协程池
func WithConcurrent(count uint) CrawlerOption {
	return func(c *Crawler) {
		c.goCount = count
	}
}

// WithRetry 请求失败时重试多少次
func WithRetry(count uint) CrawlerOption {
	return func(c *Crawler) {
		c.retryCount = count
	}
}

// WithProxy 使用一个代理
func WithProxy(proxyURL string) CrawlerOption {
	return func(c *Crawler) {
		if proxyURL[:5] == "socks" {
			// TODO: 这个警告暂时这样处理
			log.Warn("may not support socks proxy")
		}
		if strings.Contains(proxyURL, "//") {
			proxyURL = strings.Split(proxyURL, "//")[1]
		}
		c.proxyURL = proxyURL
	}
}

// WithProxyPool 使用一个代理池
func WithProxyPool(proxyURLs []string) CrawlerOption {
	return func(c *Crawler) {
		c.proxyURLPool = proxyURLs
	}
}
