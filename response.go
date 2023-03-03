/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   response.go
 * @Created At:  2021-07-24 13:34:44
 * @Modified At: 2023-03-03 10:59:56
 * @Modified By: thepoy
 */

package predator

import (
	"errors"
	"net/http"
	"os"
	"strconv"

	ctx "github.com/go-predator/predator/context"
	"github.com/go-predator/predator/json"
)

var (
	ErrIncorrectResponse = errors.New("the response status code is not 20X")
)

type Response struct {
	resp *http.Response
	// 响应状态码
	StatusCode StatusCode

	header http.Header

	// 二进制请求体
	Body []byte
	// 请求和响应之间共享的上下文
	Ctx ctx.Context `json:"-"`
	// 响应对应的请求
	Request *Request `json:"-"`
	// 是否从缓存中取得的响应
	FromCache bool
	// 服务器公网 ip
	clientIP string
	timeout  bool
	// Whether the response is valid,
	// html for invalid responses will not be parsed
	invalid bool
}

// Save writes response body to disk
func (r *Response) Save(fileName string) error {
	return os.WriteFile(fileName, r.Body, 0644)
}

// Invalidate marks the current response as invalid and skips the html parsing process
func (r *Response) Invalidate() {
	r.invalid = true
}

func (r *Response) Method() string {
	return r.Request.Method()
}

func (r *Response) ContentType() string {
	return r.header.Get("Content-Type")
}

func (r *Response) ContentLength() uint64 {
	cl := r.header.Get("Content-Length")

	var (
		length uint64
		err    error
	)

	if cl != "" {
		length, err = strconv.ParseUint(cl, 10, 64)
		if err != nil {
			panic(err)
		}
	} else {
		length = uint64(len(r.Body))
	}

	return length
}

func (r *Response) GetSetCookie() string {
	return r.resp.Header.Get("Set-Cookie")
}

func (r *Response) String() string {
	return string(r.Body)
}

type cachedHeaders struct {
	StatusCode    StatusCode
	ContentType   string // this is the most important field
	ContentLength uint64
	Server        []byte
	Location      []byte
}

type cachedResponse struct {
	Body    []byte
	Headers *cachedHeaders
}

func (r *Response) convertHeaders() (*cachedHeaders, error) {
	ch := &cachedHeaders{}
	ch.StatusCode = r.StatusCode
	ch.ContentType = r.ContentType()
	ch.ContentLength = r.ContentLength()
	ch.Server = []byte(r.ClientIP())

	if ch.StatusCode == StatusFound {
		if ch.Location == nil {
			return nil, ErrInvalidResponseStatus
		}
		ch.Location = []byte(r.resp.Header.Get("Location"))
	}

	return ch, nil
}

func (r *Response) Marshal() ([]byte, error) {
	// The cached response does not need to save all the response headers,
	// so the following code is not used to convert the response headers to bytes
	// var buf bytes.Buffer
	// b := bufio.NewWriter(&buf)
	// r.Headers.Write(b)
	// b.Flush()

	var (
		cr  cachedResponse
		err error
	)
	cr.Body = r.Body
	cr.Headers, err = r.convertHeaders()
	if err != nil {
		return nil, err
	}

	return json.Marshal(cr)
}

func (r *Response) Unmarshal(cachedBody []byte) error {
	var (
		cr  cachedResponse
		err error
	)

	err = json.Unmarshal(cachedBody, &cr)
	if err != nil {
		return err
	}

	r.Body = cr.Body
	r.StatusCode = cr.Headers.StatusCode
	r.clientIP = string(cr.Headers.Server)

	if r.header == nil {
		r.header = make(http.Header)
	}

	r.header.Set("Content-Type", cr.Headers.ContentType)
	r.header.Set("Content-Length", strconv.FormatUint(cr.Headers.ContentLength, 10))

	return nil
}

func (r *Response) ClientIP() string {
	return r.clientIP
}

func (r *Response) IsTimeout() bool {
	return r.timeout
}
