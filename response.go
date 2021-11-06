/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: response.go
 * @Created: 2021-07-24 13:34:44
 * @Modified:  2021-11-06 17:33:32
 */

package predator

import (
	"errors"
	"io/ioutil"
	"sync"

	ctx "github.com/go-predator/predator/context"
	"github.com/go-predator/predator/json"
	"github.com/valyala/fasthttp"
)

var (
	ErrIncorrectResponse = errors.New("the response status code is not 20X")
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

func (r *Response) Reset(releaseCtx bool) {
	r.StatusCode = 0
	if r.Body != nil {
		// 将 body 长度截为 0，这样不会删除引用关系，GC 不会回收，
		// 可以实现 body 的复用
		r.Body = r.Body[:0]
	}

	// 为了在链式请求中传递上下文，不能每次响应后都释放上下文。
	if releaseCtx {
		ctx.ReleaseCtx(r.Ctx)
	}

	ReleaseRequest(r.Request)
	r.Headers.Reset()
	r.FromCache = false
}

func (r Response) Marshal() ([]byte, error) {
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
func ReleaseResponse(resp *Response, releaseCtx bool) {
	resp.Reset(releaseCtx)
	responsePool.Put(resp)
}
