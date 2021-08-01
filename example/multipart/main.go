/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: main.go (c) 2021
 * @Created: 2021-07-31 11:50:11
 * @Modified: 2021-08-01 18:54:54
 */

package main

import (
	"fmt"

	"github.com/thep0y/predator"
	"github.com/thep0y/predator/context"
	"github.com/thep0y/predator/log"
	"github.com/tidwall/gjson"
)

// 创建请求体
func buildRequestBody(queryID string, page int) map[string]string {
	return map[string]string{
		"id":   queryID,
		"page": fmt.Sprint(page),
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
		"key4": "",
	}
}

// 每一页发出一个请求
func parsePerPage(c *predator.Crawler, u, queryID string, page int) error {
	// 创造请求体
	body := buildRequestBody(queryID, page)

	// 将请求体中的关键参数传入上下文
	ctx, _ := context.AcquireCtx()
	ctx.Put("qid", queryID)
	ctx.Put("page", page)
	return c.PostMultipart(u, body, ctx)
}

func main() {

	logOp := &predator.LogOp{}
	logOp.SetLevel(log.INFO)
	// logOp.ToConsole()
	// logOp.ToFile("test.log")
	logOp.ToConsoleAndFile("test.log")

	crawler := predator.NewCrawler(
		predator.WithCache(nil, true),
		predator.WithUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:90.0) Gecko/20100101 Firefox/90.0"),
		// 设置 20 个并发池
		predator.WithConcurrency(20),
		predator.WithLogger(logOp),
	)

	crawler.BeforeRequest(func(r *predator.Request) {
		headers := map[string]string{
			"Accept":          "*/*",
			"Accept-Language": "zh-CN",
			"Accept-Encoding": "gzip, deflate",
		}

		r.SetHeaders(headers)
	})

	crawler.AfterResponse(func(r *predator.Response) {
		qid := r.Ctx.Get("qid")
		if qid == "" {
			panic("没有从上下文中读取到数据")
		}

		id := gjson.ParseBytes(r.Body).Get("form.id").String()

		if id != qid {
			fmt.Printf("qid=%s, id=%s, 响应是否正确：%v，响应来自缓存：%v\n", qid, id, id == qid, r.FromCache)
		}

	})

	// 请求多页内容
	for i := 0; i < 100; i++ {
		err := parsePerPage(crawler, "https://httpbin.org/post", fmt.Sprint(i+100), i+1)
		if err != nil {
			fmt.Println("爬取失败：", err)
		}
	}

	// 使用并发请求时，需要等待全部任务完成，否则会有一个池容量的任务丢失
	crawler.Wait()

	crawler.ClearCache()
}
