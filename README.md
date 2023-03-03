# predator / 掠食者
高性能爬虫框架。

## 为什么改回`net/http`库？

原因有几个：

1. fasthttp 目前处于稳定状态，但目前没有向后扩展支持 http2/http3 的趋势。当然，对于爬虫来说这一点不算什么，所以这也不是重要的原因。

2. 使用 fasthttp 作为客户端可能会遇到其特有且无法解决的  Close 错误，这个错误会使爬虫程序在没有任何问题的情况下退出或中止，这对爬虫程序来说是致命的。

3. 爬虫可能使用并发（比如 10），但不太可能使用高并发（比如 1000 或更高），这就失去了 fasthttp 的最大优势，此时标准 http 库就是完美替代者，成熟且紧跟新技术。

当然，如果你对 http2、http3 没有执念，fasthttp 作为高并发友好的 server 还是非常好用的，只是作为爬虫的 client 差点意思。 

## 使用

下面是一个示例，基本包含了当前已完成的所有功能，使用方法可以参考注释。

### 1 创建一个 Crawler

```go
import "github.com/go-predator/predator"


func main() {
	c := predator.NewCrawler(
		predator.WithUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:90.0) Gecko/20100101 Firefox/90.0"),
		predator.WithCookies(map[string]string{"JSESSIONID": cookie}),
		predator.WithProxy(ip), // 或者使用代理池 predator.WithProxyPool([]string)
	)
}
```

创建`Crawler`时有一些可选项用来功能增强。所有可选项参考[predator/options.go](https://github.com/go-predator/predator/blob/main/options.go)。

### 2 发送 Get 请求

```go
c.Get("http://www.baidu.com")
```

对请求和响应的处理参考的是 colly，我觉得 colly 的处理方式非常舒服。

```go
// BeforeRequest 可以在发送请求前，对请求进行一些修补
c.BeforeRequest(func(r *predator.Request) error {
	headers := map[string]string{
		"Accept":           "*/*",
		"Accept-Language":  "zh-CN",
		"Accept-Encoding":  "gzip, deflate",
		"X-Requested-With": "XMLHttpRequest",
		"Origin":           "http://example.com",
	}

	r.SetHeaders(headers)
  
	// 请求和响应之间的上下文传递，上下文见下面的上下文示例
	r.Ctx.Put("id", 10)
	r.Ctx.Put("name", "tom")

	return nil
})

c.AfterResponse(func(r *predator.Response) error {
	// 从请求发送的上下文中取值
	id := r.Ctx.GetAny("id").(int)
	name := r.Ctx.Get("name")
	
	// 对于 json 响应，建议使用 gjson 进行处理
	body := gjson.ParseBytes(r.Body)
	amount := body.Get("amount").Int()
	types := body.Get("types").Array()
  
	return nil
})

// 请求语句要在 BeforeRequest 和 AfterResponse 后面调用
c.Get("http://www.baidu.com")
```

### 3 发送 Post 请求

与 Get 请求有一点不同，通常每个 Post 的请求的参数是不同的，而这些参数都在请求体中，在`BeforeRequest`中重新解析请求体获取关键参数虽然可以，但绝非最佳选择。所以在构造 Post 请求时，可以直接传入上下文，用以解决与响应的信息传递。

#### 3.1 普通 POST 表单(application/x-www-form-urlencoded)

```go
// BeforeRequest 可以在发送请求前，对请求进行一些修补
c.BeforeRequest(func(r *predator.Request) error {
	headers := map[string]string{
		"Accept":           "*/*",
		"Accept-Language":  "zh-CN",
		"Accept-Encoding":  "gzip, deflate",
		"X-Requested-With": "XMLHttpRequest",
		"Origin":           "http://example.com",
	}

	r.SetHeaders(headers)
  
	return nil
})

c.AfterResponse(func(r *predator.Response) error {
	// 从请求发送的上下文中取值
	id := r.Ctx.GetAny("id").(int)
	name := r.Ctx.Get("name")
	
	// 对于 json 响应，建议使用 gjson 进行处理
	body := gjson.ParseBytes(r.Body)
	amount := body.Get("amount").Int()
	types := body.Get("types").Array()

	return nil
})


body := map[string]string{"foo": "bar"}

// 在 Post 请求中，应该将关键参数用这种方式放进上下文
ctx, _ := context.AcquireCtx()
ctx.Put("id", 10)
ctx.Put("name", "tom")

c.Post("http://www.baidu.com", body, ctx)
```

如果不需要传入上下文，可以直接用`nil`代替：

```go
c.Post("http://www.baidu.com", body, nil)
```

#### 3.2 复杂 POST 请求(multipart/form-data)

`multipart/form-data`方法需要使用专门的`PostMultipart`方法，示例可能较长，这里不便书写。

使用方法请参考示例：https://github.com/go-predator/predator/blob/main/example/multipart/main.go

#### 3.3 JSON 请求

JSON 请求也有专门的方法`PostJSON`来完成，在使用`PostJSON`时会自动在请求头中添加`Content-Type: application/json`，无需重复添加。当然，你再重新添加一次也可以，最终将会使用你添加的`Content-Type`。

示例：

```go
func main() {
	c := NewCrawler()

	c.AfterResponse(func(r *Response) error {
		fmt.Println(r)

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
```

#### 3.4 其他 POST 请求

虽然以上三种方式已解决大部分的网站的请求，但仍然存在一小部分网站比较特殊，此时需要使用`PostRaw`方法：

```go
func (c *Crawler) PostRaw(URL string, body []byte, ctx pctx.Context) error
```

其中的请求体需要你自行构造，原始请求体可以是任何形式，构造完成后再序列化为`[]byte`作为请求体。

### 4 禁止重定向

http 标准库默认是允许重定向的。

如果你想禁止重定向，可以用如下方法：

```go
c.BeforeRequest(func(r *predator.Request) error {
	r.DoNotFollowRedirects()]
	return nil
})
```

不允许设置全局重定向，只能针对每个请求进行修补。

当然，如果全局重定向呼声高的话，再考虑是否加入。

### 5 上下文

上下文是一个接口，我实现了两种上下文：

- *ReadOp*：基于`sync.Map`实现，适用于读取上下文较多的场景
- *WriteOp*：用`map`实现，适用于读写频率相差不大或写多于读的场景，这是默认采用的上下文

爬虫中如果遇到了读远多于写时就应该换`ReadOp`了，如下代码所示：

```go
ctx, err := AcquireCtx(context.ReadOp)
```

### 6 处理 HTML

爬虫的结果大体可分为两种，一是 HTML 响应，另一种是 JSON 格式的响应。

与 JSON 相比，HTML 需要更多的代码处理。

本框架对 HTML 处理进行了一些函数封装，能方便地通过 css selector 进行元素的查找，可以提取元素中的属性和文本等。

响应头必须是`text/html`或其扩展类型如`text/html; charset=utf-8`才能执行此方法。

```go
c := NewCrawler()

c.ParseHTML("body", func(he *html.HTMLElement) error {
	// 元素内部 HTML
	h, err := he.InnerHTML()
	// 元素整体 HTML
	h, err := he.OuterHTML()
	// 元素内的文本（包括子元素的文本）
	he.Text()
	// 元素的属性
	he.Attr("class")
	// 第一个匹配的子元素
	he.FirstChild("p")
	// 最后一个匹配的子元素
	he.LastChild("p")
	// 第 2 个匹配的子元素
	he.Child("p", 2)
	// 第一个匹配的子元素的属性
	he.ChildAttr("p", "class")
	// 所有匹配到的子元素的属性切片
	he.ChildrenAttr("p", "class")

	return nil
}
```

### 7 异步 / 多协程请求

```go
c := NewCrawler(
	// 使用此 option 时自动使用指定数量的协程池发出请求，不使用此 option 则默认使用同步方式请求
	// 设置的数量不宜过少，也不宜过多，请自行测试设置不同数量时的效率
	WithConcurrency(30),
)

c.AfterResponse(func(r *predator.Response) error {
	// handle response

	return nil
})

for i := 0; i < 10; i++ {
	c.Post(ts.URL+"/post", map[string]string{
		"id": fmt.Sprint(i + 1),
	}, nil)
}

// NOTE: 开启异步时必需执行 Wait 方法
c.Wait()
```

### 8 使用缓存

默认情况下，缓存是不启用的，所有的请求都直接放行。

已经实现的缓存：

- MySQL
- PostgreSQL
- Redis
- SQLite3

缓存接口中有一个方法`Compressed(yes bool)`用来压缩响应的，毕竟有时，响应体非常长，直接保存到数据库中会影响插入和查询时的性能。

这四个接口的使用方法示例：

```go
// MySQL
c := NewCrawler(
	WithCache(&cache.MySQLCache{
		Host:     "127.0.0.1",
		Port:     "3306",
		Database: "predator",
		Username: "root",
		Password: "123456",
	}, false), // false 为关闭压缩，true 为开启压缩，下同
)

// PostgreSQL
c := NewCrawler(
	WithCache(&cache.PostgreSQLCache{
		Host:     "127.0.0.1",
		Port:     "54322",
		Database: "predator",
		Username: "postgres",
		Password: "123456",
	}, false),
)

// Redis
c := NewCrawler(
	WithCache(&cache.RedisCache{
		Addr: "localhost:6379",
	}, true),
)

// SQLite3
c := NewCrawler(
	WithCache(&cache.SQLiteCache{
		URI: uri,  // uri 为数据库存放的位置，尽量加上后缀名 .sqlite
	}, true),
)
// 也可以使用默认值。WithCache 的第一个为 nil 时，
// 默认使用 SQLite 作为缓存，且会将缓存保存在当前
// 目录下的 predator-cache.sqlite 中
c := NewCrawler(WithCache(nil, true))
```

### 9 代理

支持 HTTP 代理和 Socks5 代理。

使用代理时需要加上协议，如：

```go
WithProxyPool([]string{"http://ip:port", "socks5://ip:port"})
```

### 10 日志

日志使用的是流行日志库[`zerolog`](https://github.com/rs/zerolog)。

默认情况下，日志是不开启的，需要手动开启。

`WithLogger`选项需要填入一个参数`*log.Logger`，当填入`nil`时，默认会以`WARNING`等级从终端美化输出。

```go
	c := predator.NewCrawler(
		predator.WithLogger(nil),
	)
```

创建一个日志器：

```go
func NewLogger(level Level, out io.Writer, skip ...int) *Logger
```

第一个参数日志级别不提，第二个 out 参数可以通过下面的方面调用：

- *ToConsole*：美化输出到终端。
- *ToFile*：JSON 格式输出到文件。

- *ToConsoleAndFile*：既美化输出到终端，同时以 JSON 格式输出到文件。

至于第三个参数 skip ，则是决定输出调用位置(skip of statck frames)所用。

日志的完整示例：

```go
import (
	"github.com/go-predator/log"
)

var (
	logger = log.NewLogger(log.INFO, log.ToConsole())
)

func main() {
	c := predator.NewCrawler(
		predator.WithLogger(logger),
	)

	logger.Info(...)
}
```

### 11 关于 JSON

本来想着封装一个 JSON 包用来快速处理 JSON 响应，但是想了一两天也没想出个好办法来，因为我能想到的，[gjson](https://github.com/tidwall/gjson)都已经解决了。

对于 JSON 响应，能用`gjson`处理就不要老想着反序列化了。对于爬虫而言，反序列化是不明智的选择。

当然，如果你确实有反序列化的需求，也不要用标准库，使用封装的 JSON 包中的序列化和反序列化方法比标准库性能高。

```GO
import "github.com/go-predator/predator/json"

json.Marshal()
json.Unmarshal()
json.UnmarshalFromString()
```

对付 JSON 响应，当前足够用了。

## 目标

- [x] 完成对失败响应的重新请求，直到重试了传入的重试次数时才算最终请求失败
- [x] 识别因代理失效而造成的请求失败。当使用代理池时，代理池中剔除此代理；代理池为空时，终止整个爬虫程序
	- 考虑到使用代理必然是因为不想将本地 ip 暴露给目标网站或服务器，所以在使用代理后，当所有代理都失效时，不再继续发出请求
- [x] HTML 页面解析。方便定位查找元素
- [x] json 扩展，用来处理、筛选 json 响应的数据，原生 json 库不适合用在爬虫上
	- 暂时没想到如何封装便捷好用的 json ，当前 json 包中只能算是使用示例
- [x] 协程池，实现在多协程时对每个 goroutine 的复用，避免重复创建
- [x] 定义缓存接口，并完成一种或多种缓存。因为临时缓存在爬虫中并不实用，所以 predator 采用持久化缓存。
  - 默认使用 sqlite3 进行缓存，可以使用已实现的其他缓存数据库，也可以自己实现缓存接口
  - 可用缓存存储有 SQLite3、MySQL、PostgreSQL、Redis
  - 因为采用持久化缓存，所以不实现以内存作为缓存，如果需要请自行根据缓存接口实现
- [x] 数据库管理接口，用来保存爬虫数据，并完成一种或多种数据库的管理
	- SQL 数据库接口已实现了，NoSQL 接口与 SQL 差别较大，就不实现了，如果有使用 NoSQL 的需求，请自己实现
	- 数据库接口没有封装在 Crawler 方法中，根据需要使用，一般场景下够用，复杂场景中仍然需要自己重写数据库管理
- [x] 添加日志
  - 可能还不完善
- [x] 为`Request`和`Response`的请求体`Body`添加池管理，减少 GC 次数
	- body 本身就是`[]byte`，作为引用类型，只要不删除引用关系，其内存就不会被回收
	- 将原求就不是`nil`的 body 截断为 `body[:0]` 即可，不需要使用池来管理
- [x] 对于链式请求或多种请求，可对`POST`和`GET`设置不同的缓存字段
  - [x] 链式请求中可以对每个请求单独设置不同的缓存参数
- [x] 声明一个代理api处理方法，参数为一个整型，可以请求代理池中代理的数量返回代理切片，形成代理池。后续可以每次请求一个代理，用于实时补全代理池。这个方法需用户自行实现。
- [ ] 增加对 robots.txt 的判断，默认遵守 robots.txt 规则，但可以选择忽略
