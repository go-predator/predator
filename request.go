/*
 * @Author: thepoy
 * @Email: email@example.com
 * @File Name: request.go
 * @Created: 2021-07-24 13:29:11
 * @Modified: 2021-07-24 21:39:53
 */

package predator

import (
	"sync/atomic"

	ctx "github.com/thep0y/predator/context"
	"github.com/valyala/fasthttp"
)

type Request struct {
	// 访问的链接
	URL string
	// 请求方法
	Method string
	// 请求头
	Headers *fasthttp.RequestHeader
	// 请求和响应之间共享的上下文
	Ctx ctx.Context
	// 请求体
	Body []byte
	// 唯一标识
	ID uint32
	// 每个请求可以单独设置一个代理 ip，当前仅限于 http 代理
	ProxyURL string
	// 中断本次请求
	abort   bool
	crawler *Crawler
}

// New 使用原始请求的上下文创建一个新的请求
func (r *Request) New(method, URL string, body []byte) *Request {
	return &Request{
		Method:  method,
		URL:     URL,
		Body:    body,
		Ctx:     r.Ctx,
		Headers: &fasthttp.RequestHeader{},
		ID:      atomic.AddUint32(&r.crawler.requestCount, 1),
		crawler: r.crawler,
	}
}

func (r *Request) Abort() {
	r.abort = true
}
