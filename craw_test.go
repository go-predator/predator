/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: craw_test.go
 * @Created: 2021-07-23 09:22:36
 * @Modified:  2022-02-17 16:19:40
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
	"time"

	"github.com/go-predator/cache"
	"github.com/go-predator/predator/html"
	"github.com/go-predator/predator/log"
	"github.com/go-predator/predator/proxy"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/tidwall/gjson"
	"github.com/valyala/fasthttp"
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
		c := NewCrawler(WithConcurrency(uint64(count), false))
		So(c.goPool.GetCap(), ShouldEqual, count)
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

	mux.HandleFunc("/html", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
<title>Test Page</title>
</head>
<body>
<h1>Hello World</h1>
<p class="description">This is a 1</p>
<p class="description">This is a 2</p>
<p class="description">This is a 3</p>
</body>
</html>
		`))
	})

	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		c := &http.Cookie{Name: "test", Value: "testv", HttpOnly: false}
		http.SetCookie(w, c)
		http.Redirect(w, r, "/html", http.StatusMovedPermanently)
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

	mux.HandleFunc("/post", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(r.FormValue("id")))

			// 随机休眠几秒用于测试并发
			// rand.Seed(time.Now().UnixNano())
			// time.Sleep(time.Duration(rand.Intn(5)) * time.Second)
			return
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

	// 想运行此示例，需要自行更新 cookie 和 auth_token
	Convey("测试 PostMultipart", t, func() {
		c := NewCrawler(
			WithCookies(map[string]string{
				"PHPSESSID": "7ijqglcno1cljiqs76t2vo5oh2",
			}))
		form := NewMultipartForm(
			"-------------------",
			randomBoundary,
		)

		var err error

		form.AppendString("type", "file")
		form.AppendString("action", "upload")
		form.AppendString("timestamp", "1627871450610")
		form.AppendString("auth_token", "f43cdc8a537eff5169dfddb946c2365d1f897b0c")
		form.AppendString("nsfw", "0")
		err = form.AppendFile("source", "/Users/thepoy/Pictures/Nginx.png")
		So(err, ShouldBeNil)

		c.AfterResponse(func(r *Response) {
			status := gjson.ParseBytes(r.Body).Get("status_code").Int()
			So(status, ShouldEqual, fasthttp.StatusOK)
		})

		err = c.PostMultipart("https://imgtu.com/json", form, nil)
		So(err, ShouldBeNil)
	})

}

func TestHTTPProxy(t *testing.T) {
	ts := server()
	defer ts.Close()

	u := "https://api.bilibili.com/x/web-interface/zone?jsonp=jsonp"
	validIP := "http://123.73.209.237:46603"
	Convey("测试有效代理", t, func() {
		c := NewCrawler(
			WithUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/92.0.4515.107 Safari/537.36 Edg/92.0.902.55"),
			WithProxy(validIP),
			WithLogger(nil),
		)

		c.AfterResponse(func(r *Response) {
			ip := gjson.ParseBytes(r.Body).Get("data.addr").String()

			So(ip, ShouldEqual, strings.Split(strings.Split(validIP, "//")[1], ":")[0])
		})

		c.Get(u)
	})

	Convey("测试代理池为空时 panic", t, func() {
		defer func() {
			if err := recover(); err != nil {
				So(err.(proxy.ProxyErr).Code, ShouldEqual, proxy.ErrEmptyProxyPoolCode)
			}
		}()
		ips := []string{
			"http://14.134.203.22:45104",
			"http://14.134.204.22:45105",
			"http://14.134.205.22:45106",
			"http://14.134.206.22:45107",
			"http://14.134.207.22:45108",
			"http://14.134.208.22:45109",
		}
		c := NewCrawler(WithProxyPool(ips), WithLogger(nil))

		c.Get(u)
	})

	Convey("测试删除代理池中某个或某些无效代理", t, func() {
		ips := []string{
			"http://14.134.204.22:45105",
			validIP,
			"http://14.134.205.22:45106",
			"http://14.134.206.22:45107",
			"http://27.29.155.141:45118",
			"http://14.134.208.22:45109",
		}
		c := NewCrawler(WithProxyPool(ips), WithLogger(nil))

		c.AfterResponse(func(r *Response) {
			ip := gjson.ParseBytes(r.Body).Get("data.addr").String()
			So(c.ProxyPoolAmount(), ShouldBeLessThanOrEqualTo, len(ips))
			So(ip, ShouldEqual, strings.Split(strings.Split(validIP, "//")[1], ":")[0])
		})

		err := c.Get(u)
		So(err, ShouldBeNil)
	})

	Convey("测试多个有效代理的随机选择", t, func() {
		count := 5
		u := "http://t.ipjldl.com/index.php/api/entry?method=proxyServer.generate_api_url&packid=0&fa=0&fetch_key=&groupid=0&qty=%d&time=1&pro=&city=&port=1&format=txt&ss=1&css=&dt=1&specialTxt=3&specialJson=&usertype=2"
		client := &fasthttp.Client{}
		body := make([]byte, 0)
		_, body, err := client.Get(body, fmt.Sprintf(u, count))
		if err != nil {
			panic(err)
		}

		ips := strings.Split(string(body), "\r\n")
		for i := 0; i < len(ips); i++ {
			ips[i] = "http://" + ips[i]
		}

		c := NewCrawler(WithProxyPool(ips), WithDefaultLogger())

		c.BeforeRequest(func(r *Request) {
			r.SetHeaders(map[string]string{
				// 避免因 keep-alive 的响应无法改变代理
				"Connection": "close",
			})
		})

		c.AfterResponse(func(r *Response) {
			ip := gjson.ParseBytes(r.Body).Get("data.addr").String()
			t.Log(ip)
		})

		ipu := "https://api.bilibili.com/x/web-interface/zone?jsonp=jsonp"
		for i := 0; i < count*2; i++ {
			err := c.Get(ipu)
			So(err, ShouldBeNil)
		}
	})
}

func TestSocks5Proxy(t *testing.T) {
	proxyIP := "socks5://222.37.211.49:46601"
	u := "https://api.bilibili.com/x/web-interface/zone?jsonp=jsonp"

	Convey("测试有效代理", t, func() {
		c := NewCrawler(
			WithProxy(proxyIP),
		)

		c.AfterResponse(func(r *Response) {
			t.Log(r)

			ip := gjson.ParseBytes(r.Body).Get("data.addr").String()

			So(ip, ShouldEqual, strings.Split(strings.Split(proxyIP, "//")[1], ":")[0])
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

	Convey("测试完整 JSON 请求和响应", t, func() {
		c := NewCrawler()

		c.AfterResponse(func(r *Response) {
			t.Log(r)
		})

		type User struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}

		body := map[string]interface{}{
			"time": 156546535,
			"cid":  "10_18772100220-1625540144276-302919",
			"args": []int{1, 2, 3, 4, 5},
			"dict": map[string]string{
				"mod": "1592215036_002", "extend1": "关注", "t": "1628346994", "eleTop": "778",
			},
			"user": User{"Tom", 13},
		}

		c.PostJSON("https://httpbin.org/post", body, nil)
	})

	Convey("测试带缓存的完整 JSON 请求和响应", t, func() {
		c := NewCrawler(
			WithCache(nil, false, nil, CacheField{RequestBodyParam, "cid"}, CacheField{RequestBodyParam, "user.name"}, CacheField{RequestBodyParam, "user.age"}),
		)

		c.AfterResponse(func(r *Response) {
			t.Log(r.FromCache)
		})

		type User struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}

		body := map[string]interface{}{
			"time": 156546535,
			"cid":  "10_18772100220-1625540144276-302919",
			"args": []int{1, 2, 3, 4, 5},
			"dict": map[string]string{
				"mod": "1592215036_002", "extend1": "关注", "t": "1628346994", "eleTop": "778",
			},
			"user": User{"Tom", 13},
		}

		c.PostJSON("https://httpbin.org/post", body, nil)
	})
}

func TestJSONWithInvalidCacheField(t *testing.T) {
	c := NewCrawler(
		WithCache(nil, false, nil, CacheField{RequestBodyParam, "id"}, CacheField{RequestBodyParam, "user.name"}, CacheField{RequestBodyParam, "user.age"}),
		WithLogger(nil),
	)

	c.AfterResponse(func(r *Response) {
		t.Log(r.FromCache)
	})

	type User struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	body := map[string]interface{}{
		"time": 156546535,
		"cid":  "10_18772100220-1625540144276-302919",
		"args": []int{1, 2, 3, 4, 5},
		"dict": map[string]string{
			"mod": "1592215036_002", "extend1": "关注", "t": "1628346994", "eleTop": "778",
		},
		"user": User{"Tom", 13},
	}

	c.PostJSON("https://httpbin.org/post", body, nil)
}

func TestParseHTML(t *testing.T) {
	ts := server()
	defer ts.Close()

	Convey("测试 HTML 解析", t, func() {
		crawl := NewCrawler()

		Convey("测试解析整体 HTML", func() {
			crawl.ParseHTML("body", func(he *html.HTMLElement, r *Response) {
				h, err := he.OuterHTML()
				So(err, ShouldBeNil)
				So(h, ShouldEqual, `<body>
<h1>Hello World</h1>
<p class="description">This is a 1</p>
<p class="description">This is a 2</p>
<p class="description">This is a 3</p>


		</body>`)
			})
		})

		Convey("测试解析内部 HTML", func() {
			crawl.ParseHTML("body", func(he *html.HTMLElement, r *Response) {
				h, err := he.InnerHTML()
				So(err, ShouldBeNil)
				So(h, ShouldEqual, `
<h1>Hello World</h1>
<p class="description">This is a 1</p>
<p class="description">This is a 2</p>
<p class="description">This is a 3</p>


		`)
			})
		})

		Convey("测试解析内部文本", func() {
			crawl.ParseHTML("title", func(he *html.HTMLElement, r *Response) {
				So(he.Text(), ShouldEqual, "Test Page")
			})
		})

		Convey("测试获取属性", func() {
			crawl.ParseHTML("p", func(he *html.HTMLElement, r *Response) {
				attr := he.Attr("class")
				So(attr, ShouldEqual, "description")
			})
		})

		Convey("测试查找子元素", func() {
			crawl.ParseHTML("body", func(he *html.HTMLElement, r *Response) {
				So(he.FirstChild("p").Attr("class"), ShouldEqual, "description")
				So(he.Child("p", 2).Text(), ShouldEqual, "This is a 2")
				So(he.ChildAttr("p", "class"), ShouldEqual, "description")
				So(len(he.ChildrenAttr("p", "class")), ShouldEqual, 3)
			})
		})

		crawl.Get(ts.URL + "/html")
	})
}

func timeCost() func() {
	start := time.Now()
	return func() {
		tc := time.Since(start)
		fmt.Printf("time cost = %v\n", tc)
	}
}

func TestConcurrency(t *testing.T) {
	ts := server()
	defer ts.Close()

	Convey("测试并发和同步耗时", t, func() {
		Convey("并发", func() {
			start := time.Now()
			c := NewCrawler(
				WithConcurrency(30, false),
			)

			for i := 0; i < 10; i++ {
				err := c.Post(ts.URL+"/post", map[string]string{
					"id": fmt.Sprint(i + 1),
				}, nil)
				So(err, ShouldBeNil)
			}

			delta := time.Since(start)
			t.Log(delta)
		})

		Convey("同步", func() {
			start := time.Now()
			c := NewCrawler()

			for i := 0; i < 10; i++ {
				err := c.Post(ts.URL+"/post", map[string]string{
					"id": fmt.Sprint(i + 1),
				}, nil)
				So(err, ShouldBeNil)
			}

			delta := time.Since(start)
			t.Log(delta)
		})
	})
}

func testCache(c *Crawler, t *testing.T) {
	c.BeforeRequest(func(r *Request) {
		r.Ctx.Put("key", 999)
	})

	c.AfterResponse(func(r *Response) {
		if r.Request.ID == 1 {
			So(r.FromCache, ShouldBeFalse)
		} else {
			So(r.FromCache, ShouldBeTrue)
		}
		val := r.Ctx.GetAny("key").(int)
		So(val, ShouldEqual, 999)
	})

	for i := 0; i < 3; i++ {
		err := c.Get("http://www.baidu.com")
		So(err, ShouldBeNil)
	}
	// 测试环境中清除缓存，生产环境慎重清除
	c.ClearCache()
}

func TestCache(t *testing.T) {
	Convey("测试 SQLite 缓存", t, func() {
		uri := "/tmp/test-cache.sqlite"
		Convey("测试不压缩", func() {
			defer timeCost()()
			c := NewCrawler(
				WithCache(&cache.SQLiteCache{
					URI: uri,
				}, false, nil),
				// WithCache(nil, false),
			)

			testCache(c, t)
		})

		Convey("测试压缩", func() {
			defer timeCost()()
			c := NewCrawler(
				WithCache(&cache.SQLiteCache{
					URI: uri,
				}, false, nil),
				// WithCache(nil, false),
			)

			testCache(c, t)
		})
	})

}

func TestLog(t *testing.T) {
	ts := server()
	defer ts.Close()

	Convey("默认在终端美化输出 INFO 等级\n", t, func() {
		c := NewCrawler(
			WithLogger(nil),
		)

		c.Get(ts.URL)
	})

	Convey("在终端美化输出 DEBUG 等级\n", t, func() {
		l := new(LogOp)
		l.SetLevel(log.DEBUG)
		c := NewCrawler(
			WithLogger(l),
		)

		c.BeforeRequest(func(r *Request) {
			r.Ctx.Put("key", "value")
		})

		c.Get(ts.URL)
	})

	Convey("保存到文件\n", t, func() {
		l := new(LogOp)
		l.SetLevel(log.DEBUG)
		l.ToFile("test.log")

		c := NewCrawler(
			WithLogger(l),
		)

		c.Get(ts.URL)
	})

	Convey("既保存到文件，也输出到终端\n", t, func() {
		l := new(LogOp)
		l.SetLevel(log.DEBUG)
		l.ToConsoleAndFile("test2.log")

		c := NewCrawler(
			WithLogger(l),
		)

		c.BeforeRequest(func(r *Request) {
			r.Ctx.Put("key", "value")
		})

		c.Get(ts.URL)
	})
}

func TestRedirect(t *testing.T) {
	ts := server()
	defer ts.Close()

	Convey("测试默认情况", t, func() {
		c := NewCrawler()

		c.AfterResponse(func(r *Response) {
			So(r.StatusCode, ShouldEqual, 301)
		})

		c.Get(ts.URL + "/redirect")
	})

	Convey("测试设置重定向次数的情况", t, func() {
		c := NewCrawler()

		c.BeforeRequest(func(r *Request) {
			r.AllowRedirect(1)
		})

		c.AfterResponse(func(r *Response) {
			So(r.StatusCode, ShouldEqual, 200)
		})

		c.Get(ts.URL + "/redirect")
	})
}

func getRawCookie(c *Crawler, ts *httptest.Server) string {
	var rawCookie string

	c.AfterResponse(func(r *Response) {
		if r.StatusCode == 301 {
			rawCookie = string(r.Headers.Peek("Set-Cookie"))
		}
	})

	c.Post(ts.URL+"/redirect", map[string]string{"username": "test", "password": "test"}, nil)
	return rawCookie
}

func TestClone(t *testing.T) {
	ts := server()
	defer ts.Close()

	Convey("测试克隆", t, func() {
		c := NewCrawler()

		rawCookie := getRawCookie(c, ts)

		WithRawCookie(rawCookie)(c)
		WithConcurrency(10, false)(c)

		c.AfterResponse(func(r *Response) {
			fmt.Println(r.StatusCode)
			fmt.Println(r)
			So(r.StatusCode, ShouldEqual, 200)
			So(r.String(), ShouldEqual, "ok")
		})

		c.Get(ts.URL + "/check_cookie")
		c.Wait()
	})
}
