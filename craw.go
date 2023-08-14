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

	// The UserAgent is the User-Agent string
	// used by HTTP requests.
	UserAgent  string
	retryCount uint32

	// Retry condition, the crawler will retry only
	// if it returns true.
	retryCondition RetryCondition
	client         *http.Client

	// When setting cookies, the priority should be
	// given to the `rawCookies` field over the
	// `cookies` field.
	rawCookies string
	cookies    map[string]string

	goPool *Pool

	proxyURLPool          []string
	proxyInvalidCondition ProxyInvalidCondition

	complementProxyPool ComplementProxyPool
	requestCount        uint32
	responseCount       uint32

	// TODO: This context management can be used
	// to exit or cancel multiple goroutines in
	// a multi-goroutine context.
	Context context.Context

	// Cache successful responses.
	cache Cache

	// List of fields to be cached in the request body,
	// and the combination of these fields can represent
	// the unique request body.
	//
	// The fewer fields the better.
	cacheFields    []CacheField
	cacheCondition CacheCondition

	requestHandler []HandleRequest

	// An array of functions that will handle the response.
	responseHandler []HandleResponse
	// An array of functions that will handle parsed HTML.
	htmlHandler []*HTMLParser
	jsonHandler []*JSONParser

	// Set a timeout for all requests.
	timeout time.Duration

	// Indicate whether the remote address should be
	// recorded in the request.
	recordRemoteAddr bool

	wg *sync.WaitGroup

	log *log.Logger
}

// NewCrawler returns a new instance of Crawler with default configuration.
//
// Optional CrawlerOptions can be provided as parameters to customize the crawler.
//
// By default, the crawler uses the "Predator" user agent and the default HTTP transport.
//
// If DEBUG environment variable is set and c.log is nil, a logger with a level of DEBUG will be created.
//
// The returned Crawler instance has a background context, a RWMutex lock, and its capacity state and timeout are
// logged if a logger is provided.
//
// If a goPool is provided, it will be used for concurrent requests and its logger will be set to the logger of the
// returned Crawler instance if it has one.
func NewCrawler(opts ...CrawlerOption) *Crawler {
	c := &Crawler{
		UserAgent: "Predator",
		client: &http.Client{
			Transport: http.DefaultTransport,
		},
		Context: context.Background(),
		lock:    &sync.RWMutex{},
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

	capacityState := c.goPool != nil

	if c.log != nil {
		logArgs := []log.Arg{
			log.NewArg("state", capacityState),
			log.NewArg("timeout", c.timeout.String()),
		}
		if capacityState {
			logArgs = append(logArgs, log.NewArg("capacity", c.goPool.capacity))
		}
		c.Info("concurrent", logArgs...)
	}

	if c.log != nil && c.goPool != nil {
		c.goPool.log = c.log
	}

	return c
}

// Clone creates an exact copy of the Crawler without callbacks.
//
// It returns a pointer to the new Crawler instance.
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

	proxyPool := make([]string, len(c.proxyURLPool))
	copy(proxyPool, c.proxyURLPool)

	return &Crawler{
		lock:                c.lock,
		UserAgent:           c.UserAgent,
		retryCount:          c.retryCount,
		retryCondition:      c.retryCondition,
		client:              c.client,
		cookies:             c.cookies,
		rawCookies:          c.rawCookies,
		goPool:              pool,
		proxyURLPool:        proxyPool,
		Context:             c.Context,
		cache:               c.cache,
		cacheCondition:      c.cacheCondition,
		complementProxyPool: c.complementProxyPool,
		cacheFields:         c.cacheFields,
		requestHandler:      make([]HandleRequest, 0, 5),
		responseHandler:     make([]HandleResponse, 0, 5),
		htmlHandler:         make([]*HTMLParser, 0, 5),
		jsonHandler:         make([]*JSONParser, 0, 1),
		recordRemoteAddr:    c.recordRemoteAddr,
		wg:                  &sync.WaitGroup{},
		log:                 c.log,
		timeout:             c.timeout,
	}
}

/************************* http 请求方法 ****************************/

// request sends an HTTP request to the given URL with the specified
// method, body, header, and context.
// It also caches the response if specified in the cachedMap. If
// isChained is true, it indicates that the request is chained and
// will be followed by another request.
//
// The function returns an error if any errors occur during the request.
func (c *Crawler) request(
	method, URL string,
	body []byte,
	cachedMap map[string]string,
	reqHeader http.Header,
	ctx pctx.Context,
	isChained bool,
) error {
	// Recover from any panics that occur in the worker pool.
	defer func() {
		if c.goPool != nil {
			if err := recover(); err != nil {
				c.FatalOrPanic(fmt.Errorf("worker panic: %s", err))
			}
		}
	}()

	var err error

	// Parse the URL.
	u := AcquireURL()
	err = u.UnmarshalBinary([]byte(URL))
	if err != nil {
		return err
	}

	// Create a new request with the specified timeout.
	var request *Request
	if c.timeout <= 0 {
		request = NewRequest()
	} else {
		request = NewRequestWithTimeout(c.timeout)
	}

	// Set the request header.
	if reqHeader == nil {
		reqHeader = make(http.Header)
	}

	if reqHeader.Get("User-Agent") == "" {
		reqHeader.Set("User-Agent", c.UserAgent)
	}

	request.req.Header = reqHeader
	request.req.URL = u
	request.req.Method = method

	// Acquire a context if none was provided.
	if ctx == nil {
		ctx, err = pctx.AcquireCtx()
		if err != nil {
			if c.log != nil {
				c.Error(err)
			}
			return err
		}
	}

	// Set the request properties.
	request.Ctx = ctx
	request.body = body
	request.cachedMap = cachedMap
	request.ID = atomic.AddUint32(&c.requestCount, 1)
	request.crawler = c

	// Parse the raw cookies if any were provided.
	if c.rawCookies != "" {
		request.ParseRawCookie(c.rawCookies)
		request.req.Header.Set("Cookie", c.rawCookies)
		if c.log != nil {
			c.Debug("cookies is set", log.NewArg("cookies", c.rawCookies))
		}
	}

	// Log the request if logging is enabled.
	if c.log != nil {
		c.Info(
			"requesting",
			log.NewArg("request_id", atomic.LoadUint32(&request.ID)),
			log.NewArg("method", request.Method()),
			log.NewArg("url", request.URL()),
			log.NewArg("timeout", request.timeout.String()),
		)
	}

	// Send the request to the worker pool if one exists.
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

	// Otherwise, prepare the request and return any errors.
	err = c.prepare(request, isChained)
	if err != nil {
		return err
	}

	return nil
}

// prepare prepares the request for crawling and sends it to the server.
//
// If the response is found in the cache, it returns it directly. Otherwise,
// it sends a new request to the server and caches the response if the cache
// condition is met.
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
			c.Debug(
				"the request is aborted",
				log.NewArg("request_id", atomic.LoadUint32(&request.ID)),
			)
		}
		return
	}

	if request.Method() == "" {
		c.Fatal("请求不正确", log.NewArg("id", atomic.LoadUint32(&request.ID)))
	}

	if request.Ctx.Length() > 0 {
		if c.log != nil {
			c.Debug("using context", log.NewArg("context", request.Ctx.String()))
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
				log.NewArg("request_id", atomic.LoadUint32(&request.ID)),
				log.NewArg("cache_key", key),
			)
		}

		response, err = c.checkCache(key)
		if err != nil {
			return
		}

		if response != nil && c.log != nil {
			c.Debug("response is in the cache",
				log.NewArg("request_id", atomic.LoadUint32(&request.ID)),
				log.NewArg("cache_key", key),
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

	cost := response.recievedTime.Sub(request.sendingTime).String()
	if response.StatusCode == StatusFound {
		if c.log != nil {
			location := response.header.Get("Location")
			c.Info("response",
				log.NewArg("method", request.Method()),
				log.NewArg("status_code", response.StatusCode),
				log.NewArg("content_length", response.ContentLength()),
				log.NewArg("location", location),
				log.NewArg("request_id", atomic.LoadUint32(&request.ID)),
				log.NewArg("cost", cost),
			)
		}
	} else {
		if c.log != nil {
			l := c.log.L.Info().
				Str("method", request.Method()).
				Int("status_code", int(response.StatusCode)).
				Uint64("content_length", response.ContentLength()).
				Str("cost", cost)

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

// FatalOrPanic is a convenience function that calls Fatal if a logger is available, or panics if not.
func (c *Crawler) FatalOrPanic(err error) {
	if c.log != nil {
		c.Fatal(err)
	} else {
		panic(err)
	}
}

// checkCache checks if the given key is in the cache and returns the cached response if found.
//
// If the key is not found, it returns nil.
func (c *Crawler) checkCache(key string) (*Response, error) {
	var err error

	// check if key exists in cache
	cachedBody, ok := c.cache.IsCached(key)
	if !ok {
		return nil, nil
	}

	// unmarshal cached body to create a Response object
	resp := new(Response)
	err = resp.Unmarshal(cachedBody)
	if err != nil {
		if c.log != nil {
			c.Error(err)
		}
		return nil, err
	}

	// set FromCache flag to true to indicate that this response is from cache
	resp.FromCache = true
	return resp, nil
}

// randomProxy selects a random proxy server from a list of proxy URLs
// and returns the corresponding proxy function to be used in the request.
//
// It also updates the request's "proxyUsed" field with the selected proxy's address.
func (c *Crawler) randomProxy(req *Request) proxy.ProxyFunc {
	// select a random proxy URL from the pool
	selectedURL := c.proxyURLPool[rand.Intn(len(c.proxyURLPool))]
	// get the proxy function and its address
	proxyFunc, proxyAddr := proxy.Proxy(selectedURL)
	// update the request with the selected proxy's address
	req.proxyUsed = proxyAddr
	// return the proxy function
	return proxyFunc
}

// processProxyError checks if the error is caused by a proxy issue. If so, it removes the invalid proxy from the proxy pool and returns nil.
// Otherwise, it returns the original error.
func (c *Crawler) processProxyError(req *Request, err error) error {
	if _, ok := proxy.IsProxyError(err); !ok {
		return err
	}

	pe := err.(proxy.ProxyErr)

	c.Warning("proxy is invalid",
		log.NewArg("proxy", req.proxyUsed),
		log.NewArg("proxy_pool", c.proxyURLPool),
		log.NewArg("error", pe.Err),
	)

	err = c.removeInvalidProxy(req.proxyUsed)
	if err != nil {
		c.FatalOrPanic(err)
	}

	c.Info("removed invalid proxy",
		log.NewArg("invalid_proxy", req.proxyUsed),
		log.NewArg("new_proxy_pool", c.proxyURLPool),
	)

	return nil
}

// removeInvalidProxy removes an invalid proxy from the proxy pool.
//
// If the proxy pool is empty, it returns an ErrEmptyProxyPool error.
//
// If the proxy pool has only one proxy and there is a complement proxy pool generator, it replaces the current proxy pool with a new one.
//
// If the target proxy is found, it removes it from the proxy pool and returns nil.
//
// If the target proxy is not found, it returns an error.
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
			log.NewArg("new_proxy_pool", newProxyPool),
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
				log.NewArg("proxy", proxyAddr),
			)
		}

		if len(c.proxyURLPool) == 0 {
			return ErrEmptyProxyPool
		}
	} else {
		// In case of concurrent access, it may not find the invalid proxy, in which case it should not return an error.
		if c.goPool != nil {
			return nil
		}

		// If the invalid proxy is not found in the proxy pool, it is an unknown proxy and should raise an error.
		return fmt.Errorf("%w: %s", proxy.ErrUnkownProxyIP, proxyAddr)
	}

	return nil
}

// preprocessResponseError processes the error occurred during the HTTP response.
//
// If the error is not a URL error, it returns the error directly. Otherwise, it
// checks whether the error is a network operation error. If so, it further checks
// the operation type of the error. If the operation is a SOCKS proxy connection,
// it returns an error indicating that the SOCKS5 protocol is not expected. If the
// operation is a proxy connection, it checks whether the error is caused by an
// unexpected protocol. If the error is caused by a TLS handshake error, it returns
// an error indicating that the HTTPS protocol is not expected. If the error is a
// syscall error, it returns a new ProxyError with the original error.
func (c *Crawler) preprocessResponseError(req *Request, err error) error {
	if err == nil {
		return nil
	}

	c.Debug("raw error", log.NewArg("error", err))

	if ne, ok := err.(net.Error); ok && ne.Timeout() && req.proxyUsed != "" {
		return proxy.NewProxyError(req.proxyUsed, ErrTimeout)
	}

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

	if req.proxyUsed != "" {
		return proxy.NewProxyError(req.proxyUsed, err)
	}

	return err
}

// do sends an HTTP request and returns an HTTP response and an error.
func (c *Crawler) do(request *Request) (*Response, error) {
	// Set timeout and check redirect settings
	c.client.Timeout = request.timeout
	if request.checkRedirect != nil {
		c.client.CheckRedirect = request.checkRedirect
	} else {
		c.client.CheckRedirect = defaultCheckRedirect
	}

	// Set proxy settings if a proxy is available
	if len(c.proxyURLPool) > 0 {
		c.client.Transport = &http.Transport{
			Proxy: c.randomProxy(request),
		}

		c.Debug("request information",
			log.NewArg("header", request.Header()),
			log.NewArg("proxy", request.proxyUsed))
	} else {
		c.Debug("request information", log.NewArg("header", request.Header()))
	}

	// Add remote address to the request context if enabled
	if c.recordRemoteAddr {
		trace := &httptrace.ClientTrace{
			GotConn: func(connInfo httptrace.GotConnInfo) {
				request.req.RemoteAddr = connInfo.Conn.RemoteAddr().String()
			},
		}

		request.req = request.req.WithContext(
			httptrace.WithClientTrace(request.req.Context(), trace),
		)
	}

	// Add request body and send the request
	request.WithBody()

	request.sendingTime = time.Now()

	var err error
	resp, err := c.client.Do(request.req)
	if err != nil {
		// Handle request error and retry if necessary
		e := c.preprocessResponseError(request, err)

		// Retry if the error is a timeout
		if errors.Is(e, ErrTimeout) {
			if request.proxyUsed != "" {
				c.Warning(
					"the connection timed out, but it was not possible to determine if the error was caused by a timeout with the proxy server or a timeout between the proxy server and the target server",
				)
			}

			if c.retryCount == 0 {
				c.retryCount = 3
			}

			c.Error(err,
				log.NewArg("timeout", request.timeout.String()),
				log.NewArg("request_id", atomic.LoadUint32(&request.ID)),
				log.NewArg("proxy", request.proxyUsed),
			)

			if atomic.LoadUint32(&request.retryCounter) < c.retryCount {
				c.retryPrepare(request)
				return c.do(request)
			}

			return nil, e
		}

		// Handle proxy error and retry if necessary
		e = c.processProxyError(request, e)

		if e == nil {
			return c.do(request)
		}

		c.Error(e)
		return nil, e
	}
	defer resp.Body.Close()

	recievedTime := time.Now()

	// Read response body and create a Response object
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) && request.proxyUsed != "" {
			err = proxy.NewProxyError(request.proxyUsed, err)
		}

		err = c.processProxyError(request, err)
		if err == nil {
			return c.do(request)
		}

		c.Error(err)
		return nil, err
	}

	response := new(Response)
	response.StatusCode = StatusCode(resp.StatusCode)
	response.Body = append(response.Body, body...)
	response.Ctx = request.Ctx
	response.Request = request
	response.header = resp.Header.Clone()

	if request.proxyUsed != "" {
		response.clientIP = request.proxyUsed
	} else {
		response.clientIP = request.req.RemoteAddr
	}

	response.isJSON = strings.Contains(strings.ToLower(response.ContentType()), "application/json")
	response.recievedTime = recievedTime

	// Parse JSON response if necessary
	if response.isJSON {
		result := json.ParseBytesToJSON(body)
		response.json = &result
	}

	// Debug response header
	c.Debug("response header", log.NewArg("header", resp.Header))

	// Only count successful responses
	atomic.AddUint32(&c.responseCount, 1)

	// Retry if necessary
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
		log.NewArg("retry_count", atomic.LoadUint32(&request.retryCounter)),
		log.NewArg("method", request.Method()),
		log.NewArg("url", request.URL()),
		log.NewArg("request_id", atomic.LoadUint32(&request.ID)),
	)
}

// createPostBody is a type alias for a function that takes a map of string keys and values
// and returns a byte slice that represents the encoded form data for a POST request.
type createPostBody func(requestData map[string]string) []byte

// createBody is a function that implements the createPostBody type.
//
// It checks if the requestData map is nil and returns nil if so.
// Otherwise, it creates a url.Values object and adds each key-value pair from the map to it.
// Then it encodes the url.Values object into a byte slice and returns it.
func createBody(requestData map[string]string) []byte {
	// check if requestData is nil
	if requestData == nil {
		return nil
	}

	// create a url.Values object
	form := url.Values{}
	// iterate over the map and add each key-value pair to the form
	for k, v := range requestData {
		form.Add(k, v)
	}

	// encode the form into a byte slice and return it
	return []byte(form.Encode())
}

// get is a method of the Crawler type that performs a GET request to the given URL with the given header and context.
//
// It also accepts an optional boolean argument to indicate if the request is chained from another one,
// and a variadic argument of CacheField type to specify which query parameters to use for caching the response.
//
// It returns an error if any occurs during the request or parsing the URL.
func (c *Crawler) get(
	URL string,
	header http.Header,
	ctx pctx.Context,
	isChained bool,
	cacheFields ...CacheField,
) error {
	// Parse the query parameters from the URL and create a map to store them
	u, err := url.Parse(URL)
	if err != nil {
		c.Error(err)
		return err
	}

	params := u.Query()
	var cachedMap map[string]string
	// If cacheFields are provided, iterate over them and add them to the cachedMap with their values
	if len(cacheFields) > 0 {
		cachedMap = make(map[string]string)
		for _, field := range cacheFields {
			// Check if the field type is queryParam, otherwise panic
			if field.code != queryParam {
				c.FatalOrPanic(ErrNotAllowedCacheFieldType)
			}

			// Add the field key and value to the params and cachedMap using addQueryParamCacheField function,
			// which may return an error if something goes wrong
			key, value, err := addQueryParamCacheField(params, field)
			if err != nil {
				c.FatalOrPanic(err)
			}

			// If a prepare function is provided for the field, apply it to the value before storing it in cachedMap
			if field.prepare != nil {
				value = field.prepare(value)
			}

			cachedMap[key] = value // Store the key-value pair in cachedMap for later use in caching logic
		}

		// Log a debug message with cachedMap as an argument for debugging purposes
		c.Debug("use some specified cache fields", log.NewArg("cached_map", cachedMap))
	}

	// Perform the GET request with updated URL (including added query parameters) and return any error
	return c.request(MethodGet, u.String(), nil, cachedMap, header, ctx, isChained)
}

// Get is a method of the Crawler type that performs a GET request to the given URL with the default header and context.
//
// It calls the get method with nil header, nil context, false isChained and c.cacheFields as arguments.
//
// It returns an error if any occurs during the request or parsing the URL.
func (c *Crawler) Get(URL string) error {
	return c.GetWithCtx(URL, nil)
}

// GetWithCtx is a method of the Crawler type that performs a GET request to the given URL with the given context and default header.
//
// It calls the get method with nil header, ctx context, false isChained and c.cacheFields as arguments.
//
// It returns an error if any occurs during the request or parsing the URL.
func (c *Crawler) GetWithCtx(URL string, ctx pctx.Context) error {
	return c.get(URL, nil, ctx, false, c.cacheFields...)
}

// post is a method of the Crawler type that performs a POST request to the given URL with the given request data, header and context.
//
// It also accepts an optional boolean argument to indicate if the request is chained from another one,
// an optional function argument to create the request body from the request data,
// and a variadic argument of CacheField type to specify which query or body parameters to use for caching the response.
// It returns an error if any occurs during the request or parsing the URL.
func (c *Crawler) post(
	URL string,
	requestData map[string]string,
	header http.Header,
	ctx pctx.Context,
	isChained bool,
	createBodyFunc createPostBody,
	cacheFields ...CacheField,
) error {
	// Create a map to store the cache fields and their values
	var cachedMap map[string]string
	// If cacheFields are provided, iterate over them and add them to the cachedMap with their values
	if len(cacheFields) > 0 {
		cachedMap = make(map[string]string)

		var queryParams url.Values // A variable to store the query parameters from the URL
		for _, field := range cacheFields {
			var (
				err        error
				key, value string // The key and value of the cache field
			)

			switch field.code {
			case queryParam: // If the field type is queryParam
				if queryParams == nil { // If queryParams is nil, parse it from the URL
					u, err := url.Parse(URL)
					if err != nil {
						c.FatalOrPanic(err)
					}

					queryParams = u.Query()
				}

				// Add the field key and value to queryParams and cachedMap using addQueryParamCacheField function,
				// which may return an error if something goes wrong
				key, value, err = addQueryParamCacheField(queryParams, field)
				if field.prepare != nil { // If a prepare function is provided for the field, apply it to the value before storing it in cachedMap
					value = field.prepare(value)
				}
			case requestBodyParam: // If the field type is requestBodyParam
				if val, ok := requestData[field.Field]; ok { // If there is such a key in requestData map
					key, value = field.String(), val // Use it as key and value for caching
					if field.prepare != nil {        // If a prepare function is provided for the field, apply it to the value before storing it in cachedMap
						value = field.prepare(value)
					}
				} else { // Otherwise report an error that there is no such key in requestData map
					keys := make([]string, 0, len(requestData))
					for k := range requestData {
						keys = append(keys, k)
					}

					err = fmt.Errorf("there is no such field [%s] in the request body: %v", field.Field, keys)
				}
			default: // If none of above cases match report an error that invalid cache type code was used
				err = ErrInvalidCacheTypeCode
			}

			if err != nil { // If any error occurred during adding cache fields panic
				c.FatalOrPanic(err)
			}

			cachedMap[key] = value // Store key-value pair in cachedMap for later use in caching logic
		}

		// Log a debug message with cachedMap as an argument for debugging purposes
		c.Debug("use some specified cache fields", log.NewArg("cached_map", cachedMap))
	}

	// Create header if not provided or set default Content-Type if not set
	if len(header) == 0 {
		header = make(http.Header)
	}

	if _, ok := header["Content-Type"]; !ok {
		header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	// Use default createBodyFunc if not provided
	if createBodyFunc == nil {
		createBodyFunc = createBody
	}

	// Perform POST request with given URL (including added query parameters), created body (from given request data), cachedMap (for caching logic), header and context. Return any error.
	return c.request(
		MethodPost,
		URL,
		createBodyFunc(requestData),
		cachedMap,
		header,
		ctx,
		isChained,
	)
}

// Post is a method of the Crawler type that performs a POST request to the given URL with the given request data and the default header and context.
//
// It calls the post method with nil header, nil context, false isChained, nil createBodyFunc and c.cacheFields as arguments.
// It returns an error if any occurs during the request or parsing the URL.
func (c *Crawler) Post(URL string, requestData map[string]string) error {
	return c.post(URL, requestData, nil, nil, false, nil, c.cacheFields...)
}

// PostWithCtx is a method of the Crawler type that performs a POST request to the given URL with the given request data and context and the default header.
//
// It calls the post method with nil header, ctx context, false isChained, nil createBodyFunc and c.cacheFields as arguments.
// It returns an error if any occurs during the request or parsing the URL.
func (c *Crawler) PostWithCtx(URL string, requestData map[string]string, ctx pctx.Context) error {
	return c.post(URL, requestData, nil, ctx, false, nil, c.cacheFields...)
}

// PostWithCreateBodyFunc is a method of the Crawler type that performs a POST request to the given URL with the given request data and a custom function to create the request body.
//
// It calls the post method with nil header, nil context, false isChained, createBodyFunc and c.cacheFields as arguments.
// It returns an error if any occurs during the request or parsing the URL.
// This method was added because some websites use JavaScript to construct illegal request bodies that cannot be handled by the default createBody function. This is very common in China.
func (c *Crawler) PostWithCreateBodyFunc(
	URL string,
	requestData map[string]string,
	createBodyFunc createPostBody,
) error {
	return c.post(URL, requestData, nil, nil, false, createBodyFunc, c.cacheFields...)
}

// PostWithCtxAndCreateBodyFunc is a method of the Crawler type that performs a POST request to the given URL with the given request data, context and a custom function to create the request body.
//
// It calls the post method with nil header, ctx context, false isChained, createBodyFunc and c.cacheFields as arguments.
// It returns an error if any occurs during the request or parsing the URL.
func (c *Crawler) PostWithCtxAndCreateBodyFunc(
	URL string,
	requestData map[string]string,
	ctx pctx.Context,
	createBodyFunc createPostBody,
) error {
	return c.post(URL, requestData, nil, ctx, false, createBodyFunc, c.cacheFields...)
}

// createJSONBody is a method of the Crawler type that creates a JSON-encoded byte slice from the given request data map.
//
// It returns nil if the request data map is nil or an error occurs during encoding.
func (c *Crawler) createJSONBody(requestData map[string]any) []byte {
	if requestData == nil {
		return nil
	}

	body, err := json.Marshal(requestData) // Encode the request data map into JSON format
	if err != nil {
		c.FatalOrPanic(err) // Panic if encoding fails
	}

	return body // Return the encoded byte slice
}

// postJSON is a method of the Crawler type that performs a POST request to the given URL with the given request data map, header and context.
//
// It also accepts an optional boolean argument to indicate if the request is chained from another one,
// and a variadic argument of CacheField type to specify which query or body parameters to use for caching the response.
// It returns an error if any occurs during the request or parsing the URL.
func (c *Crawler) postJSON(
	URL string,
	requestData map[string]any,
	header http.Header,
	ctx pctx.Context,
	isChained bool,
	cacheFields ...CacheField,
) error {
	body := c.createJSONBody(
		requestData,
	) // Create a JSON-encoded byte slice from the request data map

	// Create a map to store the cache fields and their values
	var cachedMap map[string]string
	// If cacheFields are provided, iterate over them and add them to the cachedMap with their values
	if len(cacheFields) > 0 {
		cachedMap = make(map[string]string)
		bodyJson := json.ParseBytesToJSON(
			body,
		) // Parse the JSON-encoded byte slice into a JSON object

		var queryParams url.Values // A variable to store the query parameters from the URL

		for _, field := range cacheFields {
			var (
				err        error
				key, value string // The key and value of the cache field
			)

			switch field.code {
			case queryParam: // If the field type is queryParam
				if queryParams == nil { // If queryParams is nil, parse it from the URL
					u, err := url.Parse(URL)
					if err != nil {
						c.FatalOrPanic(err)
					}

					queryParams = u.Query()
				}

				// Add the field key and value to queryParams and cachedMap using addQueryParamCacheField function,
				// which may return an error if something goes wrong
				key, value, err = addQueryParamCacheField(queryParams, field)
				if field.prepare != nil { // If a prepare function is provided for the field, apply it to the value before storing it in cachedMap
					value = field.prepare(value)
				}
			case requestBodyParam: // If the field type is requestBodyParam
				if !bodyJson.Get(field.Field).
					Exists() { // If there is no such key in bodyJson object
					m := bodyJson.Map()
					keys := make([]string, 0, len(m))
					for k := range m {
						keys = append(keys, k)
					}
					err = fmt.Errorf(
						"there is no such field [%s] in the request body: %v",
						field,
						keys,
					) // Report an error that there is no such key in bodyJson object
				} else {
					key, value = field.String(), bodyJson.Get(field.Field).String() // Use it as key and value for caching
					if field.prepare != nil {                                       // If a prepare function is provided for the field, apply it to th e value before storing it in cachedMap
						value = field.prepare(value)
					}
				}
			default: // If none of above cases match report an error that invalid cache type code was used
				err = ErrInvalidCacheTypeCode
			}

			if err != nil { // If any error occurred during adding cache fields panic
				c.FatalOrPanic(err)
			}

			cachedMap[key] = value // Store key-value pair in cachedMap for later use in caching logic
		}

		// Log a debug message with cachedMap as an argument for debugging purposes
		c.Debug("use some specified cache fields", log.NewArg("cached_map", cachedMap))
	}

	// Create header if not provided or set Content-Type to application/json if not set
	if len(header) == 0 {
		header = make(http.Header)
	}

	header.Set("Content-Type", "application/json")

	// Perform POST request with given URL (including added query parameters), created body (from given request data), cachedMap (for caching logic), header and context. Return any error.
	return c.request(MethodPost, URL, body, cachedMap, header, ctx, isChained)
}

// PostJSON sends a POST request with a JSON body to the specified URL using the specified context.
//
// It serializes the request data into a JSON-encoded byte slice and sets the Content-Type header to application/json.
// It also supports caching of specified fields for future requests with the same values.
func (c *Crawler) PostJSON(URL string, requestData map[string]interface{}, ctx pctx.Context) error {
	// delegate to postJSON function with default values for header, isChained and cacheFields
	return c.postJSON(URL, requestData, nil, ctx, false, c.cacheFields...)
}

// postMultipart sends a POST request with `multipart/form-data` content-type.
//
// URL is the target URL, mfw is a MultipartFormWriter containing the request body,
// header is the HTTP header, ctx is the context, isChained is a flag indicating whether this request is chained,
// and cacheFields specifies the fields to cache.
func (c *Crawler) postMultipart(
	URL string,
	mfw *MultipartFormWriter,
	header http.Header,
	ctx pctx.Context,
	isChained bool,
	cacheFields ...CacheField,
) error {
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
					keys := make([]string, 0, len(mfw.cachedMap))
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

		c.Debug("use some specified cache fields", log.NewArg("cached_map", cachedMap))
	}

	if len(header) == 0 {
		header = make(http.Header)
	}

	contentType, buf := NewMultipartForm(mfw)

	header.Set("Content-Type", contentType)

	return c.request(MethodPost, URL, buf.Bytes(), cachedMap, header, ctx, isChained)
}

// PostMultipart sends a POST request with content-type `multipart/form-data`.
// It uses the provided MultipartFormWriter to build the request body.
// If any cacheFields are provided, it will cache the specified fields in the request.
func (c *Crawler) PostMultipart(URL string, mfw *MultipartFormWriter, ctx pctx.Context) error {
	return c.postMultipart(URL, mfw, nil, ctx, false, c.cacheFields...)
}

// PostRaw sends a POST request with a raw body and a content-type that is not json,
// `application/x-www-form-urlencoded`, or `multipart/form-data`.
func (c *Crawler) PostRaw(URL string, body []byte, ctx pctx.Context) error {
	cachedMap := map[string]string{
		"cache": string(body),
	}
	return c.request(MethodPost, URL, body, cachedMap, nil, ctx, false)
}

/************************* Public methods ****************************/

// ClearCache clears all the cache stored by the crawler
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

// ConcurrencyState returns the current state of the crawler's goroutine pool, true if it's enabled and false otherwise
func (c *Crawler) ConcurrencyState() bool {
	return c.goPool != nil
}

// BeforeRequest is a method of the Crawler type that registers a function to be executed before each request.
//
// The function takes a Request object as an argument and can modify it as needed.
// The Crawler can have multiple request handlers, which are stored in a slice and executed in order.
// This method is thread-safe and can be called concurrently.
func (c *Crawler) BeforeRequest(f HandleRequest) {
	c.lock.Lock() // Lock the Crawler to prevent data race
	if c.requestHandler == nil {
		// Initialize the request handler slice with a capacity of 5,
		// assuming that a Crawler does not need too many request handlers,
		// and expand it automatically when needed
		c.requestHandler = make([]HandleRequest, 0, 5)
	}
	c.requestHandler = append(c.requestHandler, f) // Append the given function to the slice
	c.lock.Unlock()                                // Unlock the Crawler
}

// ParseHTML is a method of the Crawler type that registers a function to be executed for each HTML element that matches a given selector string.
//
// The function takes an Element object as an argument and can extract data or perform actions on it as needed.
// The Crawler can have multiple HTML parsers, which are stored in a slice and executed in order.
// This method is thread-safe and can be called concurrently.
func (c *Crawler) ParseHTML(selector string, f HandleHTML) {
	c.lock.Lock() // Lock the Crawler to prevent data race
	if c.htmlHandler == nil {
		// Initialize the HTML handler slice with a capacity of 5,
		// assuming that a Crawler does not need too many HTML handlers,
		// and expand it automatically when needed
		c.htmlHandler = make([]*HTMLParser, 0, 5)
	}
	c.htmlHandler = append(
		c.htmlHandler,
		&HTMLParser{selector, f},
	) // Append a new HTMLParser object with the given selector and function to the slice
	c.lock.Unlock() // Unlock the Crawler
}

// ParseJSON is a method of the Crawler type that registers a function to be executed for each JSON response that matches a given strictness condition.
//
// The strict argument determines whether only responses with Content-Type header set to application/json are parsed (true) or any response body that looks like JSON is parsed (false).
// The function takes an interface{} object as an argument and can cast it to any JSON type (string, number, bool, array or map) as needed.
// The Crawler can have multiple JSON parsers, which are stored in a slice and executed in order.
// This method is thread-safe and can be called concurrently.
func (c *Crawler) ParseJSON(strict bool, f HandleJSON) {
	c.lock.Lock() // Lock the Crawler to prevent data race
	if c.jsonHandler == nil {
		c.jsonHandler = make(
			[]*JSONParser,
			0,
			1,
		) // Initialize the JSON handler slice with a capacity of 1 ,  assuming that most Crawlers only need one JSON handler ,  and expand it automatically when needed
	}
	c.jsonHandler = append(
		c.jsonHandler,
		&JSONParser{strict, f},
	) // Appenda new JSONParser object with th e given strictness condition and function to th e slice
	c.lock.Unlock() // Unlock the Crawler
}

// AfterResponse is a method of the Crawler type that registers a function to be executed after each response.
//
// The function takes a Response object as an argument and can read or modify it as needed.
// The Crawler can have multiple response handlers, which are stored in a slice and executed in order.
// This method is thread-safe and can be called concurrently.
func (c *Crawler) AfterResponse(f HandleResponse) {
	c.lock.Lock() // Lock the Crawler to prevent data race
	if c.responseHandler == nil {
		// Initialize the response handler slice with a capacity of 5,
		// assuming that a Crawler does not need too many response handlers,
		// and expand it automatically when needed
		c.responseHandler = make([]HandleResponse, 0, 5)
	}
	c.responseHandler = append(c.responseHandler, f) // Append the given function to the slice
	c.lock.Unlock()                                  // Unlock the Crawler
}

// ProxyPoolAmount is a method of the Crawler type that returns the number of proxy URLs in the Crawler's proxy pool.
// The proxy pool is a slice of strings that store the proxy URLs to be used for sending requests.
func (c Crawler) ProxyPoolAmount() int {
	return len(c.proxyURLPool)
}

// Wait is a method of the Crawler type that blocks until all requests are finished and closes the go pool.
// The go pool is a custom type that manages a fixed number of goroutines for concurrent requests.
func (c *Crawler) Wait() {
	c.wg.Wait()      // Wait for all goroutines to finish
	c.goPool.Close() // Close the go pool
}

// SetProxyInvalidCondition is a method of the Crawler type that sets a custom function to determine whether a proxy URL is invalid or not.
// The function takes a Response object as an argument and returns a boolean value indicating whether the proxy URL should be removed from the proxy pool or not.
// The default condition is based on the status code and error message of the response.
func (c *Crawler) SetProxyInvalidCondition(condition ProxyInvalidCondition) {
	c.proxyInvalidCondition = condition // Assign the given function to c.proxyInvalidCondition
}

// AddProxy is a method of the Crawler type that adds a new proxy URL to the Crawler's proxy pool.
// This method is thread-safe and can be called concurrently.
func (c *Crawler) AddProxy(newProxy string) {
	c.lock.Lock() // Lock the Crawler to prevent data race

	c.proxyURLPool = append(c.proxyURLPool, newProxy) // Append the new proxy URL to c.proxyURLPool

	c.lock.Unlock() // Unlock the Crawler
}

// AddCookie is a method of the Crawler type that adds a new cookie key-value pair to the Crawler's raw cookies string.
// The raw cookies string stores all cookies in one line separated by semicolons and can be used as Cookie header for requests.
// This method is thread-safe and can be called concurrently.
func (c *Crawler) AddCookie(key, val string) {
	c.lock.Lock() // Lock the Crawler to prevent data race

	c.rawCookies += fmt.Sprintf(
		"; %s=%s",
		key,
		val,
	) // Append the new cookie key-value pair to c.rawCookies with semicolon

	c.lock.Unlock() // Unlock the Crawler
}

// SetConcurrency is a method of th e Crawler type that sets th e number of goroutines for concurrent requests and whether to block or panic when an error occurs in th e go pool .
// This method should only be called once before sending any request , otherwise it will panic .
func (c *Crawler) SetConcurrency(count uint64, blockPanic bool) {
	if c.goPool == nil { // If c.goPool has not been initialized yet
		p, err := NewPool(count) // Createa new go pool with th e given count
		if err != nil {
			panic(err) // Panic if creating go pool fails
		}
		p.blockPanic = blockPanic // Set p.blockPanic with th e given blockPanic
		p.log = c.log             // Set p.log with c.log

		c.goPool = p               // Assign p to c.goPool
		c.wg = new(sync.WaitGroup) // Initialize c.wg as a new wait group object
	} else { // If c.goPool has been initialized already
		c.FatalOrPanic(errors.New("`c.goPool` is not nil")) // Panic with an error message
	}
}

// SetRetry sets the retry count and retry condition for the crawler.
func (c *Crawler) SetRetry(count uint32, cond RetryCondition) {
	c.retryCount = count
	c.retryCondition = cond
}

// SetCache sets the cache for the crawler.
func (c *Crawler) SetCache(
	cc Cache,
	compressed bool,
	cacheCondition CacheCondition,
	cacheFields ...CacheField,
) {
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

// ResetCacheFields resets the cache fields for the crawler.
func (c *Crawler) ResetCacheFields(cacheFields ...CacheField) {
	c.cacheFields = cacheFields
}

// UnsetCache unsets the cache for the crawler.
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

// Lock acquires the exclusive lock of the crawler's internal mutex.
func (c Crawler) Lock() {
	c.lock.Lock()
}

// Unlock releases the exclusive lock of the crawler's internal mutex.
func (c Crawler) Unlock() {
	c.lock.Unlock()
}

// RLock acquires the shared lock of the crawler's internal mutex.
func (c Crawler) RLock() {
	c.lock.RLock()
}

// RUnlock releases the shared lock of the crawler's internal mutex.
func (c Crawler) RUnlock() {
	c.lock.RUnlock()
}

// processRequestHandler executes all registered request handler functions
// for a given request in the order they were added, and returns an error if
// any of the handlers return an error.
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

// processResponseHandler executes all registered response handler functions
// for a given response in the order they were added, and returns an error if
// any of the handlers return an error. If the response is invalid, no
// further processing will occur.
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

// processJSONHandler executes all registered JSON handler functions
// for a given response in the order they were added, and returns an error if
// any of the handlers return an error. If there are multiple JSON handlers,
// a warning will be logged recommending that they be combined into a single
// call to ParseJSON. If the response is not a JSON response, processing
// will not occur.
func (c *Crawler) processJSONHandler(r *Response) error {
	if c.jsonHandler == nil {
		return nil
	}

	if len(c.jsonHandler) > 1 {
		if c.log != nil {
			c.Warning(
				"it is recommended to do full processing of the json response in one call to `ParseJSON` instead of multiple calls to `ParseJSON`",
			)
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
					c.Warning(
						`the "Content-Type" of the response header is not of the "json" type`,
						log.NewArg("Content-Type", r.ContentType()),
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

// processHTMLHandler processes the HTML response by parsing it with the html package
// and then executing the registered HTML parsers on selected elements of the parsed document.
func (c *Crawler) processHTMLHandler(r *Response) error {
	if len(c.htmlHandler) == 0 {
		return nil
	}

	if !strings.Contains(strings.ToLower(r.ContentType()), "html") {
		if c.log != nil {
			c.Debug(
				`the "Content-Type" of the response header is not of the "html" type`,
				log.NewArg("Content-Type", r.ContentType()),
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
		// Find the elements in the parsed document that match the selector of the parser
		doc.Find(parser.Selector).Each(func(_ int, s *goquery.Selection) {
			for _, n := range s.Nodes {
				// Execute the Handle function of the parser on the selected element
				parser.Handle(html.NewHTMLElementFromSelectionNode(s, n, i), r)
				i++
			}
		})
	}

	return nil
}

// Debug logs a debug message using the underlying logger.
func (c *Crawler) Debug(msg string, args ...log.Arg) {
	if c.log != nil {
		c.log.Debug(msg, args...)
	}
}

// Info logs an info message using the underlying logger.
func (c *Crawler) Info(msg string, args ...log.Arg) {
	if c.log != nil {
		c.log.Info(msg, args...)
	}
}

// Warning logs a warning message using the underlying logger.
func (c *Crawler) Warning(msg string, args ...log.Arg) {
	if c.log != nil {
		c.log.Warning(msg, args...)
	}
}

// Error logs an error using the underlying logger.
func (c *Crawler) Error(err any, args ...log.Arg) {
	if c.log != nil {
		c.log.Error(err, args...)
	}
}

// Fatal logs a fatal message using the underlying logger and exits the program.
func (c *Crawler) Fatal(err any, args ...log.Arg) {
	if c.log != nil {
		c.log.Fatal(err, args...)
	}
}
