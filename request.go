/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   request.go
 * @Created At:  2021-07-24 13:29:11
 * @Modified At: 2023-03-16 11:08:50
 * @Modified By: thepoy
 */

package predator

import (
	"bytes"
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"time"

	"net/http"
	"net/textproto"
	"sort"
	"strings"

	pctx "github.com/go-predator/predator/context"
	"github.com/go-predator/predator/json"
)

// Request represents an HTTP request, including its body, context, and metadata.
type Request struct {
	// The underlying HTTP request.
	req *http.Request
	// The body of the request.
	body []byte
	// The context shared between request and response.
	Ctx pctx.Context
	// The key-value pairs waiting to be cached.
	cachedMap map[string]string
	// The unique identifier for this request.
	ID uint32
	// Whether to abort this request.
	abort bool
	// The crawler that is responsible for sending this request.
	crawler *Crawler
	// The counter for retrying this request based on the original crawler or issuing a new request.
	retryCounter uint32
	// Whether to allow redirection.
	checkRedirect func(req *http.Request, via []*http.Request) error
	// The timeout control that only applies to a Request instance.
	timeout time.Duration
	cancel  context.CancelFunc
	// The proxy used when sending the request.
	proxyUsed string
}

// IsCached checks if the cache exists for this request.
func (r Request) IsCached() (bool, error) {
	// If the cache is not initialized, return false and the error.
	if r.crawler.cache == nil {
		return false, ErrNoCache
	}
	// Get the hash value of this request.
	hash, err := r.Hash()
	if err != nil {
		return false, err
	}
	// Check if this request is cached by the cache.
	_, ok := r.crawler.cache.IsCached(hash)
	return ok, nil
}

// Header returns the header of this request.
func (r *Request) Header() http.Header {
	return r.req.Header
}

// Abort sets the flag to abort this request.
func (r *Request) Abort() {
	r.abort = true
}

// SetHeader sets the request header to the given http.Header.
func (r *Request) SetHeader(header http.Header) {
	r.req.Header = header
}

// SetContentType sets the Content-Type header to the given contentType string.
// If the request header is nil, it will be initialized before setting the Content-Type.
func (r *Request) SetContentType(contentType string) {
	if r.req.Header == nil {
		r.req.Header = make(http.Header)
	}

	r.req.Header.Set("Content-Type", contentType)
}

func defaultCheckRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}

	return nil
}

func doNotFollowRedirect(req *http.Request, via []*http.Request) error {
	return http.ErrUseLastResponse
}

// DoNotFollowRedirects sets the checkRedirect function to doNotFollowRedirect which disables the following of redirects.
func (r *Request) DoNotFollowRedirects() {
	r.checkRedirect = doNotFollowRedirect
}

// SetTimeout sets the maximum time the request should wait for a response before timing out.
// It sets the request context with the given timeout duration.
// The function doesn't follow redirects.
func (r *Request) SetTimeout(timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	r.req = r.req.WithContext(ctx)
	r.cancel = cancel

	r.timeout = timeout
}

// SetHeaders sets the request headers to the given map of string key-value pairs.
func (r *Request) SetHeaders(headers map[string]string) {
	for k, v := range headers {
		r.req.Header.Set(k, v)
	}
}

// SetNewHeaders replaces the existing request headers with the given http.Header.
func (r *Request) SetNewHeaders(headers http.Header) {
	r.req.Header = headers
}

// AddRookies adds the given map of string key-value pairs as cookies to the request header.
func (r *Request) AddRookies(cookies map[string]string) {
	for k, v := range cookies {
		r.AddCookie(k, v)
	}
}

// WithBody sets the request body from the stored byte array.
func (r *Request) WithBody() {
	if r.body != nil {
		r.req.Body = io.NopCloser(bytes.NewBuffer(r.body))
	}
}

// ParseRawCookie parses the given rawCookie string and adds the parsed cookies to the request header.
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

// AddCookie adds a cookie to the request
func (r *Request) AddCookie(key, value string) {
	// Trim leading and trailing whitespace from key
	key = textproto.TrimString(key)
	// If the key is an empty string, panic with an error message
	if key == "" {
		panic(fmt.Errorf("key %w", ErrEmptyString))
	}

	// Trim leading and trailing whitespace from value
	value = textproto.TrimString(value)
	// If the value is an empty string, panic with an error message
	if value == "" {
		panic(fmt.Errorf("value %w", ErrEmptyString))
	}

	// If the cookie name is not valid, panic with an error message
	if !isCookieNameValid(key) {
		panic(fmt.Errorf("not a valid cookie name: %s", key))
	}

	// Parse the cookie value and check if it's valid
	val, ok := parseCookieValue(value, true)
	if !ok {
		panic(fmt.Errorf("not a valid cookie value: %s", value))
	}

	// Add the cookie to the request
	r.req.AddCookie(&http.Cookie{Name: key, Value: val})
}

// URL returns the request's URL as a string
func (r Request) URL() string {
	return r.req.URL.String()
}

// Method returns the request's HTTP method as a string
func (r Request) Method() string {
	return r.req.Method
}

// NumberOfRetries returns the number of times the request has been retried
func (r Request) NumberOfRetries() uint32 {
	return r.retryCounter
}

// Get sends a GET request to the specified URL
func (r Request) Get(u string) error {
	return r.Request(MethodGet, u, nil, nil)
}

// GetWithCache sends a GET request to the specified URL with caching
func (r Request) GetWithCache(URL string, cacheFields ...CacheField) error {
	return r.crawler.get(URL, r.Header(), r.Ctx, true, cacheFields...)
}

// Post sends a POST request to the specified URL with the specified data
func (r Request) Post(URL string, requestData map[string]string) error {
	return r.crawler.post(URL, requestData, r.Header(), r.Ctx, true, nil)
}

// PostWithCreateBodyFunc sends a POST request to the specified URL with the specified data and create body function
func (r Request) PostWithCreateBodyFunc(URL string, requestData map[string]string, createBodyFunc createPostBody) error {
	return r.crawler.post(URL, requestData, r.Header(), r.Ctx, true, createBodyFunc)
}

// PostWithCacheFields sends a POST request to the specified URL with the specified data and caching
func (r Request) PostWithCacheFields(URL string, requestData map[string]string, cacheFields ...CacheField) error {
	return r.crawler.post(URL, requestData, r.Header(), r.Ctx, true, nil, cacheFields...)
}

// PostWithCacheFieldsAndCreateBodyFunc sends a POST request to the specified URL with the specified data, create body function, and caching
func (r Request) PostWithCacheFieldsAndCreateBodyFunc(URL string, requestData map[string]string, createBodyFunc createPostBody, cacheFields ...CacheField) error {
	return r.crawler.post(URL, requestData, r.Header(), r.Ctx, true, createBodyFunc, cacheFields...)
}

// PostJSON sends a POST request to the specified URL with the given JSON data.
//
// It returns an error if the request fails.
func (r Request) PostJSON(URL string, requestData map[string]any) error {
	return r.crawler.postJSON(URL, requestData, r.Header(), r.Ctx, true)
}

// PostJSONWithCache sends a POST request to the specified URL with the given JSON data.
//
// It returns an error if the request fails.
// It also allows you to specify cache fields to use.
func (r Request) PostJSONWithCache(URL string, requestData map[string]any, cacheFields ...CacheField) error {
	return r.crawler.postJSON(URL, requestData, r.Header(), r.Ctx, true, cacheFields...)
}

// PostMultipart sends a POST request to the specified URL with the given multipart form data.
//
// It returns an error if the request fails.
func (r Request) PostMultipart(URL string, mfw *MultipartFormWriter) error {
	return r.crawler.postMultipart(URL, mfw, r.Header(), r.Ctx, true)
}

// PostMultipartWithCache sends a POST request to the specified URL with the given multipart form data.
//
// It returns an error if the request fails.
// It also allows you to specify cache fields to use.
func (r Request) PostMultipartWithCache(URL string, mfw *MultipartFormWriter, cacheFields ...CacheField) error {
	return r.crawler.postMultipart(URL, mfw, r.Header(), r.Ctx, true, cacheFields...)
}

// Request sends a HTTP request to the specified URL using the specified method and data.
//
// It returns an error if the request fails.
// It also allows you to specify cached data to use.
func (r Request) Request(method, URL string, cachedMap map[string]string, body []byte) error {
	return r.crawler.request(method, URL, body, cachedMap, r.Header(), r.Ctx, true)
}

// AbsoluteURL returns the resolved absolute URL of a URL chunk.
//
// If the URL chunk is a fragment or could not be parsed, it returns an empty string.
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
	// URL of the request
	URL string
	// HTTP method of the request
	Method string
	// Cached map to be stored
	CacheKey []byte
}

// marshalCachedMap converts a map of cached values to a byte slice
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

// marshal returns the marshalled cacheRequest as a byte slice
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
		// If the request method is GET and there are cached fields, the entire URL cannot be used as the cache key.
		// Instead, the cacheKey is used as the cache key.
		if cr.CacheKey != nil {
			return cr.CacheKey, nil
		} else {
			return []byte(r.URL()), nil
		}
	}

	return json.Marshal(cr)
}

// Hash returns the hashed value of the request's cache data as a string
func (r Request) Hash() (string, error) {
	cacheBody, err := r.marshal()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha1.Sum(cacheBody)), nil
}

// newRawRequest creates a new HTTP request with a given timeout duration
func newRawRequest(timeout time.Duration) (*http.Request, context.CancelFunc) {
	req := new(http.Request)
	req.Header = make(http.Header)

	if timeout <= 0 {
		return req, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	newReq := req.WithContext(ctx)

	return newReq, cancel
}

// NewRequestWithTimeout returns a new Request instance with a given timeout duration
func NewRequestWithTimeout(timeout time.Duration) *Request {
	req := new(Request)

	req.req, req.cancel = newRawRequest(timeout)

	req.timeout = timeout

	return req
}

// NewRequest returns a new Request instance
func NewRequest() *Request {
	return NewRequestWithTimeout(0)
}
