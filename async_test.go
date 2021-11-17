/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: async_test.go
 * @Created: 2021-07-31 13:14:09
 * @Modified:  2021-11-17 11:38:16
 */

package predator

import (
	"fmt"
	"math/rand"
	"strings"
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

func randomBoundary() string {
	var s strings.Builder
	count := 29
	for i := 0; i < count; i++ {
		if i == 0 {
			s.WriteString(fmt.Sprint(rand.Intn(9) + 1))
		} else {
			s.WriteString(fmt.Sprint(rand.Intn(10)))
		}
	}
	return s.String()
}

func parsePerPage(c *Crawler, u, queryID string, page int) error {
	// 创造请求体
	body := buildRequestBody(queryID, page)
	form := NewMultipartForm(
		"-------------------",
		randomBoundary,
	)
	for k, v := range body {
		form.AppendString(k, v)
	}

	// 将请求体中的关键参数传入上下文
	ctx, _ := context.NewContext()
	ctx.Put("qid", queryID)
	ctx.Put("page", page)

	return c.PostMultipart(u, form, ctx)
}

func testAsync(crawler *Crawler, t *testing.T) {
	crawler.BeforeRequest(func(r *Request) {
		headers := map[string]string{
			"Accept":          "*/*",
			"Accept-Language": "zh-CN",
			"Accept-Encoding": "gzip, deflate",
		}

		r.SetHeaders(headers)

	})

	crawler.AfterResponse(func(r *Response) {
		qid := r.Ctx.Get("qid")
		page := r.Ctx.GetAny("page").(int)
		t.Logf("qid=%s page=%d", qid, page)
	})

	// 请求多个分类的第一页内容
	for i := 0; i < 100; i++ {
		err := parsePerPage(crawler, "https://httpbin.org/post", fmt.Sprint(i+100), i+1)
		if err != nil {
			t.Error("爬取失败：", err)
		}
	}
}

func TestAsync(t *testing.T) {
	Convey("同步耗时", t, func() {
		defer timeCost()()
		crawler := NewCrawler(
			WithCache(nil, true, nil),
			WithUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:90.0) Gecko/20100101 Firefox/90.0"),
		)

		testAsync(crawler, t)
		crawler.ClearCache()
	})

	Convey("异步耗时", t, func() {
		defer timeCost()()
		crawler := NewCrawler(
			WithCache(nil, true, nil),
			WithUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:90.0) Gecko/20100101 Firefox/90.0"),
			WithConcurrency(30, false),
		)

		testAsync(crawler, t)

		crawler.Wait()
		crawler.ClearCache()
	})
}
