/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: response.go
 * @Created: 2021-07-24 13:34:44
 * @Modified: 2022-01-23 17:16:32
 */

package predator

import (
	"errors"
	"io/ioutil"
	"net"
	"sync"

	ctx "github.com/go-predator/predator/context"
	"github.com/go-predator/predator/json"
	"github.com/valyala/bytebufferpool"
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
	// 客户端公网 ip
	clientIP net.Addr
	// 本地局域网 ip
	localIP net.Addr
	timeout bool
	// Whether the response is valid,
	// html for invalid responses will not be parsed
	invalid bool
}

// Save writes response body to disk
func (r *Response) Save(fileName string) error {
	return ioutil.WriteFile(fileName, r.Body, 0644)
}

// Invalidate marks the current response as invalid and skips the html parsing process
func (r *Response) Invalidate() {
	r.invalid = true
}

func (r *Response) GetSetCookie() string {
	return string(r.Headers.Peek("Set-Cookie"))
}

func (r *Response) ContentType() string {
	return string(r.Headers.Peek("Content-Type"))
}

// BodyGunzip returns un-gzipped body data.
//
// This method may be used if the response header contains
// 'Content-Encoding: gzip' for reading un-gzipped body.
// Use Body for reading gzipped response body.
func (r *Response) BodyGunzip() ([]byte, error) {
	var bb bytebufferpool.ByteBuffer
	_, err := fasthttp.WriteGunzip(&bb, r.Body)
	if err != nil {
		return nil, err
	}
	return bb.B, nil
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
	r.localIP = nil
	r.clientIP = nil
}

type cachedHeaders struct {
	StatusCode    int
	ContentType   []byte // this is the most important field
	ContentLength int
	Server        []byte
}

type cachedResponse struct {
	Body    []byte
	Headers cachedHeaders
}

func (r Response) convertHeaders() cachedHeaders {
	ch := cachedHeaders{}
	ch.StatusCode = r.StatusCode
	ch.ContentType = r.Headers.ContentType()
	ch.ContentLength = r.Headers.ContentLength()
	ch.Server = r.Headers.Server()

	return ch
}

func (r Response) Marshal() ([]byte, error) {
	// The cached response does not need to save all the response headers,
	// so the following code is not used to convert the response headers to bytes
	// var buf bytes.Buffer
	// b := bufio.NewWriter(&buf)
	// r.Headers.Write(b)
	// b.Flush()

	var cr cachedResponse
	cr.Body = r.Body
	cr.Headers = r.convertHeaders()

	return json.Marshal(cr)
}

func (r *Response) Unmarshal(cachedBody []byte) error {
	var (
		cr  cachedResponse
		err error
	)
	err = json.Unmarshal(cachedBody, &cr)
	if err != nil {
		return err
	}

	r.Body = cr.Body
	r.StatusCode = cr.Headers.StatusCode
	r.Headers.SetStatusCode(r.StatusCode)
	r.Headers.SetContentTypeBytes(cr.Headers.ContentType)
	r.Headers.SetContentLength(cr.Headers.ContentLength)
	r.Headers.SetServerBytes(cr.Headers.Server)

	return nil
}

func (r Response) LocalIP() string {
	if r.localIP != nil {
		return r.localIP.String()
	}
	return ""
}

func (r Response) ClientIP() string {
	if r.clientIP != nil {
		return r.clientIP.String()
	}
	return ""
}

func (r Response) IsTimeout() bool {
	return r.timeout
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
