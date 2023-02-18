/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   parser.go
 * @Created At:  2021-07-27 20:41:02
 * @Modified At: 2023-02-18 22:30:43
 * @Modified By: thepoy
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
