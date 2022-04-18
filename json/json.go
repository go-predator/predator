/*
 * @Author:    thepoy
 * @Email:     thepoy@163.com
 * @File Name: json.go
 * @Created:   2021-07-27 20:41:02
 * @Modified:  2022-04-18 13:30:27
 */

package json

import jsoniter "github.com/json-iterator/go"

var json = jsoniter.ConfigCompatibleWithStandardLibrary

func Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

// Unmarshal 对于爬虫，反序列化是浪费资源的事，
// 应尽量使用 gjson 完成对 json 的解析，实在无法
// 用 gjson 解析时，再用此方法进行反序列化。
//
// 这里使用性能更高的第三方库完成反序列化。
func Unmarshal(src []byte, v any) error {
	return json.Unmarshal(src, v)
}

func UnmarshalFromString(src string, v any) error {
	return json.UnmarshalFromString(src, v)
}
