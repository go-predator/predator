/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: parser.go
 * @Created: 2021-07-27 20:41:02
 * @Modified: 2021-07-29 14:12:52
 */

package json

import "github.com/tidwall/gjson"

func ParseJSON(body []byte) gjson.Result {
	return gjson.ParseBytes(body)
}