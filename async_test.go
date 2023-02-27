/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   async_test.go
 * @Created At:  2021-07-31 13:14:09
 * @Modified At: 2023-02-27 10:31:31
 * @Modified By: thepoy
 */

package predator

import (
	"fmt"
	"testing"

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

	return c.PostMultipart(u, mfw, nil)
}

func testAsync(crawler *Crawler, t *testing.T, u string) {
	headers := map[string]string{
		"Accept":          "*/*",
		"Accept-Language": "zh-CN",
		"Accept-Encoding": "gzip, deflate",
	}

	crawler.BeforeRequest(func(r *Request) {
		r.SetHeaders(headers)
	})

	// 请求多个分类的第一页内容
	for i := 0; i < 100; i++ {
		err := parsePerPage(crawler, u, fmt.Sprint(i+100), i+1)
		if err != nil {
			t.Error("爬取失败：", err)
		}
	}
}

func TestAsync(t *testing.T) {
	ts := server()
	defer ts.Close()

	Convey("同步耗时", t, func() {
		defer timeCost("")()
		crawler := NewCrawler(
			WithUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:90.0) Gecko/20100101 Firefox/90.0"),
			RecordRemoteAddr(),
		)

		testAsync(crawler, t, ts.URL+"/post")
	})

	Convey("并发耗时", t, func() {
		defer timeCost("")()
		crawler := NewCrawler(
			WithUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:90.0) Gecko/20100101 Firefox/90.0"),
			WithConcurrency(30, false),
			RecordRemoteAddr(),
		)

		testAsync(crawler, t, ts.URL+"/post")

		crawler.Wait()
	})
}
