/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: craw.go
 * @Created: 2021-07-23 08:52:17
 * @Modified: 2021-07-31 09:20:12
 */

package predator

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/PuerkitoBio/goquery"
	"github.com/thep0y/predator/cache"
	pctx "github.com/thep0y/predator/context"
	"github.com/thep0y/predator/html"
	"github.com/thep0y/predator/json"
	"github.com/valyala/fasthttp"
)

// 方法
type HandleRequest func(r *Request)
type HandleResponse func(r *Response)
type HandleHTML func(he *html.HTMLElement)

// 结构体
type HTMLParser struct {
	Selector string
	Handle   HandleHTML
}

type Crawler struct {
	lock       *sync.RWMutex
	UserAgent  string
	retryCount uint32
	// 重试条件，返回结果为 true 时触发重试
	retryConditions RetryConditions
	client          *fasthttp.Client
	cookies         map[string]string
	goPool          *Pool
	proxyURLPool    []string // TODO: 当前只针对长效代理ip，需要添加代理 ip 替换或删除功能，不提供检查失效功能，由用户自己检查是否失效
	timeout         uint
	requestCount    uint32
	responseCount   uint32
	// 在多协程中这个上下文管理可以用来退出或取消多个协程
	Context context.Context

	// 缓存
	cache cache.Cache

	requestHandler []HandleRequest

	// 响应后处理响应
	responseHandler []HandleResponse
	// 响应后处理 html
	htmlHandler []*HTMLParser
}

func NewCrawler(opts ...CrawlerOption) *Crawler {
	c := new(Crawler)

	c.UserAgent = "Predator"

	c.client = new(fasthttp.Client)

	for _, op := range opts {
		op(c)
	}

	c.lock = &sync.RWMutex{}

	c.Context = context.Background()

	return c
}

/************************* http 请求方法 ****************************/

func (c *Crawler) request(method, URL string, body []byte, headers map[string]string, ctx pctx.Context) error {
	var err error

	reqHeaders := new(fasthttp.RequestHeader)
	reqHeaders.SetMethod(method)
	reqHeaders.Set("User-Agent", c.UserAgent)
	for k, v := range headers {
		reqHeaders.Set(k, v)
	}
	if c.cookies != nil {
		for k, v := range c.cookies {
			reqHeaders.SetCookie(k, v)
		}
	}

	if ctx == nil {
		ctx, err = pctx.NewContext()
		if err != nil {
			return err
		}
	}

	request := &Request{
		URL:     URL,
		Method:  method,
		Headers: reqHeaders,
		Ctx:     ctx,
		Body:    body,
		// 不论请求是否成功，请求计数器都加 1
		ID:      atomic.AddUint32(&c.requestCount, 1),
		crawler: c,
	}

	if c.goPool != nil {
		task := &Task{c, request}
		err = c.goPool.Put(task)
		if err != nil {
			return err
		}
		return nil
	}

	err = c.prepare(request)
	if err != nil {
		return err
	}

	return nil
}

func (c *Crawler) prepare(request *Request) (err error) {
	c.processRequestHandler(request)

	if request.abort {
		return
	}

	var response *Response

	key, err := request.Hash()
	if err != nil {
		return err
	}

	if c.cache != nil {
		response, err = c.checkCache(key)
		if err != nil {
			return
		}
	}

	// 缓存中取到的响应为空时才发出请求
	if response == nil {
		response, err = c.do(request)
		if err != nil {
			return
		}

		// 将得到的响应保存到缓存中
		// TODO: 此处是否用一个 goroutine 完成？
		if c.cache != nil {
			cacheVal, err := response.Marshal()
			if err != nil {
				return err
			}

			if cacheVal != nil {
				err = c.cache.Cache(key, cacheVal)
				if err != nil {
					return err
				}
			}
		}
	} else {
		response.Request = request
		response.Ctx = request.Ctx
	}

	// 处理响应
	c.processResponseHandler(response)

	err = c.processHTMLHandler(response)
	if err != nil {
		return
	}

	return
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
		return nil, err
	}
	resp.FromCache = true
	return &resp, nil
}

func (c *Crawler) do(request *Request) (*Response, error) {
	req := new(fasthttp.Request)
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

	resp := new(fasthttp.Response)

	if err := c.client.Do(req, resp); err != nil {
		if p, ok := isProxyInvalid(err); ok {
			err = c.removeInvalidProxy(p)
			if err != nil {
				panic(err)
			}

			return c.do(request)
		}
	}
	// 只统计服务器响应成功的响应数量
	atomic.AddUint32(&c.responseCount, 1)

	response := &Response{
		StatusCode: resp.StatusCode(),
		Body:       resp.Body(),
		Ctx:        request.Ctx,
		Request:    request,
		Headers:    &resp.Header,
	}

	if c.retryCount > 0 && request.retryCounter < c.retryCount {
		if c.retryConditions(*response) {
			atomic.AddUint32(&request.retryCounter, 1)
			return c.do(request)
		}
	}

	return response, nil
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

func (c *Crawler) Get(URL string) error {
	return c.request(fasthttp.MethodGet, URL, nil, nil, nil)
}

func (c *Crawler) Post(URL string, requestData map[string]string, ctx pctx.Context) error {
	return c.request(fasthttp.MethodPost, URL, createBody(requestData), nil, ctx)
}

type CustomRandomBoundary func() string

func createMultipartBody(boundary string, data map[string]string) []byte {
	dashBoundary := "-----------------------------" + boundary

	var buffer strings.Builder

	for contentType, content := range data {
		buffer.WriteString(dashBoundary + "\r\n")
		buffer.WriteString("Content-Disposition: form-data; name=" + contentType + "\r\n")
		buffer.WriteString("\r\n")
		buffer.WriteString(content)
		buffer.WriteString("\r\n")
	}
	buffer.WriteString(dashBoundary + "--\r\n")
	return []byte(buffer.String())
}

func randomBoundary() string {
	var s strings.Builder
	count := 29
	for i := 0; i < count; i++ {
		if i == 0 {
			s.WriteString(fmt.Sprint(rand.Intn(9) + 1))
		} else {
			s.WriteString(fmt.Sprint(rand.Intn(10)))
		}
	}
	return s.String()
}

func (c *Crawler) PostMultipart(URL string, requestData map[string]string, ctx pctx.Context, boundaryFunc ...CustomRandomBoundary) error {
	if len(boundaryFunc) > 1 {
		return fmt.Errorf("only one boundaryFunc can be passed in at most, but you pass in %d", len(boundaryFunc))
	}

	var boundary string
	if len(boundaryFunc) == 0 {
		boundary = randomBoundary()
	} else {
		boundary = boundaryFunc[0]()
	}

	headers := make(map[string]string)

	headers["Content-Type"] = "multipart/form-data; boundary=---------------------------" + boundary
	body := createMultipartBody(boundary, requestData)
	return c.request(fasthttp.MethodPost, URL, body, headers, ctx)
}

/************************* 公共方法 ****************************/

func (c *Crawler) ClearCache() error {
	if c.cache == nil {
		return NoCacheSet
	}
	return c.cache.Clear()
}

/************************* 公共注册方法 ****************************/

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

func (c Crawler) ProxyPoolAmount() int {
	return len(c.proxyURLPool)
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
				parser.Handle(html.NewHTMLElementFromSelectionNode(s, n, i))
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
		return EmptyProxyPoolError
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

		if len(c.proxyURLPool) == 0 {
			return EmptyProxyPoolError
		}
	}

	// 在池和 ip 都不能匹配时，这个代理 ip 肯定是用户在 BeforeRequest 中传入的，
	// 既然用户传入了代理，其必然是只想通过代理进行爬虫，所以一样报错
	return nil
}
