/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   craw_test.go
 * @Created At:  2021-07-23 09:22:36
 * @Modified At: 2023-03-03 22:19:06
 * @Modified By: thepoy
 */

package predator

import (
	"bufio"
	"bytes"
	"errors"
	"net"
	"strconv"

	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-predator/log"
	"github.com/go-predator/predator/html"
	"github.com/go-predator/predator/json"

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
		c := NewCrawler(WithConcurrency(uint64(count), false))
		So(c.goPool.GetCap(), ShouldEqual, count)
	})
	Convey("测试设置重试数量", t, func() {
		count := 5
		c := NewCrawler(WithRetry(uint32(count), func(r *Response) bool { return true }))
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
	var (
		mux  = http.NewServeMux()
		rand = rand.New(rand.NewSource(time.Now().UnixNano()))
	)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write(serverIndexResponse)
	})

	mux.HandleFunc("/sleep", func(w http.ResponseWriter, r *http.Request) {
		t := r.URL.Query().Get("timeout")
		timeout, err := strconv.ParseInt(t, 10, 64)
		if err != nil {
			panic(err)
		}

		time.Sleep(time.Duration(timeout))

		w.WriteHeader(200)
		w.Write(serverIndexResponse)

	})

	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			err := r.ParseForm()
			if err != nil {
				panic(err)
			}

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

			if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
				r.ParseMultipartForm(1024)

				source := r.MultipartForm.File["source"][0]
				w.Write([]byte(source.Filename))

				// body, err := json.Marshal(r.MultipartForm)
				// if err != nil {
				// 	panic(err)
				// }
			} else {
				body, err := json.Marshal(r.PostForm)
				if err != nil {
					panic(err)
				}
				w.Write(body)
			}

			// 随机休眠几秒用于测试并发
			time.Sleep(time.Duration(rand.Intn(50)+100) * time.Millisecond)
			return
		}
	})

	return httptest.NewServer(mux)
}

func download(u, filename string) error {
	resp, err := http.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	err = os.WriteFile(filename, body, fs.ModePerm)
	if err != nil {
		return err
	}

	return nil
}

func TestRequest(t *testing.T) {
	ts := server()
	defer ts.Close()

	Convey("测试请求、响应之间的上下文传递和响应结果", t, func() {
		c := NewCrawler()

		c.BeforeRequest(func(r *Request) error {
			r.Ctx.Put("k", "v")

			return nil
		})

		c.AfterResponse(func(r *Response) error {
			v := r.Ctx.Get("k")
			So(v, ShouldEqual, "v")
			So(bytes.Equal(serverIndexResponse, r.Body), ShouldBeTrue)

			return nil
		})

		c.Get(ts.URL)
	})

	Convey("测试 POST", t, func() {
		requestData := map[string]string{
			"name":     "tom",
			"password": "123456",
		}

		c := NewCrawler()

		c.BeforeRequest(func(r *Request) error {
			r.Ctx.Put("k", 2)

			return nil
		})

		c.AfterResponse(func(r *Response) error {
			v := r.Ctx.GetAny("k").(int)
			So(v, ShouldEqual, 2)

			So(string(r.Body), ShouldEqual, requestData["name"])
			So(r.ContentType(), ShouldEqual, "text/html")
			So(r.ContentLength(), ShouldEqual, 3)
			So(r.Method(), ShouldEqual, "POST")

			return nil
		})

		c.Post(ts.URL+"/login", requestData)
	})

	// 想运行此示例，需要自行更新 cookie 和 auth_token
	Convey("测试 PostMultipart", t, func() {
		var err error

		filename := "test.png"
		err = download("https://s2.loli.net/2023/02/22/CSn8gvNikMe7UpZ.png", filename)
		So(err, ShouldBeNil)

		c := NewCrawler()

		mfw := NewMultipartFormWriter()

		mfw.AppendString("type", "file")
		mfw.AppendString("action", "upload")
		mfw.AppendString("timestamp", "1627871450610")
		mfw.AppendFile("source", filename)

		c.AfterResponse(func(r *Response) error {
			So(r.StatusCode, ShouldEqual, StatusOK)
			So(string(r.Body), ShouldEqual, filename)

			return nil
		})

		err = c.PostMultipart(ts.URL+"/post", mfw, nil)
		So(err, ShouldBeNil)

		err = os.Remove(filename)
		So(err, ShouldBeNil)
	})

}

func TestRequestTimeout(t *testing.T) {
	ts := server()
	defer ts.Close()

	Convey("一个添加了超时控制的请求因超时退出", t, func() {
		timeout := 100 * time.Millisecond

		c := NewCrawler()

		c.BeforeRequest(func(r *Request) error {
			r.SetTimeout(timeout)

			return nil
		})

		err := c.Get(fmt.Sprintf("%s/sleep?timeout=%d", ts.URL, timeout*2))
		So(errors.Is(err, ErrTimeout), ShouldBeTrue)
	})

	Convey("一个添加了超时控制的请求在超时时间内响应", t, func() {
		timeout := 100 * time.Millisecond

		c := NewCrawler()

		c.BeforeRequest(func(r *Request) error {
			r.SetTimeout(2 * time.Second)

			return nil
		})

		err := c.Get(fmt.Sprintf("%s/sleep?timeout=%d", ts.URL, timeout/2))
		So(err, ShouldBeNil)
	})

	Convey("全部请求添加超时控制", t, func() {
		c := NewCrawler(
			WithTimeout(2 * time.Second),
		)

		err := c.Get(fmt.Sprintf("%s/sleep?timeout=%d", ts.URL, time.Second))
		So(err, ShouldBeNil)

		err = c.Get(fmt.Sprintf("%s/sleep?timeout=%d", ts.URL, 3*time.Second))
		So(errors.Is(err, ErrTimeout), ShouldBeTrue)
	})
}

func isPublicIP(IP net.IP) bool {
	if IP.IsLoopback() || IP.IsLinkLocalMulticast() || IP.IsLinkLocalUnicast() {
		return false
	}
	if ip4 := IP.To4(); ip4 != nil {
		switch {
		case ip4[0] == 10:
			return false
		case ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31:
			return false
		case ip4[0] == 192 && ip4[1] == 168:
			return false
		default:
			return true
		}
	}
	return false
}

func getPulicIP() net.IP {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Get("http://4.ipw.cn/")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	return net.ParseIP(strings.TrimSpace(string(b)))
}

func TestSocks5Proxy(t *testing.T) {
	Convey("测试 socks5 代理", t, func() {
		var proxyIP string

		if ip := os.Getenv("socks5_proxy"); ip != "" {
			proxyIP = ip
		} else {
			t.Fatal("socks5 proxy cannot be empy, you must set the `socks5_proxy` environment variable")
		}

		proxyURL, err := url.Parse(proxyIP)
		So(err, ShouldBeNil)

		ip := net.ParseIP(proxyURL.Hostname())
		So(ip, ShouldNotBeNil)

		isLocalIP := !isPublicIP(ip)

		u := "https://api.bilibili.com/x/web-interface/zone?jsonp=jsonp"

		c := NewCrawler(
			WithProxy(proxyURL.String()),
		)

		c.AfterResponse(func(r *Response) error {
			gotIP := gjson.ParseBytes(r.Body).Get("data.addr").String()

			if isLocalIP {
				So(gotIP, ShouldNotEqual, getPulicIP())
			} else {
				So(gotIP, ShouldEqual, ip.String())
			}

			return nil
		})

		err = c.Get(u)
		So(err, ShouldBeNil)
	})
}

func TestHTTPProxy(t *testing.T) {
	ts := server()
	defer ts.Close()

	Convey("测试 http/https 代理", t, func() {
		var proxyIP string

		if ip := os.Getenv("https_proxy"); ip != "" {
			proxyIP = ip
		}
		if proxyIP == "" {
			if ip := os.Getenv("http_proxy"); ip != "" {
				proxyIP = ip
			}
		}

		if proxyIP == "" {
			t.Fatal("http/https proxy cannot be empy, you must set the `https_proxy` or `http_proxy` environment variables")
		}

		proxyURL, err := url.Parse(proxyIP)
		So(err, ShouldBeNil)

		ip := net.ParseIP(proxyURL.Hostname())
		So(ip, ShouldNotBeNil)

		isLocalIP := !isPublicIP(ip)

		u := "https://api.bilibili.com/x/web-interface/zone?jsonp=jsonp"
		c := NewCrawler(WithProxy(proxyURL.String()))

		c.ParseJSON(true, func(j json.JSONResult, r *Response) error {
			gotIP := j.Get("data.addr").String()

			if isLocalIP {
				So(gotIP, ShouldNotEqual, getPulicIP())
			} else {
				So(gotIP, ShouldEqual, ip.String())
			}

			return nil
		})

		err = c.Get(u)
		So(err, ShouldBeNil)
	})
}

func getEnvProxy() []string {
	all_ps := []string{
		os.Getenv("http_proxy"),
		os.Getenv("https_proxy"),
		os.Getenv("socks5_proxy"),
	}

	ps := make([]string, 0)

	for _, p := range all_ps {
		if p != "" {
			ps = append(ps, p)
		}
	}

	return ps
}

func TestProxyPool(t *testing.T) {
	u := "https://api.bilibili.com/x/web-interface/zone?jsonp=jsonp"

	unreachableIPs := []string{
		"https://119.41.192.236:55113",
		"https://111.127.99.83:55109",
	}

	Convey("测试代理池为空时 panic", t, func() {
		ips := make([]string, len(unreachableIPs))
		copy(ips, unreachableIPs)

		run := func() {
			c := NewCrawler(WithProxyPool(ips[:1]), WithTimeout(300*time.Millisecond))

			c.Get(u)
		}

		So(run, ShouldPanic)
	})

	Convey("测试删除代理池中某个或某些无效代理", t, func() {
		ips := getEnvProxy()

		if len(ips) >= 1 {
			ips = ips[:1]
		} else {
			t.Fatal("代理环境变量 [ http_proxy|https_proxy|socks5_proxy ] 不能为空，请先添加一个有效的代理")
		}

		validIP := ips[0]

		ips = append(unreachableIPs, validIP)

		c := NewCrawler(WithProxyPool(ips), WithTimeout(300*time.Millisecond))

		c.AfterResponse(func(r *Response) error {
			ip := gjson.ParseBytes(r.Body).Get("data.addr").String()
			So(c.ProxyPoolAmount(), ShouldBeLessThanOrEqualTo, len(ips))
			So(ip, ShouldEqual, strings.Split(strings.Split(validIP, "//")[1], ":")[0])

			return nil
		})

		c.Get(u)
	})

	// Convey("测试多个有效代理的随机选择", t, func() {
	// 	count := 5
	// 	u := "http://t.ipjldl.com/index.php/api/entry?method=proxyServer.generate_api_url&packid=0&fa=0&fetch_key=&groupid=0&qty=%d&time=1&pro=&city=&port=1&format=txt&ss=1&css=&dt=1&specialTxt=3&specialJson=&usertype=2"
	// 	client := &fasthttp.Client{}
	// 	body := make([]byte, 0)
	// 	_, body, err := client.Get(body, fmt.Sprintf(u, count))
	// 	if err != nil {
	// 		panic(err)
	// 	}

	// 	ips := strings.Split(string(body), "\r\n")
	// 	for i := 0; i < len(ips); i++ {
	// 		ips[i] = "http://" + ips[i]
	// 	}

	// 	c := NewCrawler(WithProxyPool(ips), WithDefaultLogger())

	// 	c.BeforeRequest(func(r *Request) error {
	// 		r.SetHeaders(map[string]string{
	// 			// 避免因 keep-alive 的响应无法改变代理
	// 			"Connection": "close",
	// 		})

	// 		return nil
	// 	})

	// 	c.AfterResponse(func(r *Response) error {
	// 		ip := gjson.ParseBytes(r.Body).Get("data.addr").String()
	// 		t.Log(ip)

	// 		return nil
	// 	})

	// 	ipu := "https://api.bilibili.com/x/web-interface/zone?jsonp=jsonp"
	// 	for i := 0; i < count*2; i++ {
	// 		err := c.Get(ipu)
	// 		So(err, ShouldBeNil)
	// 	}
	// })
}

func TestRetry(t *testing.T) {
	ts := server()
	defer ts.Close()

	Convey("测试对失败响应发起重试", t, func() {
		cookie := map[string]string{"test": "ha"}
		c := NewCrawler(
			WithCookies(cookie),
			WithRetry(5, func(r *Response) bool {
				return r.StatusCode != 200
			}),
		)

		c.AfterResponse(func(r *Response) error {
			So(r.Request.NumberOfRetries(), ShouldEqual, 5)
			So(r.StatusCode, ShouldNotEqual, 200)

			return nil
		})

		c.Get(ts.URL + "/check_cookie")
	})
}

func TestCookies(t *testing.T) {
	ts := server()
	defer ts.Close()

	Convey("测试响应 set-cookie", t, func() {
		c := NewCrawler()

		c.AfterResponse(func(r *Response) error {
			So(r.StatusCode, ShouldEqual, 200)
			So(r.resp.Header.Get("Set-Cookie"), ShouldEqual, "test=testv")

			return nil
		})

		c.Get(ts.URL + "/set_cookie")
	})

	Convey("测试使用 cookie 请求", t, func() {
		Convey("成功", func() {
			cookie := map[string]string{"test": "testv"}
			c := NewCrawler(WithCookies(cookie))

			c.AfterResponse(func(r *Response) error {
				So(r.StatusCode, ShouldEqual, 200)
				So(r.String(), ShouldEqual, "ok")

				return nil
			})

			c.Get(ts.URL + "/check_cookie")
		})
		Convey("失败", func() {
			cookie := map[string]string{"test": "ha"}
			c := NewCrawler(WithCookies(cookie))

			c.AfterResponse(func(r *Response) error {
				So(r.StatusCode, ShouldEqual, 500)
				So(r.String(), ShouldEqual, "nok")

				return nil
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

			c.AfterResponse(func(r *Response) error {
				So(r.StatusCode, ShouldEqual, 403)
				So(r.ContentType(), ShouldEqual, "application/json; charset=UTF-8")

				var j TestResponse
				json.Unmarshal(r.Body, &j)
				So(j.Msg, ShouldEqual, "only allow access with post method")

				return nil
			})

			c.Get(ts.URL + "/json")
		})
		Convey("正确", func() {
			c := NewCrawler()

			c.AfterResponse(func(r *Response) error {
				So(r.StatusCode, ShouldEqual, 400)
				So(r.ContentType(), ShouldEqual, "application/json; charset=UTF-8")

				var j TestResponse
				json.Unmarshal(r.Body, &j)
				So(j.Msg, ShouldEqual, "unkown content type")

				return nil
			})

			c.Post(ts.URL+"/json", nil)
		})
	})

	Convey("测试请求头 Content-Type", t, func() {
		c := NewCrawler()

		c.BeforeRequest(func(r *Request) error {
			r.SetContentType("application/json")

			return nil
		})

		c.AfterResponse(func(r *Response) error {
			So(r.StatusCode, ShouldEqual, 200)
			So(r.ContentType(), ShouldEqual, "application/json; charset=UTF-8")

			var j TestResponse
			json.Unmarshal(r.Body, &j)
			So(j.Msg, ShouldEqual, "ok")

			return nil
		})

		c.Post(ts.URL+"/json", nil)
	})

	Convey("测试完整 JSON 请求和响应", t, func() {
		c := NewCrawler()

		c.AfterResponse(func(r *Response) error {
			t.Log(r)

			return nil
		})

		type User struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}

		body := map[string]any{
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
			WithCache(nil, false, nil, CacheField{requestBodyParam, "cid", nil}, CacheField{requestBodyParam, "user.name", nil}, CacheField{requestBodyParam, "user.age", nil}),
		)

		c.AfterResponse(func(r *Response) error {
			t.Log(r.FromCache)

			return nil
		})

		type User struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}

		body := map[string]any{
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
		WithCache(nil, false, nil, CacheField{requestBodyParam, "id", nil}, CacheField{requestBodyParam, "user.name", nil}, CacheField{requestBodyParam, "user.age", nil}),
		WithLogger(nil),
	)

	c.AfterResponse(func(r *Response) error {
		t.Log(r.FromCache)

		return nil
	})

	type User struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	body := map[string]any{
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
			crawl.ParseHTML("body", func(he *html.HTMLElement, r *Response) error {
				h, err := he.OuterHTML()
				So(err, ShouldBeNil)
				So(h, ShouldEqual, `<body>
<h1>Hello World</h1>
<p class="description">This is a 1</p>
<p class="description">This is a 2</p>
<p class="description">This is a 3</p>


		</body>`)

				return nil
			})
		})

		Convey("测试解析内部 HTML", func() {
			crawl.ParseHTML("body", func(he *html.HTMLElement, r *Response) error {
				h, err := he.InnerHTML()
				So(err, ShouldBeNil)
				So(h, ShouldEqual, `
<h1>Hello World</h1>
<p class="description">This is a 1</p>
<p class="description">This is a 2</p>
<p class="description">This is a 3</p>


		`)
				return nil
			})
		})

		Convey("测试解析内部文本", func() {
			crawl.ParseHTML("title", func(he *html.HTMLElement, r *Response) error {
				So(he.Text(), ShouldEqual, "Test Page")

				return nil
			})
		})

		Convey("测试获取属性", func() {
			crawl.ParseHTML("p", func(he *html.HTMLElement, r *Response) error {
				attr := he.Attr("class")
				So(attr, ShouldEqual, "description")

				return nil
			})
		})

		Convey("测试查找子元素", func() {
			crawl.ParseHTML("body", func(he *html.HTMLElement, r *Response) error {
				So(he.FirstChild("p").Attr("class"), ShouldEqual, "description")
				So(he.Child("p", 2).Text(), ShouldEqual, "This is a 2")
				So(he.ChildAttr("p", "class"), ShouldEqual, "description")
				So(len(he.ChildrenAttr("p", "class")), ShouldEqual, 3)

				return nil
			})
		})

		crawl.Get(ts.URL + "/html")
	})
}

func timeCost(label string) func() {
	start := time.Now()
	return func() {
		tc := time.Since(start)

		if label != "" {
			fmt.Printf("%s > time cost = %v\n", label, tc)
		} else {
			fmt.Printf("> time cost = %v\n", tc)
		}
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
				})
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
				})
				So(err, ShouldBeNil)
			}

			delta := time.Since(start)
			t.Log(delta)
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
		c := NewCrawler(
			WithLogger(log.NewLogger(log.DEBUG, log.ToConsole())),
		)

		c.BeforeRequest(func(r *Request) error {
			r.Ctx.Put("key", "value")

			return nil
		})

		c.Get(ts.URL)
	})

	Convey("保存到文件\n", t, func() {
		c := NewCrawler(
			WithLogger(log.NewLogger(log.DEBUG, log.MustToFile("test.log", -1))),
		)

		c.Get(ts.URL)
	})

	Convey("既保存到文件，也输出到终端\n", t, func() {
		c := NewCrawler(
			WithLogger(log.NewLogger(log.DEBUG, log.MustToConsoleAndFile("test2.log", -1))),
		)

		c.BeforeRequest(func(r *Request) error {
			r.Ctx.Put("key", "value")

			return nil
		})

		c.Get(ts.URL)
	})
}

func TestRedirect(t *testing.T) {
	ts := server()
	defer ts.Close()

	Convey("测试允许重定向（默认）", t, func() {
		c := NewCrawler()

		c.AfterResponse(func(r *Response) error {
			So(r.StatusCode, ShouldEqual, StatusMovedPermanently)

			return nil
		})

		c.Get(ts.URL + "/redirect")
	})

	Convey("测试禁止重定向", t, func() {
		c := NewCrawler()

		c.BeforeRequest(func(r *Request) error {
			r.DoNotFollowRedirects()

			return nil
		})

		c.AfterResponse(func(r *Response) error {
			So(r.StatusCode, ShouldEqual, StatusOK)

			return nil
		})

		c.Get(ts.URL + "/redirect")
	})
}

func getRawCookie(c *Crawler, ts *httptest.Server) string {
	var rawCookie string

	c.AfterResponse(func(r *Response) error {
		if r.StatusCode == StatusMovedPermanently {
			rawCookie = r.resp.Header.Get("Set-Cookie")
		}

		return nil
	})

	c.Post(ts.URL+"/redirect", map[string]string{"username": "test", "password": "test"})
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

		c.AfterResponse(func(r *Response) error {
			So(r.StatusCode, ShouldEqual, 200)
			So(r.String(), ShouldEqual, "ok")

			return nil
		})

		c.Get(ts.URL + "/check_cookie")
		c.Wait()
	})
}
