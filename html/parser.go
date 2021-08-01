/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: parser.go (c) 2021
 * @Created: 2021-07-27 20:35:31
 * @Modified: 2021-08-01 23:02:48
 */

package html

import (
	"bytes"

	"github.com/PuerkitoBio/goquery"
)

// ParseHTML 解析 html
func ParseHTML(body []byte) (*goquery.Document, error) {
	reader := bytes.NewReader(body)
	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return nil, err
	}
	return doc, nil
}
