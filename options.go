/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: options.go
 * @Created: 2021-07-23 08:58:31
 * @Modified:  2021-11-24 20:52:16
 */

package predator

import (
	"crypto/tls"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/go-predator/cache"
	"github.com/go-predator/predator/log"
	"github.com/rs/zerolog"
)

type CrawlerOption func(*Crawler)

// SkipVerification will skip verifying the certificate when
// you access the `https` protocol
func SkipVerification() CrawlerOption {
	return func(c *Crawler) {
		c.client.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	}
}

type LogOp struct {
	level zerolog.Level
	Out   io.Writer
}

func (l *LogOp) SetLevel(level zerolog.Level) {
	l.level = level
}

func (l *LogOp) ToConsole() {
	l.Out = zerolog.ConsoleWriter{Out: os.Stdout}
}

func fileWriter(filepath string) (io.Writer, error) {
	return os.OpenFile(filepath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
}

func (l *LogOp) ToFile(filepath string) error {
	writer, err := fileWriter(filepath)
	if err != nil {
		return err
	}
	l.Out = writer
	return nil
}

func (l *LogOp) ToConsoleAndFile(filepath string) error {
	fw, err := fileWriter(filepath)
	if err != nil {
		return err
	}
	l.Out = zerolog.MultiLevelWriter(fw, zerolog.ConsoleWriter{Out: os.Stdout})
	return nil
}

func WithLogger(lop *LogOp) CrawlerOption {
	if lop == nil {
		lop = new(LogOp)
		lop.level = zerolog.InfoLevel
	}

	if lop.Out == nil {
		lop.Out = zerolog.ConsoleWriter{Out: os.Stdout}
	}

	return func(c *Crawler) {
		c.log = log.NewLogger(lop.level, lop.Out)
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
		temp := strings.Split(c, "=")
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
func WithCache(cc Cache, compressed bool, cacheCondition CacheCondition, cacheFileds ...string) CrawlerOption {
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
