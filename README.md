# Predator

A high-performance(maybe) crawler framework based on fasthttp.

## Usage

### 1 Create a new `Crawler`

```go
import "github.com/go-predator/predator"


func main() {
	c := predator.NewCrawler(
		predator.WithUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:90.0) Gecko/20100101 Firefox/90.0"),
		predator.WithCookies(map[string]string{"JSESSIONID": cookie}), // or use predator.WithRawCookie(cookie string)
		predator.WithProxy(ip), // or use a proxy pool -> predator.WithProxyPool([]string)
	)
}
```

Please refer to [predator/options.go](https://github.com/go-predator/predator/blob/main/options.go) for all options。

### 2 Send request with GET method

```go
// BeforeRequest can do some patching on the request before sending it
c.BeforeRequest(func(r *predator.Request) {
	headers := map[string]string{
		"Accept":           "*/*",
		"Accept-Language":  "zh-CN",
		"Accept-Encoding":  "gzip, deflate",
		"X-Requested-With": "XMLHttpRequest",
		"Origin":           "http://example.com",
	}

	r.SetHeaders(headers)
})

c.AfterResponse(func(r *predator.Response) {
	// Get the required parameters from the context
	id := r.Ctx.GetAny("id").(int)
	name := r.Ctx.Get("name")
	page := r.Ctx.Get("page")

	fmt.Println(r.String())
})

// Send a request
c.Get("http://www.example.com")

// Or send a request with context
ctx, _ := context.AcquireCtx()
ctx.Put('page', 1)
ctx.Put("id", 10)
ctx.Put("name", "Tom")
c.GetWithCtx("http://www.example.com", ctx)
```

### 3 Send request with POST method

#### 3.1 Request body's media-type is `application/x-www-form-urlencoded`

```go
// BeforeRequest can do some patching on the request before sending it
c.BeforeRequest(func(r *predator.Request) {
	headers := map[string]string{
		"Accept":           "*/*",
		"Accept-Language":  "zh-CN",
		"Accept-Encoding":  "gzip, deflate",
		"X-Requested-With": "XMLHttpRequest",
		"Origin":           "http://example.com",
	}

	r.SetHeaders(headers)
})

c.AfterResponse(func(r *predator.Response) {
	// Get the required parameters from the context
	id := r.Ctx.GetAny("id").(int)
	name := r.Ctx.Get("name")

	fmt.Println(r.String())
})


body := map[string]string{"foo": "bar"}

// Send a request with context
ctx, _ := context.AcquireCtx()
ctx.Put("id", 10)
ctx.Put("name", "Tom")

c.Post("http://www.example.com", body, ctx)
```

If you don't need to pass a context, you can pass `nil`：

```go
c.Post("http://www.example.com", body, nil)
```

#### 3.2 Request body's media-type is `multipart/form-data`

Please refer to the complete example：https://github.com/go-predator/predator/blob/main/example/multipart/main.go

#### 3.3 Request body's media-type is `application/json`

```go
import (
	...

	"github.com/go-predator/predator"
	"github.com/go-predator/predator/context"
	"github.com/go-predator/predator/json"
)

type User struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func main() {
	c := predator.NewCrawler()

	c.ParseJSON(true, func(j json.JSONResult) {
		fmt.Println(j.Get("json"))
	})

	body := map[string]any{
		"time": 156546535,
		"cid":  "10_18772100220-1625540144276-302919",
		"args": []int{1, 2, 3, 4, 5},
		"dict": map[string]string{
			"mod": "1592215036_002", "t": "1628346994", "eleTop": "778",
		},
		"user": User{"Tom", 13},
	}

	c.PostJSON("https://httpbin.org/post", body, nil)
}
```

#### 3.4 Request body's media-type is others

If the three request functions above cannot meet your needs, please send your own binary request body via `PostRaw`.

```go
func (c *Crawler) PostRaw(URL string, body []byte, ctx pctx.Context) error
```

### 4 Allow Redirects

Redirection is disabled by default.

If you need to use redirects, you need to set the maximum number of redirects allowed via `AllowRedirect` in `BeforeRequest`.

```go
c.BeforeRequest(func(r *predator.Request) {
	if r.URL()[8:12] == "abcd" {
		r.AllowRedirect(1)
	} else if r.URL()[8:12] == "efgh" {
		r.AllowRedirect(3)
	}
})
```

Setting global redirects is not allowed.

### 5 Context

The context is an interface, and the following two contexts are currently implemented:

- _ReadOp_:Based on `sync.Map`, it is suitable for scenarios with many reading contexts

  ```go
  ctx, err := AcquireCtx(context.ReadOp)
  ```

- _WriteOp_(Default):Based on `map`, it is suitable for scenarios where the frequency of reading and writing is not much different or there are more writes than reads. This is the default context

  ```go
  ctx, err := AcquireCtx()
  ```

If you implement the `Context` interface yourself:

```go
ctx := YourContext()
```

### 6 Parse the HTML response

Responses to web requests are mostly **HTML** and **JSON**.

You can use the `ParseHTML` method to find html elements in combination with **CSS selector**.

> :warning: The `Content-Type` of the response header must be `text/html`.

```go
crawl := NewCrawler()

crawl.ParseHTML("#main", func(he *html.HTMLElement) {
	he.String()

	h, err := he.InnerHTML()

	h, err := he.OuterHTML()

	he.Text()

	he.ChildText("#title")

	he.ChildrenText("li>a")

	he.Attr("class")

	he.FirstChild("p")

	he.LastChild("p")

	he.Child("p", 2)

	he.Children("p")

	he.ChildAttr("p", "class")

	he.ChildrenAttr("p", "class")

	he.Parent()

	he.Parents()

	he.Each("li>a", func (i, h) {
		if i < 10 {
			fmt.Println(h.Attr("href"))
			return false
		} else {
			return true
		}
	})

	he.FindChildByText("span.addr", "New York")

	he.FindChildByStripedText("span.addr", "New York") // if addr like '    New York  '
}
```

### 7 Goroutine pool

```go
c := NewCrawler(
	// Use a goroutine pool with a capacity of 30 for web requests
	predator.WithConcurrency(30),
)

c.AfterResponse(func(r *predator.Response) {
	// handle response
})

for i := 0; i < 10; i++ {
	c.Post("http://www.example.com", map[string]string{
		"id": fmt.Sprint(i + 1),
	}, nil)
}

c.Wait()
```

### 8 Cache

By default no cache is used.

[`Cache`](https://github.com/go-predator/predator/blob/main/cache.go) is an interface.

SQLite-based caching is currently implemented.

If the response length is too long, in order to reduce the space usage, you can enable cache compression.

```go
import (
	"github.com/go-predator/cache"
)

// SQLite3
c := NewCrawler(
	predator.WithCache(&cache.SQLiteCache{
		URI: "test.sqlite",
	}, true), // enable compression
)
```

### 9 Proxy

You can use proxy pool:

```go
predator.WithProxyPool([]string{"http://ip:port", "socks5://ip:port"})
```

A proxy is randomly selected from the proxy pool before each request.

When a proxy fails it is automatically removed from the proxy pool, and panic when the proxy pool is empty.

To avoid panic, you can use `WithComplementProxyPool` to supplement the proxy pool when the proxy pool is empty.

```go
func GetProxyIPs() []string {
	api := "http://proxy.api"
	client := &fasthttp.Client{}
	body := make([]byte, 0)
	_, body, err := client.Get(body, api)
		if err != nil {
		panic(err)
	}

	return strings.Split(string(body), "\r\n")
}

predator.WithComplementProxyPool(GetProxyIPs)
```

### 10 Logging

Based on [`zerolog`](https://github.com/rs/zerolog).

Logging is off by default.

Use the `WithLogger` option to enable logging:

```go
func WithLogger(logger *log.Logger) CrawlerOption
```

If `logger` is nil, logs of level WARNING and above will be printed to the console.

```go
	crawler := predator.NewCrawler(
		predator.WithLogger(nil), // equal to predator.WithDefaultLogger()
	)
```

If you want to print lower level logs, refer to the following code:

```go
import "github.com/go-predator/predator/log"

func main() {
	// print to console
	logger := log.NewLogger(log.DEBUG, log.ToConsole(), 1)
	// save to file
	logger := log.NewLogger(log.DEBUG, log.MustToFile("demo.log", -1), 1)
	// print to console and save to file
	logger := log.NewLogger(log.DEBUG, log.MustToConsoleAndFile("demo.log", -1), 1)

	crawler := predator.NewCrawler(
		predator.WithLogger(logger),
	)
}
```

### 11 Other considerations

If you need to serialize some data structures into json strings, or deserialize json strings, it is recommended to use `github.com/go-predator/predator/json` instead of `encdoing/json`.

```GO
import "github.com/go-predator/predator/json"

json.Marshal(any) ([]byte, error)
json.Unmarshal([]byte, any) error
json.UnmarshalFromString(string, any) error
```
