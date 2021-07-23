/*
 * @Author: Ryan Wong
 * @Email: thepoy@163.com
 * @File Name: craw_test.go
 * @Created: 2021-07-23 09:22:36
 * @Modified: 2021-07-23 14:56:03
 */

package predator

import (
	"encoding/json"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestGet(t *testing.T) {
	Convey("测试同步", t, func() {
		c := NewCrawler(
			WithUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.164 Safari/537.36 Edg/91.0.864.71"),
			WithCookies(map[string]string{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
			}),
		)
		Convey("测试 Get", func() {
			resp, err := c.Get("https://httpbin.org/get", map[string]string{
				"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
				"Accept-Language": "zh-CN",
				"Accept-Encoding": "gzip, deflate",
				"Connection":      "keep-alive",
			})
			if err != nil {
				t.Error(err)
			}
			So(resp.StatusCode(), ShouldEqual, 200)
			t.Log(string(resp.Body()))
		})
		Convey("测试 Post", func() {
			requestData := map[string]string{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
			}
			body, err := json.Marshal(requestData)
			if err != nil {
				t.Error(err)
			}
			resp, err := c.Post("https://httpbin.org/post", map[string]string{"Content-Type": "application/json"}, body)
			if err != nil {
				t.Error(err)
			}
			So(resp.StatusCode(), ShouldEqual, 200)
			t.Log(string(resp.Body()))
		})
		Convey("测试 PostMultipart", func() {
			requestData := map[string]string{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
			}
			resp, err := c.PostMultipart("https://httpbin.org/post", nil, requestData)
			if err != nil {
				t.Error(err)
			}
			So(resp.StatusCode(), ShouldEqual, 200)
			t.Log(string(resp.Body()))
		})
	})
}
