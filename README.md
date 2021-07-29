# predator / 掠食者
基于 fasthttp 开发的高性能爬虫框架

### 使用

下面是一个示例，基本包含了当前已完成的所有功能，使用方法可以参考注释。

#### 1 创建一个 Crawler

```go
import "github.com/thep0y/predator"


func main() {
	crawler := predator.NewCrawler(
		predator.WithUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:90.0) Gecko/20100101 Firefox/90.0"),
		predator.WithCookies(map[string]string{"JSESSIONID": cookie}),
		predator.WithProxy(ip), // 或者使用代理池 predator.WithProxyPool([]string)
	)
}
```

创建`Crawler`时有一些可选项用来功能增强。所有可选项参考[predator/options.go](https://github.com/thep0y/predator/blob/main/options.go)。

#### 2 发送 Get 请求

```go
crawler.Get("http://www.baidu.com")
```

对请求和响应的处理参考的是 colly，我觉得 colly 的处理方式非常舒服。

```go
// BeforeRequest 可以在发送请求前，对请求进行一些修补
crawler.BeforeRequest(func(r *predator.Request) {
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
})

crawler.AfterResponse(func(r *predator.Response) {
	// 从请求发送的上下文中取值
	id := r.Ctx.GetAny("id").(int)
	name := r.Ctx.Get("name")
	
	// 对于 json 响应，建议使用 gjson 进行处理
	body := gjson.ParseBytes(r.Body)
	amount := body.Get("amount").Int()
	types := body.Get("types").Array()
})

// 请求语句要在 BeforeRequest 和 AfterResponse 后面调用
crawler.Get("http://www.baidu.com")
```

#### 3 发送 Post 请求

与 Get 请求有一点不同，通常每个 Post 的请求的参数是不同的，而这些参数都在请求体中，在`BeforeRequest`中处理请求体虽然可以，但绝非最佳选择，所以在构造 Post 请求时，可以直接传入上下文，用以解决与响应的信息传递。

```go
// BeforeRequest 可以在发送请求前，对请求进行一些修补
crawler.BeforeRequest(func(r *predator.Request) {
	headers := map[string]string{
		"Accept":           "*/*",
		"Accept-Language":  "zh-CN",
		"Accept-Encoding":  "gzip, deflate",
		"X-Requested-With": "XMLHttpRequest",
		"Origin":           "http://example.com",
	}

	r.SetHeaders(headers)
})

crawler.AfterResponse(func(r *predator.Response) {
	// 从请求发送的上下文中取值
	id := r.Ctx.GetAny("id").(int)
	name := r.Ctx.Get("name")
	
	// 对于 json 响应，建议使用 gjson 进行处理
	body := gjson.ParseBytes(r.Body)
	amount := body.Get("amount").Int()
	types := body.Get("types").Array()
})


body := map[string]string{"foo": "bar"}

// 在 Post 请求中，应该将关键参数用这种方式放进上下文
ctx, _ := context.NewContext()
ctx.Put("id", 10)
ctx.Put("name", "tom")

crawler.Post("http://www.baidu.com", body, ctx)
```

如果不需要传入上下文，可以直接用`nil`代替：

```go
crawler.Post("http://www.baidu.com", body, nil)
```

#### 4 发送 multipart/form-data 请求

`multipart/form-data`方法需要使用专门的`PostMultipart`方法，只是当前请求体只支持`map[string]string`，没有别的原因，因为我只用到这种类型，如果以后有别的需求，再改成`map[string]interface{}`。

我在爬某网站时的示例如下：

```go
func buildRequestBody(queryID string, page uint) map[string]string {
	return map[string]string{
		"id":                queryID,
		"filter":            "",
		"page":              fmt.Sprint(page),
		"size":              "30",
		"language":          "CN",
		"industryFlag":      "1",
	}
}

func parsePerPage(u, queryID string, cty, page uint) error {
	// 创造请求体
	body := buildRequestBody(queryID, page)

	// 将请求体中的关键参数传入上下文
	ctx, _ := context.NewContext()
	ctx.Put("cty", cty)
	ctx.Put("qid", queryID)
	ctx.Put("page", page)

	return crawler.PostMultipart(u, body, ctx)
}

func Run() {
	crawler.BeforeRequest(func(r *predator.Request) {
		header := map[string]string{
			"Accept":           "*/*",
			"Accept-Language":  "zh-CN",
			"Accept-Encoding":  "gzip, deflate",
			"X-Requested-With": "XMLHttpRequest",
			"Origin":           "http://jszl.patsev.com",
			"Referer":          "http://jszl.patsev.com/pldb/route/hostingplatform/search/searchIndex",
		}

		r.SetHeaders(headers)
	})

	crawler.AfterResponse(func(r *predator.Response) {
		cty := r.Ctx.GetAny("cty").(uint)

		total := gjson.ParseBytes(r.Body).Get("total").Int()

		log.Debugf("分类 [ %d ] 有 %d 条结果", cty, total)
	})
	
	// 请求多个分类的第一页内容
	var wg sync.WaitGroup
	for cty, qid := range TypesID {
		wg.Add(1)
		go func(cty uint, qid string) {
			defer wg.Done()

			err := parsePerPage(u, qid, cty, 1)
			if err != nil {
				log.Errorf("分类 [ id=%d ] 爬取第一页时失败：%s", cty, err)
			}
		}(cty, qid)
	}
	wg.Wait()
}
```

#### 5 上下文

上下文是一个接口，我实现了两种上下文：

- *ReadOp*：基于`sync.Map`实现，适用于读取上下文较多的场景
- *WriteOp*：用`map`实现，适用于读写频频率相差不大或写多于读的场景，这是默认采用的上下文

爬虫中如果遇到了读远多于写时就应该换`ReadOp`了，如下代码所示：

```go
ctx, err := NewContext(context.ReadOp)
```

#### 6 处理 HTML

爬虫的结果大体可分为两种，一是 HTML 响应，另一种是 JSON 格式的响应。

与 JSON 相比，HTML 需要更多的代码处理。

本框架对 HTML 处理进行了一些函数封装，能方便地通过 css selector 进行元素的查找，可以提取元素中的属性和文本等。

```go
crawl := NewCrawler()

crawl.ParseHTML("body", func(he *html.HTMLElement) {
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
}
```

#### 7 异步 / 多协程请求

```go
c := NewCrawler(
	// 使用此 option 时自动使用指定数量的协程池发出请求，不使用此 option 则默认使用同步方式请求
	// 设置的数量不宜过少，也不宜过多，请自行测试设置不同数量时的效率
	WithConcurrency(30),
)

crawler.AfterResponse(func(r *predator.Response) {
	// handle response
})

for i := 0; i < 10; i++ {
	c.Post(ts.URL+"/post", map[string]string{
		"id": fmt.Sprint(i + 1),
	}, nil)
}
```

#### 8 关于 JSON

本来想着封装一个 JSON 包用来快速处理 JSON 响应，但是想了一两天也没想出个好办法来，因为我能想到的，[gjson](https://github.com/tidwall/gjson)都已经解决了。

对于 JSON 响应，能用`gjson`处理就不要老想着反序列化了。对于爬虫而言，反序列化是不明智的选择。

当然，如果你确实有反序列化的需求，也不要用标准库，使用封装的 JSON 包中的序列化和反序列化方法比标准库性能高。

```GO
import "github.com/thep0y/predator/json"

json.Marshal()
json.Unmarshal()
json.UnmarshalFromString()
```

对付 JSON 响应，当前足够用了。

### 目标

- [x] 完成对失败响应的重新请求，直到重试了传入的重试次数时才算最终请求失败
- [x] 识别因代理失效而造成的请求失败。当使用代理池时，代理池中剔除此代理；代理池为空时，终止整个爬虫程序
	- 考虑到使用代理必然是因为不想将本地 ip 暴露给目标网站或服务器，所以在使用代理后，当所有代理都失效时，不再继续发出请求
- [x] HTML 页面解析。方便定位查找元素
- [x] json 扩展，用来处理、筛选 json 响应的数据，原生 json 库不适合用在爬虫上
	- 暂时没想到如何封装便捷好用的 json ，当前 json 包中只能算是使用示例
- [x] 协程池，实现在多协程时对每个 goroutine 的复用，避免重复创建
- [ ] 定义缓存接口，并完成一种或多种缓存
- [ ] 数据库管理接口，用来保存爬虫数据，并完成一种或多种数据库的管理
