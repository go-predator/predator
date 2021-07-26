/*
 * @Author: Ryan Wong
 * @Email: thepoy@163.com
 * @File Name: options.go
 * @Created: 2021-07-23 08:58:31
 * @Modified: 2021-07-26 10:29:55
 */

package predator

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
		c.proxyURL = proxyURL
	}
}

// WithProxyPool 使用一个代理池
func WithProxyPool(proxyURLs []string) CrawlerOption {
	return func(c *Crawler) {
		c.proxyURLPool = proxyURLs
	}
}
