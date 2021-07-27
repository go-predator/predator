/*
 * @Author: Ryan Wong
 * @Email: thepoy@163.com
 * @File Name: craw_test.go
 * @Created: 2021-07-23 09:22:36
 * @Modified: 2021-07-27 13:14:33
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
	"github.com/tidwall/gjson"
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
		c := NewCrawler(WithRetry(uint32(count), func(r Response) bool { return true }))
		So(c.retryCount, ShouldEqual, count)
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

		c.Post(ts.URL+"/login", requestData, nil)
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

		err := c.PostMultipart("https://httpbin.org/post", requestData, nil)
		So(err, ShouldBeNil)
	})

}

func TestProxy(t *testing.T) {
	ts := server()
	defer ts.Close()

	validIP := "http://222.37.130.224:46603"
	u := "http://pv.sohu.com/cityjson?ie=utf-8"

	Convey("测试有效代理", t, func() {
		type TestResponse struct {
			IP string `json:"origin"`
		}
		c := NewCrawler(WithProxy(validIP))

		c.AfterResponse(func(r *Response) {
			body := r.String()[19:]
			body = body[:len(body)-1]
			ip := gjson.Parse(body).Get("cip").String()

			So(ip, ShouldEqual, strings.Split(strings.Split(validIP, "//")[1], ":")[0])
		})

		c.Get(u)
	})

	Convey("测试代理池为空时 panic", t, func() {
		defer func() {
			if err := recover(); err != nil {
				So(err.(error), ShouldEqual, EmptyProxyPoolError)
			}
		}()

		type TestResponse struct {
			IP string `json:"origin"`
		}
		ips := []string{
			"http://14.134.203.22:45104",
			"http://14.134.204.22:45105",
			"http://14.134.205.22:45106",
			"http://14.134.206.22:45107",
			"http://14.134.207.22:45108",
			"http://14.134.208.22:45109",
		}
		c := NewCrawler(WithProxyPool(ips))

		c.Get(u)
	})

	Convey("测试删除代理池中某个或某些无效代理", t, func() {
		type TestResponse struct {
			IP string `json:"origin"`
		}
		// TODO: 写一个代理池的测试用例，池中有效和无效代理全都要有
		ips := []string{
			"http://14.134.204.22:45105",
			validIP,
			"http://14.134.205.22:45106",
			"http://14.134.206.22:45107",
			"http://27.29.155.141:45118",
			"http://14.134.208.22:45109",
		}
		c := NewCrawler(WithProxyPool(ips))

		c.AfterResponse(func(r *Response) {
			body := r.String()[19:]
			body = body[:len(body)-1]
			ip := gjson.Parse(body).Get("cip").String()
			So(c.ProxyPoolAmount(), ShouldBeLessThanOrEqualTo, len(ips))
			So(ip, ShouldEqual, strings.Split(strings.Split(validIP, "//")[1], ":")[0])
		})

		err := c.Get(u)
		So(err, ShouldBeNil)
	})
}

func TestRetry(t *testing.T) {
	ts := server()
	defer ts.Close()

	Convey("测试对失败响应发起重试", t, func() {
		cookie := map[string]string{"test": "ha"}
		c := NewCrawler(
			WithCookies(cookie),
			WithRetry(5, func(r Response) bool {
				return r.StatusCode != 200
			}),
		)

		c.AfterResponse(func(r *Response) {
			So(r.Request.NumberOfRetries(), ShouldEqual, 5)
			So(r.StatusCode, ShouldNotEqual, 200)
		})

		c.Get(ts.URL + "/check_cookie")
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

			c.Post(ts.URL+"/json", nil, nil)
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

		c.Post(ts.URL+"/json", nil, nil)
	})
}
