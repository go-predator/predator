/*
 * @Author:    thepoy
 * @Email:     thepoy@163.com
 * @File Name: json_test.go
 * @Created:   2021-07-29 18:53:57
 * @Modified:  2022-04-18 13:30:38
 */

package json

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestJSON(t *testing.T) {
	Convey("测试JSON", t, func() {
		type S struct {
			Name string         `json:"name"`
			Age  int            `json:"age"`
			M    map[string]any `json:"map"`
		}

		m := map[string]any{
			"key1": "value1",
			"key2": 2,
			"key3": 3.1,
			"key4": map[string]int{
				"one": 1,
				"two": 2,
			},
			"key5": S{"tom", 10, map[string]any{"a": 1.222}},
		}

		b, e := Marshal(m)
		So(e, ShouldBeNil)
		t.Log(string(b))
	})
}
