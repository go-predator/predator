/*
 * @Author: Ryan Wong
 * @Email: thepoy@163.com
 * @File Name: craw_test.go
 * @Created: 2021-07-23 09:22:36
 * @Modified: 2021-07-25 10:18:42
 */

package predator

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNewCrawler(t *testing.T) {
	Convey("测试设置 UA", t, func() {
		for _, ua := range []string{"foo", "bar"} {
			c := NewCrawler(WithUserAgent(ua))
			So(c.UserAgent, ShouldEqual, ua)
		}
	})
	Convey("测试设置 cookies", t, func() {
		cookie := map[string]string{"foo": "bar"}
		c := NewCrawler(WithCookies(cookie))
		So(c.cookies, ShouldEqual, cookie)
	})
	Convey("测试设置指定并发数量", t, func() {
		count := 10
		c := NewCrawler(WithConcurrent(uint(count)))
		So(c.goCount, ShouldEqual, count)
	})
	Convey("测试设置重试数量", t, func() {
		count := 5
		c := NewCrawler(WithRetry(uint(count)))
		So(c.retryCount, ShouldEqual, count)
	})
	Convey("测试设置代理", t, func() {
		p := "http://localhost:5000"
		c := NewCrawler(WithProxy(p))
		So(c.proxyURL, ShouldEqual, "localhost:5000")
	})
	Convey("测试设置代理池", t, func() {
		pp := make([]string, 0, 5)
		for i := 1; i <= 5; i++ {
			pp = append(pp, fmt.Sprintf("http://localhost:%d000", i))
		}
		c := NewCrawler(WithProxyPool(pp))
		So(reflect.DeepEqual(c.proxyURLPool, pp), ShouldBeTrue)
	})
}

var serverIndexResponse = []byte("hello world\n")

func server() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write(serverIndexResponse)
	})

	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(r.FormValue("name")))
		}
	})

	mux.HandleFunc("/set_cookie", func(w http.ResponseWriter, r *http.Request) {
		c := &http.Cookie{Name: "test", Value: "testv", HttpOnly: false}
		http.SetCookie(w, c)
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("/check_cookie", func(w http.ResponseWriter, r *http.Request) {
		cs := r.Cookies()
		if len(cs) != 1 || r.Cookies()[0].Value != "testv" {
			w.WriteHeader(500)
			w.Write([]byte("nok"))
			return
		}
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		if r.Method != "POST" {
			w.WriteHeader(403)
			w.Write([]byte(`{"msg": "only allow access with post method"}`))
			return
		}

		ct := r.Header.Get("Content-Type")
		if ct != "application/json" {
			w.WriteHeader(400)
			w.Write([]byte(`{"msg": "unkown content type"}`))
			return
		}

		w.WriteHeader(200)
		w.Write([]byte(`{"msg": "ok"}`))
	})

	mux.HandleFunc("/large_binary", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		ww := bufio.NewWriter(w)
		defer ww.Flush()
		for {
			// have to check error to detect client aborting download
			if _, err := ww.Write([]byte{0x41}); err != nil {
				return
			}
		}
	})

	return httptest.NewServer(mux)
}

func TestRequest(t *testing.T) {
	ts := server()
	defer ts.Close()

	Convey("测试请求、响应之间的上下文传递和响应结果", t, func() {
		c := NewCrawler()

		c.BeforeRequest(func(r *Request) {
			r.Ctx.Put("k", "v")
		})

		c.AfterResponse(func(r *Response) {
			v := r.Ctx.Get("k")
			So(v, ShouldEqual, "v")
			So(bytes.Equal(serverIndexResponse, r.Body), ShouldBeTrue)
		})

		c.Get(ts.URL)
	})

	Convey("测试 POST", t, func() {
		requestData := map[string]string{
			"name":     "tom",
			"password": "123456",
		}

		c := NewCrawler()

		c.BeforeRequest(func(r *Request) {
			r.Ctx.Put("k", 2)
		})

		c.AfterResponse(func(r *Response) {
			v := r.Ctx.GetAny("k").(int)
			So(v, ShouldEqual, 2)
			So(string(r.Body), ShouldEqual, requestData["name"])
			So(string(r.Headers.Peek("Content-Type")), ShouldEqual, "text/html")

		})

		c.Post(ts.URL+"/login", requestData)
	})

	Convey("测试 PostMultipart", t, func() {
		type TestResponse struct {
			Form map[string]string `json:"form"`
		}

		c := NewCrawler()
		requestData := map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
			"key4": "",
		}

		c.AfterResponse(func(r *Response) {
			So(r.StatusCode, ShouldEqual, 200)

			var f TestResponse
			json.Unmarshal(r.Body, &f)
			So(reflect.DeepEqual(f.Form, requestData), ShouldBeTrue)
		})

		err := c.PostMultipart("https://httpbin.org/post", requestData)
		So(err, ShouldBeNil)
	})

	Convey("测试代理 ip 是否生效", t, func() {
		type TestResponse struct {
			IP string `json:"origin"`
		}

		ip := "http://115.59.245.65:45120"
		c := NewCrawler(WithProxy(ip))

		c.AfterResponse(func(r *Response) {
			var j TestResponse
			json.Unmarshal(r.Body, &j)
			So(j.IP, ShouldEqual, strings.Split(strings.Split(ip, "//")[1], ":")[0])
		})

		err := c.Get("https://httpbin.org/ip")
		So(err, ShouldBeNil)
	})
}

func TestCookies(t *testing.T) {
	ts := server()
	defer ts.Close()

	Convey("测试响应 set-cookie", t, func() {
		c := NewCrawler()

		c.AfterResponse(func(r *Response) {
			So(r.StatusCode, ShouldEqual, 200)
			So(string(r.Headers.Peek("Set-Cookie")), ShouldEqual, "test=testv")
		})

		c.Get(ts.URL + "/set_cookie")
	})

	Convey("测试使用 cookie 请求", t, func() {
		Convey("成功", func() {
			cookie := map[string]string{"test": "testv"}
			c := NewCrawler(WithCookies(cookie))

			c.AfterResponse(func(r *Response) {
				So(r.StatusCode, ShouldEqual, 200)
				So(r.String(), ShouldEqual, "ok")
			})

			c.Get(ts.URL + "/check_cookie")
		})
		Convey("失败", func() {
			cookie := map[string]string{"test": "ha"}
			c := NewCrawler(WithCookies(cookie))

			c.AfterResponse(func(r *Response) {
				So(r.StatusCode, ShouldEqual, 500)
				So(r.String(), ShouldEqual, "nok")
			})

			c.Get(ts.URL + "/check_cookie")
		})
	})
}

func TestJSON(t *testing.T) {
	ts := server()
	defer ts.Close()

	type TestResponse struct {
		Msg string `json:"msg"`
	}

	Convey("测试请求方法是否正确", t, func() {
		Convey("错误", func() {
			c := NewCrawler()

			c.AfterResponse(func(r *Response) {
				So(r.StatusCode, ShouldEqual, 403)
				So(r.ContentType(), ShouldEqual, "application/json; charset=UTF-8")

				var j TestResponse
				json.Unmarshal(r.Body, &j)
				So(j.Msg, ShouldEqual, "only allow access with post method")
			})

			c.Get(ts.URL + "/json")
		})
		Convey("正确", func() {
			c := NewCrawler()

			c.AfterResponse(func(r *Response) {
				So(r.StatusCode, ShouldEqual, 400)
				So(r.ContentType(), ShouldEqual, "application/json; charset=UTF-8")

				var j TestResponse
				json.Unmarshal(r.Body, &j)
				So(j.Msg, ShouldEqual, "unkown content type")
			})

			c.Post(ts.URL+"/json", nil)
		})
	})

	Convey("测试请求头 Content-Type", t, func() {
		c := NewCrawler()

		c.BeforeRequest(func(r *Request) {
			r.SetContentType("application/json")
		})

		c.AfterResponse(func(r *Response) {
			So(r.StatusCode, ShouldEqual, 200)
			So(r.ContentType(), ShouldEqual, "application/json; charset=UTF-8")

			var j TestResponse
			json.Unmarshal(r.Body, &j)
			So(j.Msg, ShouldEqual, "ok")
		})

		c.Post(ts.URL+"/json", nil)
	})
}
