/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: craw.go
 * @Created: 2021-07-23 08:52:17
 * @Modified:  2021-11-24 20:42:18
 */

package predator

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/PuerkitoBio/goquery"
	pctx "github.com/go-predator/predator/context"
	"github.com/go-predator/predator/html"
	"github.com/go-predator/predator/json"
	"github.com/go-predator/predator/proxy"
	"github.com/rs/zerolog"
	"github.com/tidwall/gjson"
	"github.com/valyala/fasthttp"
)

var (
	ErrNoCacheSet    = errors.New("no cache set")
	ErrRequestFailed = errors.New("request failed")
	ErrTimeout       = errors.New("timeout, and it is recommended to try a new proxy if you are using a proxy pool")
)

// HandleRequest is used to patch the request
type HandleRequest func(r *Request)

// HandleResponse is used to handle the response
type HandleResponse func(r *Response)

// HandleHTML is used to process html
type HandleHTML func(he *html.HTMLElement, r *Response)

// HTMLParser is used to parse html
type HTMLParser struct {
	Selector string
	Handle   HandleHTML
}

// CustomRandomBoundary generates a custom boundary
type CustomRandomBoundary func() string

type CacheCondition func(r Response) bool

type ProxyInvalidCondition func(r Response) error

type ComplementProxyPool func() []string

// Crawler is the provider of crawlers
type Crawler struct {
	lock *sync.RWMutex
	// UserAgent is the User-Agent string used by HTTP requests
	UserAgent  string
	retryCount uint32
	// Retry conditions, the crawler will retry only
	// if it returns true
	retryConditions       RetryConditions
	client                *fasthttp.Client
	cookies               map[string]string
	goPool                *Pool
	proxyURLPool          []string
	proxyInvalidCondition ProxyInvalidCondition
	proxyInUse            string
	complementProxyPool   ComplementProxyPool
	// TODO: 动态获取代理
	// dynamicProxyFunc AcquireProxies
	requestCount  uint32
	responseCount uint32
	// 在多协程中这个上下文管理可以用来退出或取消多个协程
	Context context.Context

	// Cache successful response
	cache Cache
	// List of fields to be cached in the request body, and
	// the combination of these fields can represent the unique
	// request body.
	// The fewer fields the better.
	cacheFields    []string
	cacheCondition CacheCondition

	requestHandler []HandleRequest

	// 响应后处理响应
	responseHandler []HandleResponse
	// 响应后处理 html
	htmlHandler []*HTMLParser

	wg *sync.WaitGroup

	log *zerolog.Logger
}

// NewCrawler creates a new Crawler instance with some CrawlerOptions
func NewCrawler(opts ...CrawlerOption) *Crawler {
	c := new(Crawler)

	c.UserAgent = "Predator"

	c.client = new(fasthttp.Client)

	for _, op := range opts {
		op(c)
	}

	c.lock = &sync.RWMutex{}

	c.Context = context.Background()

	capacityState := c.goPool != nil

	if c.log != nil {
		if capacityState {
			c.log.Info().
				Bool("state", capacityState).
				Uint64("capacity", c.goPool.capacity).
				Msg("concurrent")
		} else {
			c.log.Info().
				Bool("state", capacityState).
				Msg("concurrent")
		}
	}

	if c.log != nil && c.goPool != nil {
		c.goPool.log = c.log
	}

	return c
}

// Clone creates an exact copy of a Crawler without callbacks.
func (c *Crawler) Clone() *Crawler {
	return &Crawler{
		lock:            c.lock,
		UserAgent:       c.UserAgent,
		retryCount:      c.retryCount,
		retryConditions: c.retryConditions,
		client:          c.client,
		cookies:         c.cookies,
		goPool:          c.goPool,
		proxyURLPool:    c.proxyURLPool,
		Context:         c.Context,
		cache:           c.cache,
		cacheFields:     c.cacheFields,
		requestHandler:  make([]HandleRequest, 0, 5),
		responseHandler: make([]HandleResponse, 0, 5),
		htmlHandler:     make([]*HTMLParser, 0, 5),
		wg:              &sync.WaitGroup{},
		log:             c.log,
	}
}

/************************* http 请求方法 ****************************/

func (c *Crawler) request(method, URL string, body []byte, cachedMap, headers map[string]string, ctx pctx.Context, isChained bool) error {
	defer func() {
		if c.goPool != nil {
			if err := recover(); err != nil {
				c.FatalOrPanic(fmt.Errorf("worker panic: %s", err))
			}
		}
	}()

	var err error

	reqHeaders := new(fasthttp.RequestHeader)
	reqHeaders.SetMethod(method)

	u, err := url.Parse(URL)
	if err != nil {
		if c.log != nil {
			c.log.Error().Caller().Err(err).Send()
		}
		return err
	}
	reqHeaders.SetRequestURI(u.RequestURI())

	reqHeaders.Set("User-Agent", c.UserAgent)
	for k, v := range headers {
		reqHeaders.Set(k, v)
	}
	if c.cookies != nil {
		for k, v := range c.cookies {
			reqHeaders.SetCookie(k, v)
		}
		if c.log != nil {
			c.log.Debug().
				Bytes("cookies", reqHeaders.Peek("Cookie")).
				Msg("cookies is set")
		}
	}

	if ctx == nil {
		ctx, err = pctx.AcquireCtx()
		if err != nil {
			if c.log != nil {
				c.log.Error().Caller().Err(err).Send()
			}
			return err
		}
	}

	request := AcquireRequest()
	request.URL = URL
	request.Method = method
	request.Headers = reqHeaders
	request.Ctx = ctx
	request.Body = body
	request.cachedMap = cachedMap
	request.ID = atomic.AddUint32(&c.requestCount, 1)
	request.crawler = c

	// TODO: 链式请求用 go pool 会阻塞？
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
				c.log.Error().Caller().Err(err).Send()
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

	c.processRequestHandler(request)

	if request.abort {
		if c.log != nil {
			c.log.Debug().
				Uint32("request_id", atomic.LoadUint32(&request.ID)).
				Msg("the request is aborted")
		}
		return
	}

	if c.log != nil {
		c.log.Info().
			Uint32("request_id", atomic.LoadUint32(&request.ID)).
			Str("method", request.Method).
			Str("url", request.URL).
			Str("timeout", request.timeout.String()).
			Msg("requesting")
	}

	if request.Ctx.Length() > 0 {
		if c.log != nil {
			c.log.Debug().
				RawJSON("context", request.Ctx.Bytes()).
				Msg("using context")
		}
	}

	var response *Response

	var key string

	if c.cache != nil {
		key, err = request.Hash()
		if err != nil {
			if c.log != nil {
				c.log.Error().Caller().Err(err).Send()
			}
			return
		}

		if c.log != nil {
			c.log.Debug().
				Uint32("request_id", atomic.LoadUint32(&request.ID)).
				Str("cache_key", key).
				Msg("generate cache key")
		}

		response, err = c.checkCache(key)
		if err != nil {
			return
		}

		if response != nil && c.log != nil {
			c.log.Debug().
				Uint32("request_id", atomic.LoadUint32(&request.ID)).
				Str("cache_key", key).
				Msg("response is in the cache")
		}
	}

	var rawResp *fasthttp.Response
	// A new request is issued when there
	// is no response from the cache
	if response == nil {
		response, rawResp, err = c.do(request)
		if err != nil {
			return
		}

		// Cache the response from the request if the statuscode is 20X
		if c.cache != nil && c.cacheCondition(*response) && key != "" {
			cacheVal, err := response.Marshal()
			if err != nil {
				if c.log != nil {
					c.log.Error().Caller().Err(err).Send()
				}
				return err
			}

			if cacheVal != nil {
				c.lock.Lock()
				err = c.cache.Cache(key, cacheVal)
				if err != nil {
					if c.log != nil {
						c.log.Error().Caller().Err(err).Send()
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

	if response.StatusCode == fasthttp.StatusFound {
		location := response.Headers.Peek("location")

		if c.log != nil {
			c.log.Info().
				Str("method", request.Method).
				Int("status_code", response.StatusCode).
				Str("location", string(location)).
				Uint32("request_id", atomic.LoadUint32(&request.ID)).
				Msg("response")
		}
	} else {
		if c.log != nil {
			l := c.log.Info().
				Str("method", request.Method).
				Int("status_code", response.StatusCode)

			if !response.FromCache {
				if c.ProxyPoolAmount() > 0 {
					l = l.Str("proxy", response.ClientIP())
				} else {
					l = l.Str("server_addr", response.ClientIP())
				}
			}

			l.Bool("from_cache", response.FromCache).
				Uint32("request_id", atomic.LoadUint32(&request.ID)).
				Msg("response")
		}
	}

	c.processResponseHandler(response)

	err = c.processHTMLHandler(response)
	if err != nil {
		return
	}

	// 这里不需要调用 ReleaseRequest，因为 ReleaseResponse 中执行了 ReleaseRequest 方法
	ReleaseResponse(response, !isChained)
	if rawResp != nil {
		// 原始响应应该在自定义响应之后释放，不然一些字段的值会出错
		fasthttp.ReleaseResponse(rawResp)
	}

	return
}

func (c *Crawler) FatalOrPanic(err error) {
	if c.log != nil {
		c.log.Fatal().Caller(1).Err(err).Send()
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
	var resp Response
	err = json.Unmarshal(cachedBody, &resp)
	if err != nil {
		if c.log != nil {
			c.log.Error().Caller().Err(err).Send()
		}
		return nil, err
	}
	resp.FromCache = true
	return &resp, nil
}

func (c *Crawler) do(request *Request) (*Response, *fasthttp.Response, error) {
	req := fasthttp.AcquireRequest()

	req.Header = *request.Headers
	req.SetRequestURI(request.URL)

	if request.Method == fasthttp.MethodPost {
		req.SetBody(request.Body)
	}

	if request.Method == fasthttp.MethodPost && req.Header.Peek("Content-Type") == nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	if len(c.proxyURLPool) > 0 {
		rand.Seed(time.Now().UnixMicro())

		c.client.Dial = func(addr string) (net.Conn, error) {
			// TODO: 代理池中至少保证有一个代理，不然有可能报错
			return c.ProxyDialerWithTimeout(c.proxyURLPool[rand.Intn(len(c.proxyURLPool))], request.timeout)(addr)
		}
	}

	if req.Header.Peek("Accept") == nil {
		req.Header.Set("Accept", "*/*")
	}

	resp := fasthttp.AcquireResponse()

	var err error

	if request.maxRedirectsCount == 0 {
		if c.ProxyPoolAmount() > 0 {
			req.SetConnectionClose()
		}

		if request.timeout > 0 {
			err = c.client.DoTimeout(req, resp, request.timeout)
		} else {
			err = c.client.Do(req, resp)
		}
	} else {
		err = c.client.DoRedirects(req, resp, int(request.maxRedirectsCount))
	}

	response := AcquireResponse()
	response.StatusCode = resp.StatusCode()
	response.Body = append(response.Body, resp.Body()...)
	response.Ctx = request.Ctx
	response.Request = request
	response.Headers = resp.Header
	response.clientIP = resp.RemoteAddr()
	response.localIP = resp.LocalAddr()

	if response.StatusCode == fasthttp.StatusOK && len(response.Body) == 0 {
		// fasthttp.Response 会将空响应的状态码设置为 200，这不合理
		response.StatusCode = 0
	}

	if x, ok := err.(interface{ Timeout() bool }); ok && x.Timeout() {
		response.timeout = true
		err = ErrTimeout
	}

	if err == nil || err == ErrTimeout || err == fasthttp.ErrDialTimeout {
		if c.ProxyPoolAmount() > 0 && c.proxyInvalidCondition != nil {
			e := c.proxyInvalidCondition(*response)
			if e != nil {
				err = e
			}
		}
	}

	if err != nil {
		if p, ok := proxy.IsProxyInvalid(err); ok {
			c.Warning("proxy is invalid", map[string]interface{}{
				"proxy":      p,
				"proxy_pool": c.proxyURLPool,
				"msg":        err.Error(),
			})

			err = c.removeInvalidProxy(p)
			if err != nil {
				c.FatalOrPanic(err)
			}

			c.Info("removed invalid proxy", map[string]interface{}{
				"invalid_proxy":  p,
				"new_proxy_pool": c.proxyURLPool,
			})

			fasthttp.ReleaseRequest(req)
			fasthttp.ReleaseResponse(resp)
			// TODO: Request 和 Response 应该分开 release，否则在重新发出请求前释放 Response，会导致 Request 不能复用，请求的地址变为 /
			// ReleaseResponse(response, false)

			return c.do(request)
		} else {
			if err == ErrTimeout || err == fasthttp.ErrDialTimeout {
				// re-request if the request timed out.
				// re-request 3 times by default when the request times out.

				// if you are using a proxy, the timeout error is probably
				// because the proxy is invalid, and it is recommended
				// to try a new proxy
				c.Error(err)
				if c.retryCount == 0 {
					c.retryCount = 3
				}
				if atomic.LoadUint32(&request.retryCounter) < c.retryCount {
					c.retryPrepare(request, req, resp)
					return c.do(request)
				}
				fasthttp.ReleaseRequest(req)
				fasthttp.ReleaseResponse(resp)
				ReleaseResponse(response, true)

				c.Error(err)
				return nil, nil, ErrTimeout
			} else {
				if err == fasthttp.ErrConnectionClosed {
					// Feature error of fasthttp, there is no solution yet, only try again if c.retryCount > 0 or panic
					c.Error(err)
					if atomic.LoadUint32(&request.retryCounter) < c.retryCount {
						c.retryPrepare(request, req, resp)
						return c.do(request)
					}
				}
				c.Error(err)
				return nil, nil, err
			}
		}
	}

	// Only count successful responses
	atomic.AddUint32(&c.responseCount, 1)
	// release req
	fasthttp.ReleaseRequest(req)

	if c.retryCount > 0 && atomic.LoadUint32(&request.retryCounter) < c.retryCount {
		if c.retryConditions != nil && c.retryConditions(*response) {
			c.retryPrepare(request, req, resp)
			return c.do(request)
		}
	}

	return response, resp, nil
}

func (c *Crawler) retryPrepare(request *Request, req *fasthttp.Request, resp *fasthttp.Response) {
	atomic.AddUint32(&request.retryCounter, 1)
	if c.log != nil {
		c.log.Info().
			Uint32("retry_count", atomic.LoadUint32(&request.retryCounter)).
			Str("method", request.Method).
			Str("url", request.URL).
			Uint32("request_id", atomic.LoadUint32(&request.ID)).
			Msg("retrying")
	}

	fasthttp.ReleaseRequest(req)
	fasthttp.ReleaseResponse(resp)
}

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

func (c *Crawler) get(URL string, headers map[string]string, ctx pctx.Context, isChained bool, cacheFields ...string) error {
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
			if val := params.Get(field); val != "" {
				cachedMap[field] = val
			} else {
				// 如果设置了 cachedFields，但 url 查询参数中却没有某个 field，则报异常退出
				c.FatalOrPanic(fmt.Errorf("there is no such field [%s] in the query parameters: %v", field, params.Encode()))
			}
		}
	}

	return c.request(fasthttp.MethodGet, URL, nil, cachedMap, headers, ctx, isChained)
}

// Get is used to send GET requests
func (c *Crawler) Get(URL string) error {
	return c.GetWithCtx(URL, nil)
}

// GetWithCtx is used to send GET requests with a context
func (c *Crawler) GetWithCtx(URL string, ctx pctx.Context) error {
	return c.get(URL, nil, ctx, false, c.cacheFields...)
}

func (c *Crawler) post(URL string, requestData map[string]string, headers map[string]string, ctx pctx.Context, isChained bool, cacheFields ...string) error {
	var cachedMap map[string]string
	if len(cacheFields) > 0 {

		cachedMap = make(map[string]string)
		for _, field := range cacheFields {
			if val, ok := requestData[field]; ok {
				cachedMap[field] = val
			} else {
				keys := make([]string, 0, len(requestData))
				for _, k := range requestData {
					keys = append(keys, k)
				}
				// 如果 cachedFields 中某个 field 是请求表单中没有的，则报异常退出
				c.FatalOrPanic(fmt.Errorf("there is no such field [%s] in the request body: %v", field, keys))
			}
		}
	}
	return c.request(fasthttp.MethodPost, URL, createBody(requestData), cachedMap, headers, ctx, isChained)
}

// Post is used to send POST requests
func (c *Crawler) Post(URL string, requestData map[string]string, ctx pctx.Context) error {
	return c.post(URL, requestData, nil, ctx, false, c.cacheFields...)
}

func (c *Crawler) createJSONBody(requestData map[string]interface{}) []byte {
	if requestData == nil {
		return nil
	}
	body, err := json.Marshal(requestData)
	if err != nil {
		c.FatalOrPanic(err)
	}
	return body
}

func (c *Crawler) postJSON(URL string, requestData map[string]interface{}, headers map[string]string, ctx pctx.Context, isChained bool, cacheFields ...string) error {
	body := c.createJSONBody(requestData)

	var cachedMap map[string]string
	if len(cacheFields) > 0 {
		cachedMap = make(map[string]string)
		bodyJson := gjson.ParseBytes(body)
		for _, field := range cacheFields {
			if !bodyJson.Get(field).Exists() {
				m := bodyJson.Map()
				var keys = make([]string, 0, len(m))
				for k := range m {
					keys = append(keys, k)
				}
				// 如果 cachedFields 中某个 field 是请求 json 中没有的，则报异常退出
				c.FatalOrPanic(fmt.Errorf("there is no such field [%s] in the request body: %v", field, keys))
			}
			val := bodyJson.Get(field).String()
			cachedMap[field] = val
		}
	}

	if len(headers) == 0 {
		headers = make(map[string]string)
	}
	headers["Content-Type"] = "application/json"

	return c.request(fasthttp.MethodPost, URL, body, cachedMap, headers, ctx, isChained)
}

// PostJSON is used to send a POST request body in json format
func (c *Crawler) PostJSON(URL string, requestData map[string]interface{}, ctx pctx.Context) error {
	return c.postJSON(URL, requestData, nil, ctx, false, c.cacheFields...)
}

func (c *Crawler) postMultipart(URL string, form *MultipartForm, headers map[string]string, ctx pctx.Context, isChained bool, cacheFields ...string) error {
	var cachedMap map[string]string
	if len(cacheFields) > 0 {
		cachedMap = make(map[string]string)
		for _, field := range cacheFields {
			if val, ok := form.bodyMap[field]; ok {
				cachedMap[field] = val
			} else {
				var keys = make([]string, 0, len(form.bodyMap))
				for k := range form.bodyMap {
					keys = append(keys, k)
				}
				// 如果 cachedFields 中某个 field 是请求表单中没有的，则报异常退出
				c.FatalOrPanic(fmt.Errorf("there is no such field [%s] in the request body: %v", field, keys))
			}
		}
	}

	if len(headers) == 0 {
		headers = make(map[string]string)
	}
	headers["Content-Type"] = form.FormDataContentType()

	return c.request(fasthttp.MethodPost, URL, form.Bytes(), cachedMap, headers, ctx, isChained)
}

// PostMultipart
func (c *Crawler) PostMultipart(URL string, form *MultipartForm, ctx pctx.Context) error {
	return c.postMultipart(URL, form, nil, ctx, false, c.cacheFields...)
}

// PostRaw 发送非 form、multipart、json 的原始的 post 请求
func (c *Crawler) PostRaw(URL string, body []byte, ctx pctx.Context) error {
	cachedMap := map[string]string{
		"cache": string(body),
	}
	return c.request(fasthttp.MethodPost, URL, body, cachedMap, nil, ctx, false)
}

/************************* 公共方法 ****************************/

// ClearCache will clear all cache
func (c *Crawler) ClearCache() error {
	if c.cache == nil {
		c.Error(ErrNoCacheSet)
		return ErrNoCacheSet
	}
	if c.log != nil {
		c.log.Warn().Msg("clear all cache")
	}
	return c.cache.Clear()
}

func (c Crawler) ProxyInUse() string {
	if strings.Contains(c.proxyInUse, "//") {
		return strings.Split(c.proxyInUse, "//")[1]
	}
	return c.proxyInUse
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

func (c *Crawler) SetRetry(count uint32, cond RetryConditions) {
	c.retryCount = count
	c.retryConditions = cond
}

/************************* 私有注册方法 ****************************/

func (c *Crawler) processRequestHandler(r *Request) {
	for _, f := range c.requestHandler {
		f(r)
	}
}

func (c *Crawler) processResponseHandler(r *Response) {
	for _, f := range c.responseHandler {
		f(r)
	}
}

func (c *Crawler) processHTMLHandler(r *Response) error {
	if len(c.htmlHandler) == 0 {
		return nil
	}

	if !strings.Contains(strings.ToLower(r.ContentType()), "html") {
		if c.log != nil {
			c.log.
				Debug().
				Caller().
				Str("Content-Type", r.ContentType()).
				Msg(`the "Content-Type" of the response header is not of the "html" type`)
		}
		return nil
	}

	doc, err := html.ParseHTML(r.Body)
	if err != nil {
		if c.log != nil {
			c.log.Error().Caller().Err(err).Send()
		}
		return err
	}

	for _, parser := range c.htmlHandler {
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
		return proxy.ProxyErr{
			Code: proxy.ErrEmptyProxyPoolCode,
			Msg:  "the current proxy pool is empty",
		}
	}

	if c.ProxyPoolAmount() == 1 && c.complementProxyPool != nil {
		newProxyPool := c.complementProxyPool()
		c.proxyURLPool = append(c.proxyURLPool, newProxyPool...)
		c.log.Info().
			Strs("new_proxy_pool", newProxyPool).
			Msg("a new proxy pool has replaced to the old proxy pool")
	}

	targetIndex := -1
	for i, p := range c.proxyURLPool {
		addr := strings.Split(p, "//")[1]
		if addr == proxyAddr {
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
			c.log.Debug().
				Str("proxy", proxyAddr).
				Msg("invalid proxy have been deleted from the proxy pool")
		}

		if len(c.proxyURLPool) == 0 {
			return proxy.ProxyErr{
				Code: proxy.ErrEmptyProxyPoolCode,
				Msg:  "the current proxy pool is empty after removing a invalid proxy",
			}
		}
	} else {
		// 没有在代理池中找到失效代理，这个代理来路不明，一样报错
		return &proxy.ProxyErr{
			Code: proxy.ErrUnkownProxyIPCode,
			Msg:  "proxy address is unkown",
			Args: map[string]string{
				"unkown_proxy_addr": proxyAddr,
			},
		}
	}

	return nil
}

func guessType(l *zerolog.Event, args ...map[string]interface{}) *zerolog.Event {
	if len(args) > 1 {
		panic("too many args")
	}
	if len(args) == 1 {
		for k, arg := range args[0] {
			switch v := arg.(type) {
			case string:
				l = l.Str(k, v)
			case int:
				l = l.Int(k, v)
			case int32:
				l = l.Int32(k, v)
			case int64:
				l = l.Int64(k, v)
			case uint:
				l = l.Uint(k, v)
			case uint32:
				l = l.Uint32(k, v)
			case uint64:
				l = l.Uint64(k, v)
			case float32:
				l = l.Float32(k, v)
			case float64:
				l = l.Float64(k, v)
			case bool:
				l = l.Bool(k, v)
			case []int:
				l = l.Ints(k, v)
			case []int32:
				l = l.Ints32(k, v)
			case []uint:
				l = l.Uints(k, v)
			case []uint32:
				l = l.Uints32(k, v)
			case []uint64:
				l = l.Uints64(k, v)
			case []float32:
				l = l.Floats32(k, v)
			case []float64:
				l = l.Floats64(k, v)
			case []bool:
				l = l.Bools(k, v)
			case []string:
				l = l.Strs(k, v)
			default:
				panic("unkown type")
			}
		}
	}
	return l
}

func (c *Crawler) Debug(msg string, args ...map[string]interface{}) {
	if c.log != nil {
		l := c.log.Debug().Caller(1)
		l = guessType(l, args...)
		l.Msg(msg)
	}
}

func (c *Crawler) Info(msg string, args ...map[string]interface{}) {
	if c.log != nil {
		l := c.log.Info()
		l = guessType(l, args...)
		l.Msg(msg)
	}
}

func (c *Crawler) Warning(msg string, args ...map[string]interface{}) {
	if c.log != nil {
		l := c.log.Warn().Caller(1)
		l = guessType(l, args...)
		l.Msg(msg)
	}
}

func (c *Crawler) Error(err error, args ...map[string]interface{}) {
	if c.log != nil {
		l := c.log.Error().Caller(1).Err(err)
		l = guessType(l, args...)
		l.Send()
	}
}

func (c *Crawler) Fatal(err error, args ...map[string]interface{}) {
	if c.log != nil {
		l := c.log.Fatal().Caller(1).Err(err)
		l = guessType(l, args...)
		l.Send()
	}
}
