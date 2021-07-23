/*
 * @Author: Ryan Wong
 * @Email: thepoy@163.com
 * @File Name: options.go
 * @Created: 2021-07-23 08:58:31
 * @Modified: 2021-07-23 14:16:45
 */

package http

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

func WithConcurrent(count uint) CrawlerOption {
	return func(c *Crawler) {
		c.goCount = count
	}
}

func WithRetry(count uint) CrawlerOption {
	return func(c *Crawler) {
		c.retryCount = count
	}
}
