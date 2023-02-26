/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   map_test.go
 * @Created At:  2023-02-20 20:48:06
 * @Modified At: 2023-02-26 09:46:56
 * @Modified By: thepoy
 */

package predator

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestMap(t *testing.T) {
	Convey("测试重置 map", t, func() {
		m := map[string]int{
			"1": 1,
			"2": 2,
			"3": 3,
		}
		ResetMap(m)

		So(m, ShouldNotBeNil)
		So(len(m), ShouldEqual, 0)
	})

	Convey("测试重置空 map", t, func() {
		var m map[string]int

		ResetMap(m)

		So(m, ShouldBeNil)
		So(len(m), ShouldEqual, 0)
	})
}
