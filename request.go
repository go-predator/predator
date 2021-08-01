/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: request.go (c) 2021
 * @Created: 2021-07-24 13:29:11
 * @Modified: 2021-08-01 12:24:42
 */

package predator

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"

	pctx "github.com/thep0y/predator/context"
	"github.com/thep0y/predator/json"
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
	Ctx pctx.Context
	// 请求体
	Body []byte
	// 原始请求体，以后根据需要改成 map[string]interface{}
	bodyMap map[string]string
	// 唯一标识
	ID uint32
	// 中断本次请求
	abort bool
	// 基于原 crawler 重试或发出新请求
	crawler *Crawler
	// 重试计数器
	retryCounter uint32
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

func (r *Request) SetContentType(contentType string) {
	r.Headers.Set("Content-Type", contentType)
}

func (r *Request) SetHeaders(headers map[string]string) {
	for k, v := range headers {
		r.Headers.Set(k, v)
	}
}

func (r Request) NumberOfRetries() uint32 {
	return r.retryCounter
}

func (r Request) Get(u string) error {
	return r.crawler.Get(u)
}

func (r Request) Post(URL string, requestData map[string]string, ctx pctx.Context) error {
	return r.crawler.Post(URL, requestData, ctx)
}

type cacheRequest struct {
	// 访问的链接
	URL string
	// 请求方法
	Method string
	// 请求体
	Body []byte
}

func marshalPostBody(body map[string]string) []byte {
	keys := make([]string, 0, len(body))
	for k := range body {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b bytes.Buffer

	b.WriteString("{")
	for i, k := range keys {
		if i > 0 {
			b.WriteString(`, `)
		}
		b.WriteString(`"`)
		b.WriteString(k)
		b.WriteString(`": `)
		b.WriteString(`"`)
		b.WriteString(body[k])
		b.WriteString(`"`)
	}
	b.WriteString("}")

	return b.Bytes()
}

func (r Request) Marshal() ([]byte, error) {
	cr := &cacheRequest{
		URL:    r.URL,
		Method: r.Method,
	}

	if r.Method == fasthttp.MethodPost {
		cr.Body = marshalPostBody(r.bodyMap)
	}

	return json.Marshal(cr)
}

func (r Request) Hash() (string, error) {
	cacheBody, err := r.Marshal()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha1.Sum(cacheBody)), nil
}

func (r *Request) Reset() {
	r.URL = ""
	r.Method = ""
	r.Headers.Reset()
	r.Body = r.Body[:0]
	r.bodyMap = make(map[string]string)
	r.ID = 0
	r.abort = false
	r.crawler = nil
	r.retryCounter = 0
}

var (
	requestPool sync.Pool
)

// AcquireRequest returns an empty Request instance from request pool.
//
// The returned Request instance may be passed to ReleaseRequest when it is
// no longer needed. This allows Request recycling, reduces GC pressure
// and usually improves performance.
func AcquireRequest() *Request {
	v := requestPool.Get()
	if v == nil {
		return &Request{}
	}
	return v.(*Request)
}

// ReleaseRequest returns req acquired via AcquireRequest to request pool.
//
// It is forbidden accessing req and/or its' members after returning
// it to request pool.
func ReleaseRequest(req *Request) {
	req.Reset()
	requestPool.Put(req)
}
