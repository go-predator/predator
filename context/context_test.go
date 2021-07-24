/*
 * @Author: thepoy
 * @Email: email@example.com
 * @File Name: context_test.go
 * @Created: 2021-07-24 12:18:30
 * @Modified: 2021-07-24 13:12:55
 */

package context

import (
	"bytes"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func putSomeCtx(ctx Context) {
	ctx.Put("four", 4)
	ctx.Put("five", "5")
	ctx.Put("six", '6')
	ctx.Put("seven", []int{7})
	ctx.Put("eight", [1]int{8})
}

func ctxTest(ctx Context) {
	Convey("添加整数", func() {
		ctx.Put("one", 1)
		Convey("获取整数", func() {
			val := ctx.GetAny("one").(int)
			So(val, ShouldEqual, 1)
			Convey("删除添加的整数", func() {
				ctx.Delete("one")
				val := ctx.GetAny("one")
				So(val, ShouldBeNil)
			})
		})
	})
	Convey("添加字符串", func() {
		ctx.Put("two", "2")
		Convey("获取字符串", func() {
			val := ctx.Get("two")
			So(val, ShouldEqual, "2")
			Convey("获取并删除添加的字符串", func() {
				deleted := ctx.GetAndDelete("two")
				val := ctx.Get("two")
				So(deleted.(string), ShouldEqual, "2")
				So(val, ShouldEqual, "")
			})
		})
	})
	Convey("添加字节切片", func() {
		ctx.Put("three", []byte("3"))
		Convey("获取字节切片", func() {
			val := ctx.GetAny("three").([]byte)
			So(bytes.Equal(val, []byte("3")), ShouldBeTrue)
		})
		Convey("遍历上下文", func() {
			ctx.Put("four", 4)
			ctx.Put("five", "5")
			ctx.Put("six", '6')
			ctx.Put("seven", []int{7})
			ctx.Put("eight", [1]int{8})
			val := ctx.ForEach(func(key string, val interface{}) interface{} {
				return val
			})
			So(len(val), ShouldEqual, 6)
		})
	})
	Convey("上下文长度", func() {
		putSomeCtx(ctx)
		So(ctx.Length(), ShouldEqual, 5)
	})
	Convey("清空上下文", func() {
		putSomeCtx(ctx)
		ctx.Clear()
		So(ctx.Length(), ShouldEqual, 0)
	})
}

func TestContext(t *testing.T) {
	Convey("上下文测试", t, func() {
		Convey("读上下文测试", func() {
			ctx, _ := NewContext(ReadOp)
			ctxTest(ctx)
		})
		Convey("写上下文测试", func() {
			ctx, _ := NewContext(WriteOp)
			ctxTest(ctx)
		})
	})
}
