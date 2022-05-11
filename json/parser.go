/*
 * @Author:    thepoy
 * @Email:     thepoy@163.com
 * @File Name: parser.go
 * @Created:   2021-07-27 20:41:02
 * @Modified:  2022-05-11 09:34:28
 */

package json

import "github.com/tidwall/gjson"

type JSONResult = gjson.Result

// ParseBytesToJSON converts `[]byte` variable to JSONResult
func ParseBytesToJSON(body []byte) JSONResult {
	return gjson.ParseBytes(body)
}

// ParseBytesToJSON converts `string` variable to JSONResult
func ParseJSON(body string) JSONResult {
	return gjson.Parse(body)
}
