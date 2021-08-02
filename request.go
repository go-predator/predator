/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: request.go (c) 2021
 * @Created: 2021-07-24 13:29:11
 * @Modified: 2021-08-02 14:37:27
 */

package predator

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	// "io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

// MultipartForm 请求体的构造
type MultipartForm struct {
	buf *bytes.Buffer
	// 每个网站 boundary 前的横线数量是固定的，直接赋给这个字段
	boundary string
	bodyMap  map[string]string
}

func NewMultipartForm(dash string, f CustomRandomBoundary) *MultipartForm {
	return &MultipartForm{
		buf:      &bytes.Buffer{},
		boundary: dash + f(),
		bodyMap:  make(map[string]string),
	}
}

// Boundary returns the Writer's boundary.
func (mf *MultipartForm) Boundary() string {
	return mf.boundary
}

// FormDataContentType returns the Content-Type for an HTTP
// multipart/form-data with this Writer's Boundary.
func (mf *MultipartForm) FormDataContentType() string {
	b := mf.boundary
	// We must quote the boundary if it contains any of the
	// tspecials characters defined by RFC 2045, or space.
	if strings.ContainsAny(b, `()<>@,;:\"/[]?= `) {
		b = `"` + b + `"`
	}
	return "multipart/form-data; boundary=" + b
}

func (mf *MultipartForm) appendHead() {
	bodyBoundary := "--" + mf.boundary
	mf.buf.WriteString(bodyBoundary + "\r\n")
}

func (mf *MultipartForm) appendTail() {
	mf.buf.WriteString("\r\n")
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

func (mf *MultipartForm) AppendString(name, value string) {
	mf.appendHead()
	mf.buf.WriteString(`Content-Disposition: form-data; name="`)
	mf.buf.WriteString(name)
	mf.buf.WriteString(`"`)
	mf.buf.WriteString("\r\n\r\n")
	mf.buf.WriteString(value)
	mf.appendTail()

	mf.bodyMap[name] = value
}

func getMimeType(buf []byte) string {
	return http.DetectContentType(buf)
}

func (mf *MultipartForm) AppendFile(name, filePath string) error {
	_, filename := filepath.Split(filePath)

	mf.appendHead()
	mf.buf.WriteString(`Content-Disposition: form-data; name="`)
	mf.buf.WriteString(name)
	mf.buf.WriteString(`"; filename="`)
	mf.buf.WriteString(filename)
	mf.buf.WriteString(`"`)
	mf.buf.WriteString("\r\nContent-Type: ")

	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	// 只需要使用前 512 个字节即可检测出一个文件的类型
	contentType := getMimeType(fileBytes[:512])

	mf.buf.WriteString(contentType)
	mf.buf.WriteString("\r\n\r\n")

	mf.buf.Write(fileBytes)

	mf.appendTail()

	mf.bodyMap[filename] = filePath

	return nil
}

func (mf *MultipartForm) Bytes() []byte {
	bodyBoundary := "--" + mf.boundary + "--"
	mf.buf.WriteString(bodyBoundary)
	return mf.buf.Bytes()
}
