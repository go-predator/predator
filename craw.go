/*
 * @Author: Ryan Wong
 * @Email: thepoy@163.com
 * @File Name: craw.go
 * @Created: 2021-07-23 08:52:17
 * @Modified: 2021-07-23 14:17:50
 */

package http

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/valyala/fasthttp"
)

type Crawler struct {
	UserAgent  string
	retryCount uint
	client     *fasthttp.Client
	cookies    map[string]string
	goCount    uint
}

// TODO: 代理、ua、headers、cookie、上下文通信、缓存接口

func NewCrawler(opts ...CrawlerOption) *Crawler {
	c := new(Crawler)

	c.UserAgent = "Predator"

	c.client = new(fasthttp.Client)

	for _, op := range opts {
		op(c)
	}

	return c
}

func (c Crawler) request(method, URL string, body []byte, headers map[string]string) (*fasthttp.Response, error) {
	req := new(fasthttp.Request)
	req.SetRequestURI(URL)
	req.Header.SetMethod(method)
	req.Header.Add("User-Agent", c.UserAgent)

	if c.cookies != nil {
		for k, v := range c.cookies {
			req.Header.SetCookie(k, v)
		}
	}

	if headers != nil {
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	}

	if method == fasthttp.MethodPost {
		req.SetBody(body)
	}

	resp := new(fasthttp.Response)

	if err := c.client.Do(req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c Crawler) Get(URL string, headers map[string]string) (*fasthttp.Response, error) {
	return c.request(fasthttp.MethodGet, URL, nil, headers)
}

func (c Crawler) Post(URL string, headers map[string]string, body []byte) (*fasthttp.Response, error) {
	return c.request(fasthttp.MethodPost, URL, body, headers)
}

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

func (c Crawler) PostMultipart(URL string, requestData map[string]string, headers map[string]string) (*fasthttp.Response, error) {
	if headers == nil {
		headers = make(map[string]string)
	}
	boundary := randomBoundary()
	headers["Content-Type"] = "multipart/form-data; boundary=---------------------------" + boundary

	body := createMultipartBody(boundary, requestData)
	return c.request(fasthttp.MethodPost, URL, body, headers)
}
