/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: response.go (c) 2021
 * @Created: 2021-07-24 13:34:44
 * @Modified: 2021-08-01 10:30:20
 */

package predator

import (
	"errors"
	"io/ioutil"
	"sync"

	ctx "github.com/thep0y/predator/context"
	"github.com/thep0y/predator/json"
	"github.com/valyala/fasthttp"
)

var (
	IncorrectResponse = errors.New("the response status code is not 200 or 201")
)

type Response struct {
	// 响应状态码
	StatusCode int
	// 二进制请求体
	Body []byte
	// 请求和响应之间共享的上下文
	Ctx ctx.Context `json:"-"`
	// 响应对应的请求
	Request *Request `json:"-"`
	// 响应头
	Headers fasthttp.ResponseHeader
	// 是否从缓存中取得的响应
	FromCache bool
}

// Save writes response body to disk
func (r *Response) Save(fileName string) error {
	return ioutil.WriteFile(fileName, r.Body, 0644)
}

func (r *Response) GetSetCookie() string {
	return string(r.Headers.Peek("Set-Cookie"))
}

func (r *Response) ContentType() string {
	return string(r.Headers.Peek("Content-Type"))
}

func (r *Response) String() string {
	return string(r.Body)
}

func (r *Response) Reset() {
	r.StatusCode = 0
	r.Body = r.Body[:0]
	ctx.ReleaseCtx(r.Ctx)
	ReleaseRequest(r.Request)
	r.Headers.Reset()
	r.FromCache = false
}

func (r Response) Marshal() ([]byte, error) {
	if r.StatusCode != fasthttp.StatusOK && r.StatusCode != fasthttp.StatusCreated {
		return nil, IncorrectResponse
	}
	return json.Marshal(r)
}

var (
	responsePool sync.Pool
)

// AcquireResponse returns an empty Response instance from response pool.
//
// The returned Response instance may be passed to ReleaseResponse when it is
// no longer needed. This allows Response recycling, reduces GC pressure
// and usually improves performance.
func AcquireResponse() *Response {
	v := responsePool.Get()
	if v == nil {
		return &Response{}
	}
	return v.(*Response)
}

// ReleaseResponse returns resp acquired via AcquireResponse to response pool.
//
// It is forbidden accessing resp and/or its' members after returning
// it to response pool.
func ReleaseResponse(resp *Response) {
	resp.Reset()
	responsePool.Put(resp)
}
