/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: element.go (c) 2021
 * @Created: 2021-07-27 20:35:31
 * @Modified: 2021-08-01 23:02:45
 */

package html

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

type HTMLElement struct {
	Name  string
	DOM   *goquery.Selection
	Index int
	Node  *html.Node
}

// NewHTMLElementFromSelectionNode 创建一个元素
func NewHTMLElementFromSelectionNode(s *goquery.Selection, n *html.Node, index int) *HTMLElement {
	return &HTMLElement{
		Name:  n.Data,
		DOM:   s,
		Index: index,
		Node:  n,
	}
}

// Attr 获取 HTML 元素的一个属性值
func (he HTMLElement) Attr(key string) string {
	for _, attr := range he.Node.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

// OuterHTML 当前元素的整体 HTML 代码
func (he HTMLElement) OuterHTML() (string, error) {
	return goquery.OuterHtml(he.DOM)
}

// InnerHTML 当前元素的内部 HTML 代码
func (he HTMLElement) InnerHTML() (string, error) {
	return he.DOM.Html()
}

// Text 当前元素的所有文本（包括子元素）
func (he HTMLElement) Text() string {
	return he.DOM.Text()
	// return goquery.NewDocumentFromNode(he.Node).Text()
}

// ChildText 在当前元素内部根据选择器查找元素，并返回第一个匹配的目标元素的文本内容
func (he HTMLElement) ChildText(selector string) string {
	return strings.TrimSpace(he.DOM.Find(selector).Text())
}

// ChildrenText 在当前元素内部根据选择器查找元素，并返回全部匹配的目标元素的文本切片
func (he HTMLElement) ChildrenText(selector string) []string {
	var res []string
	he.DOM.Find(selector).Each(func(_ int, s *goquery.Selection) {
		res = append(res, strings.TrimSpace(s.Text()))
	})
	return res
}

// ChildAttr 在当前元素内部根据选择器查找元素，并返回第一个匹配的目标元素的属性值
func (he HTMLElement) ChildAttr(selector, attrName string) string {
	if attr, ok := he.DOM.Find(selector).Attr(attrName); ok {
		return strings.TrimSpace(attr)
	}
	return ""
}

// ChildrenAttr 在当前元素内部根据选择器查找元素，并返回全部匹配的目标元素的属性值切片
func (he HTMLElement) ChildrenAttr(selector, attrName string) []string {
	var res []string
	he.DOM.Find(selector).Each(func(_ int, s *goquery.Selection) {
		if attr, ok := s.Attr(attrName); ok {
			res = append(res, strings.TrimSpace(attr))
		}
	})
	return res
}

// Each 处理每一个与选择器匹配的元素
func (he HTMLElement) Each(selector string, callback func(int, *HTMLElement)) {
	i := 0
	he.DOM.Find(selector).Each(func(_ int, s *goquery.Selection) {
		for _, n := range s.Nodes {
			callback(i, NewHTMLElementFromSelectionNode(s, n, i))
			i++
		}
	})
}

// Child 返回第 num 个匹配选择器的子元素，num 不是索引，而是从 1 开始的顺序号
func (he HTMLElement) Child(selector string, num int) *HTMLElement {
	s := he.DOM.Find(selector)
	nodes := s.Nodes
	if len(nodes) == 0 {
		return nil
	}

	if num == -1 {
		num = s.Length()
	}

	return NewHTMLElementFromSelectionNode(
		goquery.NewDocumentFromNode(nodes[num-1]).Selection,
		nodes[num-1],
		num-1,
	)
}

// FirstChild 返回第一个匹配选择器的子元素
func (he HTMLElement) FirstChild(selector string) *HTMLElement {
	return he.Child(selector, 1)
}

// LastChild 返回最后一个匹配选择器的子元素
func (he HTMLElement) LastChild(selector string) *HTMLElement {
	return he.Child(selector, -1)
}
