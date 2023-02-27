/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   request.go
 * @Created At:  2021-07-24 13:29:11
 * @Modified At: 2023-02-27 13:41:39
 * @Modified By: thepoy
 */

package predator

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io"
	"time"

	"net/http"
	"net/textproto"
	"sort"
	"strings"
	"sync"

	pctx "github.com/go-predator/predator/context"
	"github.com/go-predator/predator/json"
)

type Request struct {
	req  *http.Request
	body []byte
	// 请求和响应之间共享的上下文
	Ctx pctx.Context
	// 待缓存的键值对
	cachedMap map[string]string
	// 唯一标识
	ID uint32
	// 中断本次请求
	abort bool
	// 基于原 crawler 重试或发出新请求
	crawler *Crawler
	// 重试计数器
	retryCounter uint32
	// 允许重定向
	checkRedirect func(req *http.Request, via []*http.Request) error
	timeout       time.Duration
}

func (r Request) IsCached() (bool, error) {
	if r.crawler.cache == nil {
		return false, ErrNoCache
	}

	hash, err := r.Hash()
	if err != nil {
		return false, err
	}

	_, ok := r.crawler.cache.IsCached(hash)
	return ok, nil
}

func (r *Request) Header() http.Header {
	return r.req.Header
}

func (r *Request) Abort() {
	r.abort = true
}

func (r *Request) SetHeader(header http.Header) {
	r.req.Header = header
}

func (r *Request) SetContentType(contentType string) {
	if r.req.Header == nil {
		r.req.Header = make(http.Header)
	}

	r.req.Header.Set("Content-Type", contentType)
}

// func defaultCheckRedirect(req *http.Request, via []*http.Request) error {
// 	if len(via) >= 10 {
// 		return errors.New("stopped after 10 redirects")
// 	}

// 	return nil
// }

func doNotFollowRedirect(req *http.Request, via []*http.Request) error {
	return http.ErrUseLastResponse
}

func (r *Request) DoNotFollowRedirects() {
	r.checkRedirect = doNotFollowRedirect
}

// SetTimeout sets the waiting time for each request before
// the remote end returns a response.
//
// The function doesn't follow redirects.
func (r *Request) SetTimeout(t time.Duration) {
	r.timeout = t
}

func (r *Request) SetHeaders(headers map[string]string) {
	for k, v := range headers {
		r.req.Header.Set(k, v)
	}
}

func (r *Request) SetNewHeaders(headers http.Header) {
	r.req.Header = headers
}

func (r *Request) AddRookies(cookies map[string]string) {
	for k, v := range cookies {
		v, ok := parseCookieValue(v, true)
		if !ok {
			continue
		}

		s := fmt.Sprintf("%s=%s", k, v)
		if c := r.req.Header.Get("Cookie"); c != "" {
			r.req.Header.Set("Cookie", c+"; "+s)
		} else {
			r.req.Header.Set("Cookie", s)
		}
	}
}

func (r *Request) WithBody() {
	if r.body != nil {
		r.req.Body = io.NopCloser(bytes.NewBuffer(r.body))
	}
}

func (r *Request) ParseRawCookie(rawCookie string) {
	line := textproto.TrimString(rawCookie)

	var part string
	for len(line) > 0 { // continue since we have rest
		part, line, _ = strings.Cut(line, ";")
		part = textproto.TrimString(part)
		if part == "" {
			continue
		}
		name, val, _ := strings.Cut(part, "=")
		name = textproto.TrimString(name)
		if !isCookieNameValid(name) {
			continue
		}
		val, ok := parseCookieValue(val, true)
		if !ok {
			continue
		}

		r.req.AddCookie(&http.Cookie{Name: name, Value: val})
	}
}

func (r Request) URL() string {
	return r.req.URL.String()
}

func (r Request) Method() string {
	return r.req.Method
}

func (r Request) NumberOfRetries() uint32 {
	return r.retryCounter
}

func (r Request) Get(u string) error {
	return r.Request(MethodGet, u, nil, nil)
}

func (r Request) GetWithCache(URL string, cacheFields ...CacheField) error {
	return r.crawler.get(URL, r.Header(), r.Ctx, true, cacheFields...)
}

func (r Request) Post(URL string, requestData map[string]string) error {
	return r.crawler.post(URL, requestData, r.Header(), r.Ctx, true)
}

func (r Request) PostWithCache(URL string, requestData map[string]string, cacheFields ...CacheField) error {
	return r.crawler.post(URL, requestData, r.Header(), r.Ctx, true, cacheFields...)
}
func (r Request) PostJSON(URL string, requestData map[string]any) error {
	return r.crawler.postJSON(URL, requestData, r.Header(), r.Ctx, true)
}

func (r Request) PostJSONWithCache(URL string, requestData map[string]any, cacheFields ...CacheField) error {
	return r.crawler.postJSON(URL, requestData, r.Header(), r.Ctx, true, cacheFields...)
}
func (r Request) PostMultipart(URL string, mfw *MultipartFormWriter) error {
	return r.crawler.postMultipart(URL, mfw, r.Header(), r.Ctx, true)
}

func (r Request) PostMultipartWithCache(URL string, mfw *MultipartFormWriter, cacheFields ...CacheField) error {
	return r.crawler.postMultipart(URL, mfw, r.Header(), r.Ctx, true, cacheFields...)
}

func (r Request) Request(method, URL string, cachedMap map[string]string, body []byte) error {
	return r.crawler.request(method, URL, body, cachedMap, r.Header(), r.Ctx, true)
}

// AbsoluteURL returns with the resolved absolute URL of an URL chunk.
// AbsoluteURL returns empty string if the URL chunk is a fragment or
// could not be parsed
func (r Request) AbsoluteURL(src string) string {
	if strings.HasPrefix(src, "#") {
		return ""
	}

	absoluteURL, err := r.req.URL.Parse(src)
	if err != nil {
		return ""
	}
	absoluteURL.Fragment = ""
	if absoluteURL.Scheme == "//" {
		absoluteURL.Scheme = r.req.URL.Scheme
	}
	return absoluteURL.String()
}

type cacheRequest struct {
	// 访问的链接
	URL string
	// 请求方法
	Method string
	// 待缓存的 map
	CacheKey []byte
}

func marshalCachedMap(cachedMap map[string]string) []byte {
	keys := make([]string, 0, len(cachedMap))
	for k := range cachedMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b bytes.Buffer

	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteString(`, `)
		}
		b.WriteByte('"')
		b.WriteString(k)
		b.WriteString(`": `)
		b.WriteByte('"')
		b.WriteString(cachedMap[k])
		b.WriteByte('"')
	}
	b.WriteByte('}')

	return b.Bytes()
}

func (r Request) marshal() ([]byte, error) {
	cr := &cacheRequest{
		URL:    r.URL(),
		Method: r.Method(),
	}

	if r.cachedMap != nil {
		cr.CacheKey = marshalCachedMap(r.cachedMap)
	} else {
		cr.CacheKey = r.body
	}

	if r.Method() == MethodGet {
		// 为 GET 设置 cachedFields，则说明一定是因为 url 是变化的，所以不能将整个 url 作为缓存标志，
		// 此时将 CacheKey 作为缓存标志是最佳选择
		if cr.CacheKey != nil {
			return cr.CacheKey, nil
		} else {
			return []byte(r.URL()), nil
		}
	}

	return json.Marshal(cr)
}

func (r Request) Hash() (string, error) {
	cacheBody, err := r.marshal()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha1.Sum(cacheBody)), nil
}

func resetRequest(req *http.Request) {
	ReleaseURL(req.URL)

	req.Method = ""
	req.Proto = "HTTP/1.0"
	req.ProtoMajor = 1
	req.ProtoMinor = 0

	req.Body = nil
	req.GetBody = nil
	req.ContentLength = 0
	req.TransferEncoding = req.TransferEncoding[:0]
	req.Close = false
	req.Host = ""

	ResetMap(req.Form)
	ResetMap(req.PostForm)

	req.MultipartForm = nil

	ResetMap(req.Trailer)

	req.RemoteAddr = ""
	req.RequestURI = ""
	req.TLS = nil

	req.Header = nil

	// TODO: 释放 Response
	req.Response = nil
}

func (r *Request) Reset() {
	if r.body != nil {
		r.body = r.body[:0]
	}
	ResetMap(r.cachedMap)
	r.ID = 0
	r.abort = false
	r.crawler = nil
	r.retryCounter = 0
	r.checkRedirect = nil
	r.timeout = 0
}

var (
	rawRequestPool = &sync.Pool{
		New: func() any {
			r := new(http.Request)
			r.Header = make(http.Header)

			return r
		},
	}
	requestPool = &sync.Pool{
		New: func() any {
			r := new(Request)
			r.req = acquireRequest()

			return r
		},
	}
)

func acquireRequest() *http.Request {
	return rawRequestPool.Get().(*http.Request)
}

func releaseRequest(req *http.Request) {
	resetRequest(req)
	rawRequestPool.Put(req)
}

// AcquireRequest returns an empty Request instance from request pool.
//
// The returned Request instance may be passed to ReleaseRequest when it is
// no longer needed. This allows Request recycling, reduces GC pressure
// and usually improves performance.
func AcquireRequest() *Request {
	r := requestPool.Get().(*Request)

	return r
}

// ReleaseRequest returns req acquired via AcquireRequest to request pool.
//
// It is forbidden accessing req and/or its' members after returning
// it to request pool.
func ReleaseRequest(req *Request) {
	req.Reset()
	releaseRequest(req.req)

	requestPool.Put(req)
}
