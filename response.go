/*
 * @Author: thepoy
 * @Email: email@example.com
 * @File Name: response.go
 * @Created: 2021-07-24 13:34:44
 * @Modified: 2021-07-25 08:43:31
 */

package predator

import (
	"io/ioutil"

	ctx "github.com/thep0y/predator/context"
	"github.com/valyala/fasthttp"
)

type Response struct {
	// 响应状态码
	StatusCode int
	// 二进制请求体
	Body []byte
	// 请求和响应之间共享的上下文
	Ctx ctx.Context
	// 响应对应的请求
	Request *Request
	// 响应头
	Headers *fasthttp.ResponseHeader
}

// Save writes response body to disk
func (r *Response) Save(fileName string) error {
	return ioutil.WriteFile(fileName, r.Body, 0644)
}

func (r *Response) GetSetCookie() string {
	return string(r.Headers.Peek("Set-Cookie"))
}

func (r *Response) ContentType() string {
	return string(r.Headers.Peek("Content-Type"))
}

func (r *Response) String() string {
	return string(r.Body)
}
