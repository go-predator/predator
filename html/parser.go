/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   parser.go
 * @Created At:  2021-07-27 20:35:31
 * @Modified At: 2023-02-18 22:30:01
 * @Modified By: thepoy
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
