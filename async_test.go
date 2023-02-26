/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   async_test.go
 * @Created At:  2021-07-31 13:14:09
 * @Modified At: 2023-02-26 13:06:56
 * @Modified By: thepoy
 */

package predator

import (
	"fmt"
	"testing"

	"github.com/go-predator/predator/context"
	. "github.com/smartystreets/goconvey/convey"
)

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

func parsePerPage(c *Crawler, u, queryID string, page int) error {
	// 创造请求体
	body := buildRequestBody(queryID, page)
	mfw := NewMultipartFormWriter()

	for k, v := range body {
		mfw.AddValue(k, v)
	}

	// 将请求体中的关键参数传入上下文
	ctx, _ := context.NewContext()
	ctx.Put("qid", queryID)
	ctx.Put("page", page)

	return c.PostMultipart(u, mfw, ctx)
}

func testAsync(crawler *Crawler, t *testing.T) {
	headers := map[string]string{
		"Accept":          "*/*",
		"Accept-Language": "zh-CN",
		"Accept-Encoding": "gzip, deflate",
	}

	crawler.BeforeRequest(func(r *Request) {
		// header := http.Header.Add()
		// r.SetHeaders(headers)
		r.SetHeader(NewHeader(headers))
	})

	crawler.AfterResponse(func(r *Response) {
		qid := r.Ctx.Get("qid")
		page := r.Ctx.GetAny("page").(int)
		t.Logf("qid=%s page=%d", qid, page)
	})

	// 请求多个分类的第一页内容
	for i := 0; i < 100; i++ {
		err := parsePerPage(crawler, "http://localhost:8080/post", fmt.Sprint(i+100), i+1)
		if err != nil {
			t.Error("爬取失败：", err)
		}
	}
}

func TestAsync(t *testing.T) {
	Convey("同步耗时", t, func() {
		defer timeCost("同步")()
		crawler := NewCrawler(
			WithUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:90.0) Gecko/20100101 Firefox/90.0"),
		)

		testAsync(crawler, t)
	})

	Convey("异步耗时", t, func() {
		defer timeCost("并发")()
		crawler := NewCrawler(
			WithUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:90.0) Gecko/20100101 Firefox/90.0"),
			WithConcurrency(30, false),
			// RecordRemoteAddr(),
		)

		testAsync(crawler, t)

		crawler.Wait()
	})
}
