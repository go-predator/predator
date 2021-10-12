/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: parser.go
 * @Created: 2021-07-27 20:41:02
 * @Modified: 2021-10-12 09:44:25
 */

package json

import "github.com/tidwall/gjson"

func ParseJSON(body []byte) gjson.Result {
	return gjson.ParseBytes(body)
}
