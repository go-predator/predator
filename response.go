/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   response.go
 * @Created At:  2021-07-24 13:34:44
 * @Modified At: 2023-03-02 11:43:26
 * @Modified By: thepoy
 */

package predator

import (
	"errors"
	"net/http"
	"os"
	"strconv"
	"sync"

	ctx "github.com/go-predator/predator/context"
	"github.com/go-predator/predator/json"
)

var (
	ErrIncorrectResponse = errors.New("the response status code is not 20X")
)

type Response struct {
	resp *http.Response
	// 响应状态码
	StatusCode StatusCode

	header http.Header

	// 二进制请求体
	Body []byte
	// 请求和响应之间共享的上下文
	Ctx ctx.Context `json:"-"`
	// 响应对应的请求
	Request *Request `json:"-"`
	// 是否从缓存中取得的响应
	FromCache bool
	// 服务器公网 ip
	clientIP string
	timeout  bool
	// Whether the response is valid,
	// html for invalid responses will not be parsed
	invalid bool
}

// Save writes response body to disk
func (r *Response) Save(fileName string) error {
	return os.WriteFile(fileName, r.Body, 0644)
}

// Invalidate marks the current response as invalid and skips the html parsing process
func (r *Response) Invalidate() {
	r.invalid = true
}

func (r *Response) Method() string {
	return r.Request.Method()
}

func (r *Response) ContentType() string {
	return r.header.Get("Content-Type")
}

func (r *Response) ContentLength() uint64 {
	cl := r.header.Get("Content-Length")
	length, err := strconv.ParseUint(cl, 10, 64)
	if err != nil {
		panic(err)
	}

	return length
}

func (r *Response) GetSetCookie() string {
	return r.resp.Header.Get("Set-Cookie")
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
	ResetMap(r.resp.Header)

	r.FromCache = false
	r.invalid = false
	r.clientIP = ""
}

type cachedHeaders struct {
	StatusCode    StatusCode
	ContentType   string // this is the most important field
	ContentLength int64
	Server        []byte
	Location      []byte
}

type cachedResponse struct {
	Body    []byte
	Headers *cachedHeaders
}

func (r *Response) convertHeaders() (*cachedHeaders, error) {
	ch := &cachedHeaders{}
	ch.StatusCode = r.StatusCode
	ch.ContentType = r.ContentType()
	ch.ContentLength = r.resp.ContentLength
	ch.Server = []byte(r.ClientIP())

	if ch.StatusCode == StatusFound {
		if ch.Location == nil {
			return nil, ErrInvalidResponseStatus
		}
		ch.Location = []byte(r.resp.Header.Get("Location"))
	}

	return ch, nil
}

func (r *Response) Marshal() ([]byte, error) {
	// The cached response does not need to save all the response headers,
	// so the following code is not used to convert the response headers to bytes
	// var buf bytes.Buffer
	// b := bufio.NewWriter(&buf)
	// r.Headers.Write(b)
	// b.Flush()

	var (
		cr  cachedResponse
		err error
	)
	cr.Body = r.Body
	cr.Headers, err = r.convertHeaders()
	if err != nil {
		return nil, err
	}

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
	r.clientIP = string(cr.Headers.Server)
	r.resp.Header.Set("Content-Type", cr.Headers.ContentType)
	r.resp.Header.Set("Content-Length", strconv.FormatInt(cr.Headers.ContentLength, 10))

	return nil
}

func (r *Response) ClientIP() string {
	return r.clientIP
}

func (r *Response) IsTimeout() bool {
	return r.timeout
}

var (
	rawResponsePool = &sync.Pool{
		New: func() any {
			return new(http.Response)
		},
	}
	responsePool = &sync.Pool{
		New: func() any {
			resp := new(Response)
			resp.resp = acquireResponse()

			return resp
		},
	}
)

// AcquireResponse returns an empty Response instance from response pool.
//
// The returned Response instance may be passed to ReleaseResponse when it is
// no longer needed. This allows Response recycling, reduces GC pressure
// and usually improves performance.
func AcquireResponse() *Response {
	return responsePool.Get().(*Response)
}

// ReleaseResponse returns resp acquired via AcquireResponse to response pool.
//
// It is forbidden accessing resp and/or its' members after returning
// it to response pool.
func ReleaseResponse(resp *Response, releaseCtx bool) {
	resp.Reset(releaseCtx)
	responsePool.Put(resp)
}

func acquireResponse() *http.Response {
	return rawResponsePool.Get().(*http.Response)
}

func resetResponse(resp *http.Response) {
	if resp == nil {
		return
	}

	resp.Status = ""
	resp.StatusCode = 0
	resp.Proto = ""
	resp.ProtoMajor = 0
	resp.ProtoMinor = 0

	ResetMap(resp.Header)

	resp.Body = nil
	resp.ContentLength = 0

	if resp.TransferEncoding != nil {
		resp.TransferEncoding = resp.TransferEncoding[:0]
	}

	resp.Close = false
	resp.Uncompressed = false

	ResetMap(resp.Trailer)

	releaseRequest(resp.Request)

	resp.TLS = nil
}

func releaseResponse(resp *http.Response) {
	resetResponse(resp)
	rawResponsePool.Put(resp)
}
