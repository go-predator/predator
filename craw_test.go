/*
 * @Author: Ryan Wong
 * @Email: thepoy@163.com
 * @File Name: craw_test.go
 * @Created: 2021-07-23 09:22:36
 * @Modified: 2021-07-24 22:08:50
 */

package predator

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNewCrawler(t *testing.T) {
	Convey("测试 UA", t, func() {
		for _, ua := range []string{"foo", "bar"} {
			c := NewCrawler(WithUserAgent(ua))
			So(c.UserAgent, ShouldEqual, ua)
		}
	})
	Convey("测试 cookies", t, func() {
		cookie := map[string]string{"foo": "bar"}
		c := NewCrawler(WithCookies(cookie))
		So(c.cookies, ShouldEqual, cookie)
	})
	Convey("测试指定并发数量", t, func() {
		count := 10
		c := NewCrawler(WithConcurrent(uint(count)))
		So(c.goCount, ShouldEqual, count)
	})
	Convey("测试重试数量", t, func() {
		count := 5
		c := NewCrawler(WithRetry(uint(count)))
		So(c.retryCount, ShouldEqual, count)
	})
	Convey("测试代理", t, func() {
		p := "http://localhost:5000"
		c := NewCrawler(WithProxy(p))
		So(c.proxyURL, ShouldEqual, p)
	})
	Convey("测试代理池", t, func() {
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
}

// func TestGet(t *testing.T) {
// 	Convey("测试同步", t, func() {
// 		c := NewCrawler(
// 			WithUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.164 Safari/537.36 Edg/91.0.864.71"),
// 			WithCookies(map[string]string{
// 				"key1": "value1",
// 				"key2": "value2",
// 				"key3": "value3",
// 			}),
// 		)
// 		Convey("测试 Get", func() {
// 			err := c.Get("https://httpbin.org/get", map[string]string{
// 				"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
// 				"Accept-Language": "zh-CN",
// 				"Accept-Encoding": "gzip, deflate",
// 				"Connection":      "keep-alive",
// 			})
// 			if err != nil {
// 				t.Error(err)
// 			}
// 			So(resp.StatusCode(), ShouldEqual, 200)
// 			t.Log(string(resp.Body()))
// 		})
// 		Convey("测试 Post", func() {
// 			requestData := map[string]string{
// 				"key1": "value1",
// 				"key2": "value2",
// 				"key3": "value3",
// 			}
// 			body, err := json.Marshal(requestData)
// 			if err != nil {
// 				t.Error(err)
// 			}
// 			resp, err := c.Post("https://httpbin.org/post", map[string]string{"Content-Type": "application/json"}, body)
// 			if err != nil {
// 				t.Error(err)
// 			}
// 			So(resp.StatusCode(), ShouldEqual, 200)
// 			t.Log(string(resp.Body()))
// 		})
// 		Convey("测试 PostMultipart", func() {
// 			requestData := map[string]string{
// 				"key1": "value1",
// 				"key2": "value2",
// 				"key3": "value3",
// 			}
// 			resp, err := c.PostMultipart("https://httpbin.org/post", nil, requestData)
// 			if err != nil {
// 				t.Error(err)
// 			}
// 			So(resp.StatusCode(), ShouldEqual, 200)
// 			t.Log(string(resp.Body()))
// 		})
// 	})
// }
