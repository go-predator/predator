/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   options.go
 * @Created At:  2021-07-23 08:58:31
 * @Modified At: 2023-03-16 10:21:13
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

// CrawlerOption is a function type that takes a pointer to Crawler and configures it.
type CrawlerOption func(*Crawler)

// SkipVerification returns a CrawlerOption that skips TLS verification.
func SkipVerification() CrawlerOption {
	return func(c *Crawler) {
		c.client.Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
}

// WithLogger returns a CrawlerOption that sets the logger for Crawler.
func WithLogger(logger *log.Logger) CrawlerOption {
	if logger == nil {
		logger = log.NewLogger(log.WARNING, log.ToConsole(), 2)
	}

	return func(c *Crawler) {
		c.log = log.NewLogger(log.Level(logger.L.GetLevel()), logger.Out(), 2)
	}
}

// WithConsoleLogger returns a CrawlerOption that sets the logger to output to console.
func WithConsoleLogger(level log.Level) CrawlerOption {
	return func(c *Crawler) {
		c.log = log.NewLogger(level, log.ToConsole(), 2)
	}
}

// WithFileLogger returns a CrawlerOption that sets the logger to output to a file.
func WithFileLogger(level log.Level, filename string) CrawlerOption {
	return func(c *Crawler) {
		c.log = log.NewLogger(level, log.MustToFile(filename, -1), 2)
	}
}

// WithConsoleAndFileLogger returns a CrawlerOption that sets the logger to output to both console and file.
func WithConsoleAndFileLogger(level log.Level, filename string) CrawlerOption {
	return func(c *Crawler) {
		c.log = log.NewLogger(level, log.MustToConsoleAndFile(filename, -1), 2)
	}
}

// WithDefaultLogger returns a CrawlerOption that sets the logger to default (console, level WARNING).
func WithDefaultLogger() CrawlerOption {
	return WithLogger(nil)
}

// WithUserAgent returns a CrawlerOption that sets the User-Agent for Crawler.
func WithUserAgent(ua string) CrawlerOption {
	return func(c *Crawler) {
		c.UserAgent = ua
	}
}

// RecordRemoteAddr returns a CrawlerOption that records remote addresses.
func RecordRemoteAddr() CrawlerOption {
	return func(c *Crawler) {
		c.recordRemoteAddr = true
	}
}

// WithRawCookie returns a CrawlerOption that sets the raw cookies for Crawler.
func WithRawCookie(cookies string) CrawlerOption {
	return func(c *Crawler) {
		c.rawCookies = cookies
	}
}

// WithCookies returns a CrawlerOption that sets the cookies for Crawler.
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

// WithConcurrency returns a CrawlerOption that sets the concurrency for Crawler.
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

// RetryCondition is a function type that takes a response object and returns a boolean value.
//
// It is used as a condition to decide whether to retry a request or not.
type RetryCondition func(r *Response) bool

// WithRetry returns a CrawlerOption function that sets the retry count and condition.
//
// The count parameter specifies the maximum number of times to retry the request, and cond is a function that decides whether to retry the request or not.
func WithRetry(count uint32, cond RetryCondition) CrawlerOption {
	return func(c *Crawler) {
		c.retryCount = count
		c.retryCondition = cond
	}
}

// WithProxy returns a CrawlerOption function that sets the proxy URL for the crawler.
//
// The proxyURL parameter specifies the URL of the proxy to use.
func WithProxy(proxyURL string) CrawlerOption {
	return func(c *Crawler) {
		c.proxyURLPool = []string{proxyURL}
	}
}

// WithProxyPool returns a CrawlerOption function that sets a pool of proxy URLs for the crawler.
//
// The proxyURLs parameter specifies the pool of proxy URLs to use.
func WithProxyPool(proxyURLs []string) CrawlerOption {
	return func(c *Crawler) {
		c.proxyURLPool = proxyURLs
	}
}

// WithTimeout returns a CrawlerOption function that sets the timeout duration for the crawler.
//
// The timeout parameter specifies the duration of the timeout.
func WithTimeout(timeout time.Duration) CrawlerOption {
	return func(c *Crawler) {
		c.timeout = timeout
	}
}

// WithComplementProxyPool returns a CrawlerOption function that sets a function to complement the proxy pool.
//
// The f parameter is a function that takes the current proxy pool and returns a new pool of proxy URLs.
func WithComplementProxyPool(f ComplementProxyPool) CrawlerOption {
	return func(c *Crawler) {
		c.complementProxyPool = f
	}
}

// WithCache returns a CrawlerOption function that sets the cache for the crawler.
//
// The cc parameter specifies the cache implementation to use.
// The compressed parameter specifies whether to compress the cache data or not.
// The cacheCondition parameter is a function that decides whether to cache the response or not.
// The cacheFields parameter specifies the fields of the response to cache.
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

// DisableIPv6 returns a CrawlerOption function that disables IPv6.
//
// It sets the dial context of the crawler's HTTP transport to a new Dialer with Timeout, KeepAlive, and FallbackDelay values.
func DisableIPv6() CrawlerOption {
	return func(c *Crawler) {
		c.client.Transport.(*http.Transport).DialContext = (&net.Dialer{
			Timeout:       30 * time.Second,
			KeepAlive:     30 * time.Second,
			FallbackDelay: -1,
		}).DialContext
	}
}
