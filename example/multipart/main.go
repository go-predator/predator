/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: main.go
 * @Created: 2021-07-31 11:50:11
 * @Modified:  2021-11-06 17:27:04
 */

package main

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/go-predator/predator"
	"github.com/go-predator/predator/context"
	"github.com/go-predator/predator/log"
	"github.com/tidwall/gjson"
)

// 自定义生成 boundary 的方法
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

func main() {
	// 创建日志option
	logOp := &predator.LogOp{}
	logOp.SetLevel(log.DEBUG)
	logOp.ToConsole()

	c := predator.NewCrawler(
		// 使用 cookie
		predator.WithCookies(map[string]string{
			"PHPSESSID": "7ijqglcno1cljiqs76t2vo5oh2",
		}),
		// 使用日志
		predator.WithLogger(logOp),
		predator.WithCache(nil, false, nil),
	)

	// 创建 multipart/form-data
	form := predator.NewMultipartForm(
		// boundary 前的横线
		"-------------------",
		// 传入自定义生成 boundary 的方法
		randomBoundary,
	)

	var err error

	// 向 form 中添加表单信息
	form.AppendString("type", "file")
	form.AppendString("action", "upload")
	form.AppendString("timestamp", "1627871450610")
	form.AppendString("auth_token", "f43cdc8a537eff5169dfddb946c2365d1f897b0c")
	form.AppendString("nsfw", "0")
	err = form.AppendFile("source", "/Users/thepoy/Pictures/Nginx.png")
	if err != nil {
		panic(err)
	}

	c.AfterResponse(func(r *predator.Response) {
		// 读取上下文
		val := r.Ctx.Get("foo")
		fmt.Println("value from context:", val)

		status := gjson.ParseBytes(r.Body).Get("status_code").Int()
		fmt.Println("status_code:", status)
	})

	// 创建上下文，并传入一些键值对
	ctx, err := context.NewContext()
	if err != nil {
		panic(err)
	}
	ctx.Put("foo", "bar")

	// 发送 multipart/form-data POST 请求
	err = c.PostMultipart("https://imgtu.com/json", form, ctx)
	if err != nil {
		panic(err)
	}

	// 清除缓存
	c.ClearCache()
}
