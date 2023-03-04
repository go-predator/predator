/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   craw.go
 * @Created At:  2021-07-23 08:52:17
 * @Modified At: 2023-03-04 10:46:25
 * @Modified By: thepoy
 */

package predator

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-predator/log"
	pctx "github.com/go-predator/predator/context"
	"github.com/go-predator/predator/html"
	"github.com/go-predator/predator/json"
	"github.com/go-predator/predator/proxy"
)

// HandleRequest is used to patch the request
type HandleRequest func(r *Request) error

// HandleResponse is used to handle the response
type HandleResponse func(r *Response) error

// HandleHTML is used to process html
type HandleHTML func(he *html.HTMLElement, r *Response) error

type HandleJSON func(j json.JSONResult, r *Response) error

// HTMLParser is used to parse html
type HTMLParser struct {
	Selector string
	Handle   HandleHTML
}

// JSONParser is used to parse json
type JSONParser struct {
	strict bool
	Handle HandleJSON
}

// CustomRandomBoundary generates a custom boundary
type CustomRandomBoundary func() string

type CacheCondition func(r *Response) bool

type ProxyInvalidCondition func(r *Response) error

type ComplementProxyPool func() []string

// Crawler is the provider of crawlers
type Crawler struct {
	lock *sync.RWMutex
	// UserAgent is the User-Agent string used by HTTP requests
	UserAgent  string
	retryCount uint32
	// Retry condition, the crawler will retry only
	// if it returns true
	retryCondition RetryCondition
	client         *http.Client

	//  Priority: rawCookies > cookies
	rawCookies string
	cookies    map[string]string

	goPool *Pool

	proxyURLPool          []string
	proxyInvalidCondition ProxyInvalidCondition

	complementProxyPool ComplementProxyPool
	requestCount        uint32
	responseCount       uint32
	// TODO: 在多协程中这个上下文管理可以用来退出或取消多个协程
	Context context.Context

	// Cache successful response
	cache Cache
	// List of fields to be cached in the request body, and
	// the combination of these fields can represent the unique
	// request body.
	// The fewer fields the better.
	cacheFields    []CacheField
	cacheCondition CacheCondition

	requestHandler []HandleRequest

	// Array of functions to handle the response
	responseHandler []HandleResponse
	// Array of functions to handle parsed html
	htmlHandler []*HTMLParser
	jsonHandler []*JSONParser

	// 对所有 Request 有效的超时控制
	timeout time.Duration

	// 记录远程/服务器地址
	recordRemoteAddr bool

	wg *sync.WaitGroup

	log *log.Logger
}

// NewCrawler creates a new Crawler instance with some CrawlerOptions
func NewCrawler(opts ...CrawlerOption) *Crawler {
	c := new(Crawler)

	c.UserAgent = "Predator"

	c.client = &http.Client{
		Transport: http.DefaultTransport,
	}

	for _, op := range opts {
		op(c)
	}

	// If there is `DEBUG` in the environment variable and `c.log` is nil,
	// create a logger with a level of `DEBUG`
	if c.log == nil && log.IsDebug() {
		c.log = log.NewLogger(
			log.DEBUG,
			log.ToConsole(),
			2,
		)
	}

	c.lock = &sync.RWMutex{}

	c.Context = context.Background()

	capacityState := c.goPool != nil

	if c.log != nil {
		if capacityState {
			c.Info("concurrent",
				log.Arg{Key: "state", Value: capacityState},
				log.Arg{Key: "capacity", Value: c.goPool.capacity},
				log.Arg{Key: "timeout", Value: c.timeout.String()},
			)
		} else {
			c.Info("concurrent",
				log.Arg{Key: "state", Value: capacityState},
				log.Arg{Key: "timeout", Value: c.timeout.String()},
			)
		}
	}

	if c.log != nil && c.goPool != nil {
		c.goPool.log = c.log
	}

	return c
}

// Clone creates an exact copy of a Crawler without callbacks.
func (c *Crawler) Clone() *Crawler {
	var (
		pool *Pool
		err  error
	)
	if c.goPool == nil {
		pool = nil
	} else {
		pool, err = NewPool(c.goPool.capacity)
		if err != nil {
			c.FatalOrPanic(err)
		}
	}
	return &Crawler{
		lock:             c.lock,
		UserAgent:        c.UserAgent,
		retryCount:       c.retryCount,
		retryCondition:   c.retryCondition,
		client:           c.client,
		cookies:          c.cookies,
		rawCookies:       c.rawCookies,
		goPool:           pool,
		proxyURLPool:     c.proxyURLPool,
		Context:          c.Context,
		cache:            c.cache,
		cacheCondition:   c.cacheCondition,
		cacheFields:      c.cacheFields,
		requestHandler:   make([]HandleRequest, 0, 5),
		responseHandler:  make([]HandleResponse, 0, 5),
		htmlHandler:      make([]*HTMLParser, 0, 5),
		jsonHandler:      make([]*JSONParser, 0, 1),
		recordRemoteAddr: c.recordRemoteAddr,
		wg:               &sync.WaitGroup{},
		log:              c.log,
		timeout:          c.timeout,
	}
}

/************************* http 请求方法 ****************************/

func (c *Crawler) request(method, URL string, body []byte, cachedMap map[string]string, reqHeader http.Header, ctx pctx.Context, isChained bool) error {
	defer func() {
		if c.goPool != nil {
			if err := recover(); err != nil {
				c.FatalOrPanic(fmt.Errorf("worker panic: %s", err))
			}
		}
	}()

	var err error

	u := AcquireURL()
	err = u.UnmarshalBinary([]byte(URL))
	if err != nil {
		return err
	}

	var request *Request
	if c.timeout <= 0 {
		request = NewRequest()
	} else {
		request = NewRequestWithTimeout(c.timeout)
	}

	if reqHeader == nil {
		reqHeader = make(http.Header)
	}

	if reqHeader.Get("User-Agent") == "" {
		reqHeader.Set("User-Agent", c.UserAgent)
	}

	request.req.Header = reqHeader
	request.req.URL = u
	request.req.Method = method

	if ctx == nil {
		ctx, err = pctx.AcquireCtx()
		if err != nil {
			if c.log != nil {
				c.Error(err)
			}
			return err
		}
	}

	request.Ctx = ctx
	request.body = body
	request.cachedMap = cachedMap
	request.ID = atomic.AddUint32(&c.requestCount, 1)
	request.crawler = c

	if c.rawCookies != "" {
		request.ParseRawCookie(c.rawCookies)
		request.req.Header.Set("Cookie", c.rawCookies)
		if c.log != nil {
			c.Debug("cookies is set", log.Arg{Key: "cookies", Value: c.rawCookies})
		}
	} else {
		if c.cookies != nil {
			request.AddRookies(c.cookies)
		}
	}

	if c.log != nil {
		c.Info(
			"requesting",
			log.Arg{Key: "request_id", Value: atomic.LoadUint32(&request.ID)},
			log.Arg{Key: "method", Value: request.Method()},
			log.Arg{Key: "url", Value: request.URL()},
			log.Arg{Key: "timeout", Value: request.timeout.String()},
		)
	}

	if c.goPool != nil {
		c.wg.Add(1)
		task := &Task{
			crawler:   c,
			req:       request,
			isChained: isChained,
		}
		err = c.goPool.Put(task)
		if err != nil {
			if c.log != nil {
				c.Error(err)
			}
			return err
		}
		return nil
	}

	err = c.prepare(request, isChained)
	if err != nil {
		return err
	}

	return nil
}

func (c *Crawler) prepare(request *Request, isChained bool) (err error) {
	if c.goPool != nil {
		defer c.wg.Done()
	}

	err = c.processRequestHandler(request)
	if err != nil {
		return
	}
	if request.cancel != nil {
		defer request.cancel()
	}

	if request.abort {
		if c.log != nil {
			c.Debug("the request is aborted", log.Arg{Key: "request_id", Value: atomic.LoadUint32(&request.ID)})
		}
		return
	}

	if request.Method() == "" {
		c.Fatal("请求不正确", log.Arg{Key: "id", Value: atomic.LoadUint32(&request.ID)})
	}

	if request.Ctx.Length() > 0 {
		if c.log != nil {
			c.Debug("using context", log.Arg{Key: "context", Value: request.Ctx.String()})
		}
	}

	var response *Response

	var key string

	if c.cache != nil {
		key, err = request.Hash()
		if err != nil {
			if c.log != nil {
				c.Error(err)
			}
			return
		}

		if c.log != nil {
			c.Debug(
				"generate cache key",
				log.Arg{Key: "request_id", Value: atomic.LoadUint32(&request.ID)},
				log.Arg{Key: "cache_key", Value: key},
			)
		}

		response, err = c.checkCache(key)
		if err != nil {
			return
		}

		if response != nil && c.log != nil {
			c.Debug("response is in the cache",
				log.Arg{Key: "request_id", Value: atomic.LoadUint32(&request.ID)},
				log.Arg{Key: "cache_key", Value: key},
			)
		}
	}

	// A new request is issued when there
	// is no response from the cache
	if response == nil {
		response, err = c.do(request)
		if err != nil {
			return
		}

		// Cache the response from the request if the statuscode is 20X
		if c.cache != nil && c.cacheCondition(response) && key != "" {
			cacheVal, err := response.Marshal()
			if err != nil {
				if c.log != nil {
					c.Error(err)
				}
				return err
			}

			if cacheVal != nil {
				c.lock.Lock()
				err = c.cache.Cache(key, cacheVal)
				if err != nil {
					if c.log != nil {
						c.Error(err)
					}
					return err
				}
				c.lock.Unlock()
			}
		}
	} else {
		response.Request = request
		response.Ctx = request.Ctx
	}

	if response.StatusCode == StatusFound {
		location := response.resp.Header.Get("Location")

		if c.log != nil {
			c.Info("response",
				log.Arg{Key: "method", Value: request.Method()},
				log.Arg{Key: "status_code", Value: response.StatusCode},
				log.Arg{Key: "content_length", Value: response.ContentLength()},
				log.Arg{Key: "location", Value: location},
				log.Arg{Key: "request_id", Value: atomic.LoadUint32(&request.ID)},
			)
		}
	} else {
		if c.log != nil {
			l := c.log.L.Info().
				Str("method", request.Method()).
				Int("status_code", int(response.StatusCode)).
				Uint64("content_length", response.ContentLength())

			if !response.FromCache {
				if c.ProxyPoolAmount() > 0 {
					l = l.Str("proxy", response.ClientIP())
				} else {
					if c.recordRemoteAddr {
						l = l.Str("server_addr", response.ClientIP())
					}
				}
			}

			l.Bool("from_cache", response.FromCache).
				Uint32("request_id", atomic.LoadUint32(&request.ID)).
				Msg("response")
		}
	}

	err = c.processResponseHandler(response)
	if err != nil {
		return
	}

	if !response.invalid {
		err = c.processHTMLHandler(response)
		if err != nil {
			return
		}

		err = c.processJSONHandler(response)
		if err != nil {
			return
		}
	}

	return
}

func (c *Crawler) FatalOrPanic(err error) {
	if c.log != nil {
		c.Fatal(err)
	} else {
		panic(err)
	}
}

func (c *Crawler) checkCache(key string) (*Response, error) {
	var err error
	cachedBody, ok := c.cache.IsCached(key)
	if !ok {
		return nil, nil
	}

	resp := new(Response)
	err = resp.Unmarshal(cachedBody)
	if err != nil {
		if c.log != nil {
			c.Error(err)
		}
		return nil, err
	}
	resp.FromCache = true
	return resp, nil
}

func (c *Crawler) proxy(req *Request) proxy.ProxyFunc {
	rand := rand.New(rand.NewSource(time.Now().UnixNano()))
	f, addr := proxy.Proxy(c.proxyURLPool[rand.Intn(len(c.proxyURLPool))])
	req.proxyUsed = addr

	return f
}

func (c *Crawler) processProxyError(req *Request, err error) error {
	if _, ok := proxy.IsProxyError(err); !ok {
		return err
	}

	pe := err.(proxy.ProxyErr)

	c.Warning("proxy is invalid",
		log.Arg{Key: "proxy", Value: req.proxyUsed},
		log.Arg{Key: "proxy_pool", Value: c.proxyURLPool},
		log.Arg{Key: "error", Value: pe.Err},
	)

	err = c.removeInvalidProxy(req.proxyUsed)
	if err != nil {
		c.FatalOrPanic(err)
	}

	c.Info("removed invalid proxy",
		log.Arg{Key: "invalid_proxy", Value: req.proxyUsed},
		log.Arg{Key: "new_proxy_pool", Value: c.proxyURLPool},
	)

	return nil
}

func (c *Crawler) preprocessResponseError(req *Request, err error) error {
	if err == nil {
		return nil
	}

	c.Debug("raw error", log.Arg{Key: "error", Value: err})

	e := &url.Error{}

	if !errors.As(err, &e) {
		return err
	}

	opErr := &net.OpError{}

	if !errors.As(e.Err, &opErr) {
		if errors.Is(e.Err, context.DeadlineExceeded) {
			return fmt.Errorf("%w -> %w", ErrTimeout, e.Err)
		}

		return err
	}

	if strings.HasPrefix(opErr.Op, "socks") {
		return proxy.UnexpectedProtocol(req.proxyUsed, proxy.SOCKS5)
	}

	if strings.HasPrefix(opErr.Op, "proxyconnect") {
		re := tls.RecordHeaderError{}
		if errors.As(opErr.Err, &re) {
			if re.Msg == "first record does not look like a TLS handshake" {
				return proxy.UnexpectedProtocol(req.proxyUsed, proxy.HTTPS)
			}
		}

		scErr := &os.SyscallError{}
		if errors.As(opErr.Err, &scErr) {
			// no := scErr.Err.(syscall.Errno)
			// return scErr
			return proxy.NewProxyError(req.proxyUsed, scErr)
		}
	}

	return err
}

func (c *Crawler) do(request *Request) (*Response, error) {
	c.client.Timeout = request.timeout
	if request.checkRedirect != nil {
		c.client.CheckRedirect = request.checkRedirect
	} else {
		c.client.CheckRedirect = defaultCheckRedirect
	}

	if len(c.proxyURLPool) > 0 {
		c.client.Transport = &http.Transport{
			Proxy: c.proxy(request),
		}

		c.Debug("request infomation", log.Arg{Key: "header", Value: request.Header()}, log.Arg{Key: "proxy", Value: request.proxyUsed})
	} else {
		c.Debug("request infomation", log.Arg{Key: "header", Value: request.Header()})
	}

	if c.recordRemoteAddr {
		trace := &httptrace.ClientTrace{
			GotConn: func(connInfo httptrace.GotConnInfo) {
				request.req.RemoteAddr = connInfo.Conn.RemoteAddr().String()
			},
		}

		request.req = request.req.WithContext(httptrace.WithClientTrace(request.req.Context(), trace))
	}

	request.WithBody()

	var err error

	resp, err := c.client.Do(request.req)
	if err != nil {
		e := c.preprocessResponseError(request, err)

		// TODO: 重写 Tranport.Dial，对 proxy 请求的错误进行封装。
		if errors.Is(e, ErrTimeout) {
			if request.proxyUsed != "" {
				c.Warning("the connection timed out, but it was not possible to determine if the error was caused by a timeout with the proxy server or a timeout between the proxy server and the target server")
			}

			// re-request if the request timed out.
			// re-request 3 times by default when the request times out.

			// if you are using a proxy, the timeout error is probably
			// because the proxy is invalid, and it is recommended
			// to try a new proxy
			if c.retryCount == 0 {
				c.retryCount = 3
			}

			c.Error(err,
				log.Arg{Key: "timeout", Value: request.timeout.String()},
				log.Arg{Key: "request_id", Value: atomic.LoadUint32(&request.ID)},
				log.Arg{Key: "proxy", Value: request.proxyUsed},
			)

			if atomic.LoadUint32(&request.retryCounter) < c.retryCount {
				c.retryPrepare(request)
				return c.do(request)
			}

			return nil, e
		}

		e = c.processProxyError(request, e)

		if e == nil {
			return c.do(request)
		}

		c.Error(e)
		return nil, e
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.Error(err)
		return nil, err
	}

	response := new(Response)
	response.StatusCode = StatusCode(resp.StatusCode)
	response.Body = append(response.Body, body...)
	response.Ctx = request.Ctx
	response.Request = request
	response.header = resp.Header.Clone()
	response.clientIP = request.req.RemoteAddr
	response.isJSON = strings.Contains(strings.ToLower(response.ContentType()), "application/json")

	if response.isJSON {
		result := json.ParseBytesToJSON(body)
		response.json = &result
	}

	// if response.StatusCode == StatusOK && len(response.Body) == 0 {
	// 	response.StatusCode = 0
	// }

	c.Debug("response header", log.Arg{Key: "header", Value: resp.Header})

	// Only count successful responses
	atomic.AddUint32(&c.responseCount, 1)

	if c.retryCount > 0 && atomic.LoadUint32(&request.retryCounter) < c.retryCount {
		if c.retryCondition != nil && c.retryCondition(response) {
			c.Warning("the response meets the retry condition and will be retried soon")
			c.retryPrepare(request)
			return c.do(request)
		}
	}

	return response, nil
}

func (c *Crawler) retryPrepare(request *Request) {
	atomic.AddUint32(&request.retryCounter, 1)
	c.Info(
		"retrying",
		log.Arg{Key: "retry_count", Value: atomic.LoadUint32(&request.retryCounter)},
		log.Arg{Key: "method", Value: request.Method()},
		log.Arg{Key: "url", Value: request.URL()},
		log.Arg{Key: "request_id", Value: atomic.LoadUint32(&request.ID)},
	)
}

type createPostBody func(requestData map[string]string) []byte

func createBody(requestData map[string]string) []byte {
	if requestData == nil {
		return nil
	}
	form := url.Values{}
	for k, v := range requestData {
		form.Add(k, v)
	}
	return []byte(form.Encode())
}

func (c *Crawler) get(URL string, header http.Header, ctx pctx.Context, isChained bool, cacheFields ...CacheField) error {
	// Parse the query parameters and create a `cachedMap` based on `cacheFields`
	u, err := url.Parse(URL)
	if err != nil {
		c.Error(err)
		return err
	}

	params := u.Query()
	var cachedMap map[string]string
	if len(cacheFields) > 0 {
		cachedMap = make(map[string]string)
		for _, field := range cacheFields {
			if field.code != queryParam {
				c.FatalOrPanic(ErrNotAllowedCacheFieldType)
			}

			key, value, err := addQueryParamCacheField(params, field)
			if err != nil {
				c.FatalOrPanic(err)
			}

			if field.prepare != nil {
				value = field.prepare(value)
			}

			cachedMap[key] = value
		}

		c.Debug("use some specified cache fields", log.Arg{Key: "cached_map", Value: cachedMap})
	}

	return c.request(MethodGet, URL, nil, cachedMap, header, ctx, isChained)
}

// Get is used to send GET requests
func (c *Crawler) Get(URL string) error {
	return c.GetWithCtx(URL, nil)
}

// GetWithCtx is used to send GET requests with a context
func (c *Crawler) GetWithCtx(URL string, ctx pctx.Context) error {
	return c.get(URL, nil, ctx, false, c.cacheFields...)
}

func (c *Crawler) post(URL string, requestData map[string]string, header http.Header, ctx pctx.Context, isChained bool, createBodyFunc createPostBody, cacheFields ...CacheField) error {
	var cachedMap map[string]string
	if len(cacheFields) > 0 {
		cachedMap = make(map[string]string)

		var queryParams url.Values
		for _, field := range cacheFields {
			var (
				err        error
				key, value string
			)

			switch field.code {
			case queryParam:
				if queryParams == nil {
					u, err := url.Parse(URL)
					if err != nil {
						c.FatalOrPanic(err)
					}

					queryParams = u.Query()
				}

				key, value, err = addQueryParamCacheField(queryParams, field)
				if field.prepare != nil {
					value = field.prepare(value)
				}
			case requestBodyParam:
				if val, ok := requestData[field.Field]; ok {
					key, value = field.String(), val
					if field.prepare != nil {
						value = field.prepare(value)
					}
				} else {
					keys := make([]string, 0, len(requestData))
					for k := range requestData {
						keys = append(keys, k)
					}

					err = fmt.Errorf("there is no such field [%s] in the request body: %v", field.Field, keys)
				}
			default:
				err = ErrInvalidCacheTypeCode
			}

			if err != nil {
				c.FatalOrPanic(err)
			}

			cachedMap[key] = value
		}

		c.Debug("use some specified cache fields", log.Arg{Key: "cached_map", Value: cachedMap})
	}

	if len(header) == 0 {
		header = make(http.Header)
	}
	if _, ok := header["Content-Type"]; !ok {
		// use default `Content-Type`
		header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	if createBodyFunc == nil {
		createBodyFunc = createBody
	}

	return c.request(MethodPost, URL, createBodyFunc(requestData), cachedMap, header, ctx, isChained)
}

// Post is used to send POST requests
func (c *Crawler) Post(URL string, requestData map[string]string) error {
	return c.post(URL, requestData, nil, nil, false, nil, c.cacheFields...)
}

// PostWithCtx is used to send POST requests with a context
func (c *Crawler) PostWithCtx(URL string, requestData map[string]string, ctx pctx.Context) error {
	return c.post(URL, requestData, nil, ctx, false, nil, c.cacheFields...)
}

// PostWithCreateBodyFunc is used to send POST requests with a createBodyFunc
//
// Why was this method added?
//
// Because there are many request bodies constructed by JavaScript that are
// not legal request bodies, which is very common in China.
func (c *Crawler) PostWithCreateBodyFunc(URL string, requestData map[string]string, createBodyFunc createPostBody) error {
	return c.post(URL, requestData, nil, nil, false, createBodyFunc, c.cacheFields...)
}

// PostWithCtxAndCreateBodyFunc is used to send POST requests with a context
// and a createBodyFunc
func (c *Crawler) PostWithCtxAndCreateBodyFunc(URL string, requestData map[string]string, ctx pctx.Context, createBodyFunc createPostBody) error {
	return c.post(URL, requestData, nil, ctx, false, createBodyFunc, c.cacheFields...)
}

func (c *Crawler) createJSONBody(requestData map[string]any) []byte {
	if requestData == nil {
		return nil
	}
	body, err := json.Marshal(requestData)
	if err != nil {
		c.FatalOrPanic(err)
	}
	return body
}

func (c *Crawler) postJSON(URL string, requestData map[string]any, header http.Header, ctx pctx.Context, isChained bool, cacheFields ...CacheField) error {
	body := c.createJSONBody(requestData)

	var cachedMap map[string]string
	if len(cacheFields) > 0 {
		cachedMap = make(map[string]string)
		bodyJson := json.ParseBytesToJSON(body)

		var queryParams url.Values

		for _, field := range cacheFields {
			var (
				err        error
				key, value string
			)

			switch field.code {
			case queryParam:
				if queryParams == nil {
					u, err := url.Parse(URL)
					if err != nil {
						c.FatalOrPanic(err)
					}

					queryParams = u.Query()
				}

				key, value, err = addQueryParamCacheField(queryParams, field)
				if field.prepare != nil {
					value = field.prepare(value)
				}
			case requestBodyParam:
				if !bodyJson.Get(field.Field).Exists() {
					m := bodyJson.Map()
					var keys = make([]string, 0, len(m))
					for k := range m {
						keys = append(keys, k)
					}
					err = fmt.Errorf("there is no such field [%s] in the request body: %v", field, keys)
				} else {
					key, value = field.String(), bodyJson.Get(field.Field).String()
					if field.prepare != nil {
						value = field.prepare(value)
					}
				}
			default:
				err = ErrInvalidCacheTypeCode
			}

			if err != nil {
				c.FatalOrPanic(err)
			}

			cachedMap[key] = value
		}

		c.Debug("use some specified cache fields", log.Arg{Key: "cached_map", Value: cachedMap})
	}

	if len(header) == 0 {
		header = make(http.Header)
	}
	header.Set("Content-Type", "application/json")

	return c.request(MethodPost, URL, body, cachedMap, header, ctx, isChained)
}

// PostJSON is used to send POST requests whose content-type is json
func (c *Crawler) PostJSON(URL string, requestData map[string]any, ctx pctx.Context) error {
	return c.postJSON(URL, requestData, nil, ctx, false, c.cacheFields...)
}

func (c *Crawler) postMultipart(URL string, mfw *MultipartFormWriter, header http.Header, ctx pctx.Context, isChained bool, cacheFields ...CacheField) error {
	var cachedMap map[string]string
	if len(cacheFields) > 0 {
		cachedMap = make(map[string]string)

		var queryParams url.Values

		for _, field := range cacheFields {
			var (
				err        error
				key, value string
			)

			switch field.code {
			case queryParam:
				if queryParams == nil {
					u, err := url.Parse(URL)
					if err != nil {
						c.FatalOrPanic(err)
					}

					queryParams = u.Query()
				}

				key, value, err = addQueryParamCacheField(queryParams, field)
				if field.prepare != nil {
					value = field.prepare(value)
				}
			case requestBodyParam:
				if val, ok := mfw.cachedMap[field.Field]; ok {
					key, value = field.String(), val
					if field.prepare != nil {
						value = field.prepare(value)
					}
				} else {
					var keys = make([]string, 0, len(mfw.cachedMap))
					for k := range mfw.cachedMap {
						keys = append(keys, k)
					}
					err = fmt.Errorf("there is no such field [%s] in the request body: %v", field, keys)
				}
			default:
				err = ErrInvalidCacheTypeCode
			}

			if err != nil {
				c.FatalOrPanic(err)
			}

			cachedMap[key] = value
		}

		c.Debug("use some specified cache fields", log.Arg{Key: "cached_map", Value: cachedMap})
	}

	if len(header) == 0 {
		header = make(http.Header)
	}

	contentType, buf := NewMultipartForm(mfw)

	header.Set("Content-Type", contentType)

	return c.request(MethodPost, URL, buf.Bytes(), cachedMap, header, ctx, isChained)
}

// PostMultipart is used to send POST requests whose content-type is `multipart/form-data`
func (c *Crawler) PostMultipart(URL string, mfw *MultipartFormWriter, ctx pctx.Context) error {
	return c.postMultipart(URL, mfw, nil, ctx, false, c.cacheFields...)
}

// PostRaw is used to send POST requests whose content-type is not in [json, `application/x-www-form-urlencoded`, `multipart/form-data`]
func (c *Crawler) PostRaw(URL string, body []byte, ctx pctx.Context) error {
	cachedMap := map[string]string{
		"cache": string(body),
	}
	return c.request(MethodPost, URL, body, cachedMap, nil, ctx, false)
}

/************************* Public methods ****************************/

// ClearCache will clear all cache
func (c *Crawler) ClearCache() error {
	if c.cache == nil {
		c.Error(ErrNoCache)
		return ErrNoCache
	}
	if c.log != nil {
		c.Warning("clear all cache")
	}
	return c.cache.Clear()
}

func (c *Crawler) ConcurrencyState() bool {
	return c.goPool != nil
}

/************************* 公共注册方法 ****************************/

// BeforeRequest used to process requests, such as
// setting headers, passing context, etc.
func (c *Crawler) BeforeRequest(f HandleRequest) {
	c.lock.Lock()
	if c.requestHandler == nil {
		// 一个 ccrawler 不应该有太多处理请求的方法，这里设置为 5 个，
		// 当不够时自动扩容
		c.requestHandler = make([]HandleRequest, 0, 5)
	}
	c.requestHandler = append(c.requestHandler, f)
	c.lock.Unlock()
}

// ParseHTML can parse html to find the data you need,
// and process the data
func (c *Crawler) ParseHTML(selector string, f HandleHTML) {
	c.lock.Lock()
	if c.htmlHandler == nil {
		// 一个 ccrawler 不应该有太多处理 html 的方法，这里设置为 5 个，
		// 当不够时自动扩容
		c.htmlHandler = make([]*HTMLParser, 0, 5)
	}
	c.htmlHandler = append(c.htmlHandler, &HTMLParser{selector, f})
	c.lock.Unlock()
}

// ParseJSON can parse json to find the data you need,
// and process the data.
//
// If you set `strict` to true, responses that do not contain
// `application/json` in the content-type of the response header will
// not be processed.
//
// It is recommended to do full processing of the json response in one
// call to `ParseJSON` instead of multiple calls to `ParseJSON`.
func (c *Crawler) ParseJSON(strict bool, f HandleJSON) {
	c.lock.Lock()
	if c.jsonHandler == nil {
		c.jsonHandler = make([]*JSONParser, 0, 1)
	}
	c.jsonHandler = append(c.jsonHandler, &JSONParser{strict, f})
	c.lock.Unlock()
}

// AfterResponse is used to process the response, this
// method should be used for the response body in non-html format
func (c *Crawler) AfterResponse(f HandleResponse) {
	c.lock.Lock()
	if c.responseHandler == nil {
		// 一个 ccrawler 不应该有太多处理响应的方法，这里设置为 5 个，
		// 当不够时自动扩容
		c.responseHandler = make([]HandleResponse, 0, 5)
	}
	c.responseHandler = append(c.responseHandler, f)
	c.lock.Unlock()
}

// ProxyPoolAmount returns the number of proxies in
// the proxy pool
func (c Crawler) ProxyPoolAmount() int {
	return len(c.proxyURLPool)
}

// Wait waits for the end of all concurrent tasks
func (c *Crawler) Wait() {
	c.wg.Wait()
	c.goPool.Close()
}

func (c *Crawler) SetProxyInvalidCondition(condition ProxyInvalidCondition) {
	c.proxyInvalidCondition = condition
}

func (c *Crawler) AddProxy(newProxy string) {
	c.lock.Lock()

	c.proxyURLPool = append(c.proxyURLPool, newProxy)

	c.lock.Unlock()
}

func (c *Crawler) AddCookie(key, val string) {
	c.lock.Lock()

	c.rawCookies += fmt.Sprintf("; %s=%s", key, val)

	c.lock.Unlock()
}

// SetConcurrency 使用并发，参数为要创建的协程池数量
func (c *Crawler) SetConcurrency(count uint64, blockPanic bool) {
	if c.goPool == nil {
		p, err := NewPool(count)
		if err != nil {
			panic(err)
		}
		p.blockPanic = blockPanic
		p.log = c.log

		c.goPool = p
		c.wg = new(sync.WaitGroup)
	} else {
		c.FatalOrPanic(errors.New("`c.goPool` is not nil"))
	}
}

func (c *Crawler) SetRetry(count uint32, cond RetryCondition) {
	c.retryCount = count
	c.retryCondition = cond
}

func (c *Crawler) SetCache(cc Cache, compressed bool, cacheCondition CacheCondition, cacheFields ...CacheField) {
	cc.Compressed(compressed)
	err := cc.Init()
	if err != nil {
		panic(err)
	}
	c.cache = cc
	if cacheCondition == nil {
		cacheCondition = func(r *Response) bool {
			return r.StatusCode/100 == 2
		}
	}
	c.cacheCondition = cacheCondition
	if len(cacheFields) > 0 {
		c.cacheFields = cacheFields
	} else {
		c.cacheFields = nil
	}
}

// ResetCacheFields 重新设置缓存字段。
//
// 通常在 c.Clone() 之后调用，以便在发送另一种请求时缓存响应。
func (c *Crawler) ResetCacheFields(cacheFields ...CacheField) {
	c.cacheFields = cacheFields
}

// 有时发出的请求不能缓存，可以用此方法关闭特定的 Crawler 实例的缓存。
//
// 通常用来关闭`Clone()`实例的缓存。
func (c *Crawler) UnsetCache() {
	if c.cache != nil {
		c.cache = nil

		if c.cacheCondition != nil {
			c.cacheCondition = nil
		}

		if c.cacheFields != nil {
			c.cacheFields = nil
		}
	}
}

func (c Crawler) Lock() {
	c.lock.Lock()
}

func (c Crawler) Unlock() {
	c.lock.Unlock()
}

func (c Crawler) RLock() {
	c.lock.RLock()
}

func (c Crawler) RUnlock() {
	c.lock.RUnlock()
}

/************************* 私有注册方法 ****************************/

func (c *Crawler) processRequestHandler(r *Request) error {
	var err error

	for _, f := range c.requestHandler {
		err = f(r)
		if err != nil {
			c.Error(err)
			return err
		}
	}

	return nil
}

func (c *Crawler) processResponseHandler(r *Response) error {
	var err error

	for _, f := range c.responseHandler {
		if r.invalid {
			break
		}
		err = f(r)
		if err != nil {
			c.Error(err)
			return err
		}
	}

	return err
}

func (c *Crawler) processJSONHandler(r *Response) error {
	if c.jsonHandler == nil {
		return nil
	}

	if len(c.jsonHandler) > 1 {
		if c.log != nil {
			c.Warning("it is recommended to do full processing of the json response in one call to `ParseJSON` instead of multiple calls to `ParseJSON`")
		}
	}

	var err error

	var result json.JSONResult
	if r.isJSON {
		result = *r.json
	} else {
		result = json.ParseBytesToJSON(r.Body)
	}

	for _, parser := range c.jsonHandler {
		if parser.strict {
			if !strings.Contains(strings.ToLower(r.ContentType()), "application/json") {
				if c.log != nil {
					c.Debug(
						`the "Content-Type" of the response header is not of the "json" type`,
						log.Arg{Key: "Content-Type", Value: r.ContentType()},
					)
				}
				continue
			}
		}
		err = parser.Handle(result, r)
		if err != nil {
			c.Error(err)
			return err
		}
	}

	return nil
}

func (c *Crawler) processHTMLHandler(r *Response) error {
	if len(c.htmlHandler) == 0 {
		return nil
	}

	if !strings.Contains(strings.ToLower(r.ContentType()), "html") {
		if c.log != nil {
			c.Debug(
				`the "Content-Type" of the response header is not of the "html" type`,
				log.Arg{Key: "Content-Type", Value: r.ContentType()},
			)
		}
		return nil
	}

	doc, err := html.ParseHTML(r.Body)
	if err != nil {
		if c.log != nil {
			c.Error(err)
		}
		return err
	}

	for _, parser := range c.htmlHandler {
		if r.invalid {
			break
		}

		i := 0
		doc.Find(parser.Selector).Each(func(_ int, s *goquery.Selection) {
			for _, n := range s.Nodes {
				parser.Handle(html.NewHTMLElementFromSelectionNode(s, n, i), r)
				i++
			}
		})
	}

	return nil
}

// removeInvalidProxy 只有在使用代理池且当前请求使用的代理来自于代理池时，才能真正删除失效代理
func (c *Crawler) removeInvalidProxy(proxyAddr string) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.ProxyPoolAmount() == 0 {
		return ErrEmptyProxyPool
	}

	if c.ProxyPoolAmount() == 1 && c.complementProxyPool != nil {
		newProxyPool := c.complementProxyPool()
		c.proxyURLPool = append(c.proxyURLPool, newProxyPool...)
		c.Info(
			"a new proxy pool has replaced to the old proxy pool",
			log.Arg{Key: "new_proxy_pool", Value: newProxyPool},
		)
	}

	targetIndex := -1
	for i, p := range c.proxyURLPool {
		if p == proxyAddr {
			targetIndex = i
			break
		}
	}

	if targetIndex >= 0 {
		c.proxyURLPool = append(
			c.proxyURLPool[:targetIndex],
			c.proxyURLPool[targetIndex+1:]...,
		)

		if c.log != nil {
			c.Debug(
				"invalid proxy have been deleted from the proxy pool",
				log.Arg{Key: "proxy", Value: proxyAddr},
			)
		}

		if len(c.proxyURLPool) == 0 {
			return ErrEmptyProxyPool
		}
	} else {
		// 并发时可能也会存在找不到失效的代理的情况，这时不能返回 error
		if c.goPool != nil {
			return nil
		}

		// 没有在代理池中找到失效代理，这个代理来路不明，一样报错
		return fmt.Errorf("%w: %s", proxy.ErrUnkownProxyIP, proxyAddr)
	}

	return nil
}

func (c *Crawler) Debug(msg string, args ...log.Arg) {
	if c.log != nil {
		c.log.Debug(msg, args...)
	}
}

func (c *Crawler) Info(msg string, args ...log.Arg) {
	if c.log != nil {
		c.log.Info(msg, args...)
	}
}

func (c *Crawler) Warning(msg string, args ...log.Arg) {
	if c.log != nil {
		c.log.Warning(msg, args...)
	}
}

func (c *Crawler) Error(err any, args ...log.Arg) {
	if c.log != nil {
		c.log.Error(err, args...)
	}
}

func (c *Crawler) Fatal(err any, args ...log.Arg) {
	if c.log != nil {
		c.log.Fatal(err, args...)
	}
}
