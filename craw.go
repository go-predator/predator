/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: craw.go
 * @Created: 2021-07-23 08:52:17
 * @Modified: 2021-10-28 11:07:42
 */

package predator

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-predator/predator/cache"
	pctx "github.com/go-predator/predator/context"
	"github.com/go-predator/predator/html"
	"github.com/go-predator/predator/json"
	"github.com/rs/zerolog"
	"github.com/tidwall/gjson"
	"github.com/valyala/fasthttp"
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

// Crawler is the provider of crawlers
type Crawler struct {
	lock *sync.RWMutex
	// UserAgent is the User-Agent string used by HTTP requests
	UserAgent  string
	retryCount uint32
	// Retry conditions, the crawler will retry only
	// if it returns true
	retryConditions RetryConditions
	client          *fasthttp.Client
	cookies         map[string]string
	goPool          *Pool
	proxyURLPool    []string
	// TODO: 动态获取代理
	// dynamicProxyFunc AcquireProxies
	requestCount  uint32
	responseCount uint32
	// 在多协程中这个上下文管理可以用来退出或取消多个协程
	Context context.Context

	// Cache successful response
	cache cache.Cache
	// List of fields to be cached in the request body, and
	// the combination of these fields can represent the unique
	// request body.
	// The fewer fields the better.
	cacheFields []string

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

func (c *Crawler) request(method, URL string, body []byte, cachedMap map[string]string, headers map[string]string, ctx pctx.Context, isChained bool) error {
	defer func() {
		if c.goPool != nil {
			if err := recover(); err != nil {
				c.log.Fatal().Err(fmt.Errorf("worker panic: %s", err))
			}
		}
	}()

	var err error

	reqHeaders := new(fasthttp.RequestHeader)
	reqHeaders.SetMethod(method)

	u, err := url.Parse(URL)
	if err != nil {
		c.log.Error().Caller().Err(err).Send()
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
		c.log.Debug().
			Bytes("cookies", reqHeaders.Peek("Cookie")).
			Msg("cookies is set")
	}

	if ctx == nil {
		ctx, err = pctx.AcquireCtx()
		if err != nil {
			c.log.Error().Caller().Err(err).Send()
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

	if c.goPool != nil {
		c.wg.Add(1)
		task := &Task{
			crawler:   c,
			req:       request,
			isChained: isChained,
		}
		err = c.goPool.Put(task)
		if err != nil {
			c.log.Error().Caller().Err(err).Send()
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

	c.log.Info().
		Uint32("request_id", atomic.LoadUint32(&request.ID)).
		Str("method", request.Method).
		Str("url", request.URL).
		Str("timeout", request.timeout.String()).
		Msg("requesting")

	if request.Ctx.Length() > 0 {
		c.log.Debug().
			RawJSON("context", request.Ctx.Bytes()).
			Msg("using context")
	}

	if request.abort {
		c.log.Debug().
			Uint32("request_id", atomic.LoadUint32(&request.ID)).
			Msg("the request is aborted")
		return
	}

	var response *Response

	var key string

	if c.cache != nil {
		key, err = request.Hash()
		if err != nil {
			c.log.Error().Caller().Err(err).Send()
			return
		}

		c.log.Debug().
			Uint32("request_id", atomic.LoadUint32(&request.ID)).
			Str("cache_key", key).
			Msg("generate cache key")

		response, err = c.checkCache(key)
		if err != nil {
			return
		}

		if response != nil {
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

		// Save the response from the request to the cache
		if c.cache != nil {
			cacheVal, err := response.Marshal()
			if err != nil {
				c.log.Error().Caller().Err(err).Send()
				return err
			}

			if cacheVal != nil {
				c.lock.Lock()
				err = c.cache.Cache(key, cacheVal)
				if err != nil {
					c.log.Error().Caller().Err(err).Send()
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
		c.log.Info().
			Str("method", request.Method).
			Int("status_code", response.StatusCode).
			Str("location", string(location)).
			Uint32("request_id", atomic.LoadUint32(&request.ID)).
			Msg("response")
	} else {
		c.log.Info().
			Str("method", request.Method).
			Int("status_code", response.StatusCode).
			Bool("from_cache", response.FromCache).
			Uint32("request_id", atomic.LoadUint32(&request.ID)).
			Msg("response")
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

func (c *Crawler) fatalOrPanic(err error) {
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
		c.log.Error().Caller().Err(err).Send()
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

	if c.ProxyPoolAmount() > 0 {
		c.client.Dial = c.DialWithProxy()
	}

	if req.Header.Peek("Accept") == nil {
		req.Header.Set("Accept", "*/*")
	}

	resp := fasthttp.AcquireResponse()

	var err error

	if request.maxRedirectsCount == 0 {
		if request.timeout > 0 {
			err = c.client.DoTimeout(req, resp, request.timeout)
		} else {
			err = c.client.Do(req, resp)
		}
	} else {
		err = c.client.DoRedirects(req, resp, int(request.maxRedirectsCount))
	}

	if err != nil {
		if p, ok := isProxyInvalid(err); ok {
			err = c.removeInvalidProxy(p)
			if err != nil {
				c.fatalOrPanic(err)
			}
			return c.do(request)
		} else {
			c.fatalOrPanic(err)
		}
	}

	// Only count successful responses
	atomic.AddUint32(&c.responseCount, 1)
	// release req
	fasthttp.ReleaseRequest(req)

	response := &Response{
		StatusCode: resp.StatusCode(),
		Body:       resp.Body(),
		Ctx:        request.Ctx,
		Request:    request,
		Headers:    resp.Header,
	}

	if c.retryCount > 0 && request.retryCounter < c.retryCount {
		if c.retryConditions(*response) {
			atomic.AddUint32(&request.retryCounter, 1)
			c.log.Info().
				Uint32("retry_count", atomic.LoadUint32(&request.retryCounter)).
				Str("method", request.Method).
				Str("url", request.URL).
				Uint32("request_id", atomic.LoadUint32(&request.ID)).
				Msg("retrying")
			return c.do(request)
		}
	}

	return response, resp, nil
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

// Get is used to send GET requests
func (c *Crawler) Get(URL string) error {
	return c.GetWithCtx(URL, nil)
}

// GetWithCtx is used to send GET requests with a context
func (c *Crawler) GetWithCtx(URL string, ctx pctx.Context) error {
	// Parse the query parameters and create a `cachedMap` based on `cacheFields`
	u, err := url.Parse(URL)
	if err != nil {
		return err
	}

	params := u.Query()
	var cachedMap = make(map[string]string)
	if c.cacheFields != nil {
		for _, field := range c.cacheFields {
			if val := params.Get(field); val != "" {
				cachedMap[field] = val
			} else {
				c.fatalOrPanic(fmt.Errorf("there is no such field in the query parameters: %s", field))
			}
		}
	}

	return c.request(fasthttp.MethodGet, URL, nil, cachedMap, nil, ctx, false)
}

// Post is used to send POST requests
func (c *Crawler) Post(URL string, requestData map[string]string, ctx pctx.Context) error {
	var cachedMap = make(map[string]string)
	if c.cacheFields != nil {
		for _, field := range c.cacheFields {
			if val, ok := requestData[field]; ok {
				cachedMap[field] = val
			} else {
				c.fatalOrPanic(fmt.Errorf("there is no such field in the request body: %s", field))
			}
		}
	}
	return c.request(fasthttp.MethodPost, URL, createBody(requestData), cachedMap, nil, ctx, false)
}

func (c *Crawler) createJSONBody(requestData map[string]interface{}) []byte {
	if requestData == nil {
		return nil
	}
	body, err := json.Marshal(requestData)
	if err != nil {
		c.fatalOrPanic(err)
	}
	return body
}

// PostJSON is used to send a POST request body in json format
func (c *Crawler) PostJSON(URL string, requestData map[string]interface{}, ctx pctx.Context) error {
	body := c.createJSONBody(requestData)

	var cachedMap = make(map[string]string)
	if c.cacheFields != nil {
		bodyJson := gjson.ParseBytes(body)
		for _, field := range c.cacheFields {
			if !bodyJson.Get(field).Exists() {
				c.fatalOrPanic(fmt.Errorf("there is no such field in the request body: %s", field))
			}
			val := bodyJson.Get(field).String()
			cachedMap[field] = val
		}
	}

	headers := make(map[string]string)
	headers["Content-Type"] = "application/json"

	return c.request(fasthttp.MethodPost, URL, body, cachedMap, headers, ctx, false)
}

// PostMultipart
func (c *Crawler) PostMultipart(URL string, form *MultipartForm, ctx pctx.Context) error {
	var cachedMap = make(map[string]string)
	if c.cacheFields != nil {
		for _, field := range c.cacheFields {
			if val, ok := form.bodyMap[field]; ok {
				cachedMap[field] = val
			} else {
				c.fatalOrPanic(fmt.Errorf("there is no such field in the request body: %s", field))
			}
		}
	}

	headers := make(map[string]string)
	headers["Content-Type"] = form.FormDataContentType()

	return c.request(fasthttp.MethodPost, URL, form.Bytes(), cachedMap, headers, ctx, false)
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
		return ErrNoCacheSet
	}
	c.log.Warn().Msg("clear all cache")
	return c.cache.Clear()
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
	if len(c.htmlHandler) == 0 || !strings.Contains(strings.ToLower(r.ContentType()), "html") {
		return nil
	}

	doc, err := html.ParseHTML(r.Body)
	if err != nil {
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
func (c *Crawler) removeInvalidProxy(proxy string) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.ProxyPoolAmount() == 0 {
		return ErrEmptyProxyPool
	}

	targetIndex := -1
	for i, p := range c.proxyURLPool {
		addr := strings.Split(p, "//")[1]
		if addr == proxy {
			targetIndex = i
			break
		}
	}

	if targetIndex >= 0 {
		c.proxyURLPool = append(
			c.proxyURLPool[:targetIndex],
			c.proxyURLPool[targetIndex+1:]...,
		)

		c.log.Debug().
			Str("proxy", proxy).
			Msg("invalid proxy have been deleted from the proxy pool")

		if len(c.proxyURLPool) == 0 {
			return ErrEmptyProxyPool
		}
	} else {
		// 没有在代理池中找到失效代理，这个代理来路不明，一样报错
		return ErrUnkownProxyIP
	}

	return nil
}

func (c *Crawler) Error(err error) {
	c.log.Error().Caller().Err(err).Send()
}
