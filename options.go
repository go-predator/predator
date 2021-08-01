/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: options.go (c) 2021
 * @Created: 2021-07-23 08:58:31
 * @Modified: 2021-08-01 21:59:51
 */

package predator

import (
	"io"
	"os"
	"sync"

	"github.com/rs/zerolog"
	"github.com/thep0y/predator/cache"
	"github.com/thep0y/predator/log"
)

type CrawlerOption func(*Crawler)

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

// WithCache 使用缓存，可以选择是否压缩缓存的响应
func WithCache(cc cache.Cache, compressed bool) CrawlerOption {
	return func(c *Crawler) {
		if cc == nil {
			cc = &cache.SQLiteCache{}
		}
		cc.Compressed(compressed)
		err := cc.Init()
		if err != nil {
			panic(err)
		}
		c.cache = cc
	}
}
