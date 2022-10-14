/*
 * @Author:    thepoy
 * @Email:     thepoy@163.com
 * @File Name: element.go
 * @Created:   2021-07-27 20:35:31
 * @Modified:  2022-05-26 20:23:43
 */

package html

import (
	"errors"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-predator/tools"
	"golang.org/x/net/html"
)

var (
	ErrNilElement = errors.New("the current element is nil")
)

// HTMLElement is the representation of a HTML tag.
type HTMLElement struct {
	// Name is the name of the tag
	Name string

	// DOM is the goquery parsed DOM object of the page. DOM is relative
	// to the current HTMLElement
	DOM *goquery.Selection

	// Index stores the position of the current element within
	// all the elements matched by an OnHTML callback
	Index int

	Node *html.Node
}

func (he HTMLElement) String() string {
	var s strings.Builder

	s.WriteByte('<')
	s.WriteString(he.Name)

	for _, attr := range he.Node.Attr {
		s.WriteByte(' ')
		s.WriteString(attr.Key)

		if len(attr.Val) > 0 {
			s.WriteByte('=')
			s.WriteByte('"')
			s.WriteString(attr.Val)
			s.WriteByte('"')
		}
	}

	s.WriteByte('>')

	if fc := he.Node.FirstChild; fc != nil {
		if fc.Type == html.TextNode {
			text := strings.TrimSpace(fc.Data)
			runes := []rune(text)
			if len(runes) == 0 {
				s.WriteString("...")
			} else if len(runes) > 10 {
				s.WriteString(string(runes[:10]))
				s.WriteString("...")
			} else {
				s.WriteString(text)
			}
		} else {
			s.WriteString("...")
		}
	}

	s.WriteString("</")
	s.WriteString(he.Name)
	s.WriteByte('>')

	return s.String()
}

// NewHTMLElementFromSelectionNode creates a HTMLElement from a goquery.Selection Node.
func NewHTMLElementFromSelectionNode(s *goquery.Selection, n *html.Node, index int) *HTMLElement {
	return &HTMLElement{
		Name:  n.Data,
		DOM:   s,
		Index: index,
		Node:  n,
	}
}

// Attr returns the selected attribute of a HTMLElement or empty string
// if no attribute found
func (he *HTMLElement) Attr(key string) string {
	for _, attr := range he.Node.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

// OuterHtml returns the outer HTML rendering of the first item in
// the selection - that is, the HTML including the first element's
// tag and attributes.
func (he *HTMLElement) OuterHTML() (string, error) {
	return goquery.OuterHtml(he.DOM)
}

// InnerHTML gets the HTML contents of the first element in the set of matched
// elements. It includes text and comment nodes.
func (he *HTMLElement) InnerHTML() (string, error) {
	return he.DOM.Html()
}

// Text gets the combined text contents of each element in the set of matched
// elements, including their descendants.
func (he *HTMLElement) Text() string {
	return he.DOM.Text()
}

// Texts Gets all child text elements in the current element and returns a []string
func (he *HTMLElement) Texts() []string {
	if he == nil {
		return nil
	}

	var texts []string

	// Slightly optimized vs calling Each: no single selection object created
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.TextNode {
			text := tools.Strip(n.Data)
			if text != "" {
				// 当使用 Selection.ReplaceWithHtml 将原节点替换成了一个 TextNode 时
				// 很可能会出现多个文本节点连接，这在现实 DOM 结构是不可能存在的，但 ReplaceWithHtml
				// 方法的不完备却可能出现此情况，故只能在此判断前面的节点是否为文本节点，如果是则将两个文本
				// 节点的文本合并。
				if n.PrevSibling != nil && n.PrevSibling.Type == html.TextNode {
					if len(texts) > 0 {
						texts[len(texts)-1] += text
					} else {
						texts = append(texts, text)
					}
				} else {
					texts = append(texts, text)
				}
			}
		}
		if n.FirstChild != nil {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				f(c)
			}
		}
	}
	for _, n := range he.DOM.Nodes {
		f(n)
	}

	return texts
}

// ChildText returns the concatenated and stripped text content of the matching
// elements.
func (he *HTMLElement) ChildText(selector string) string {
	return strings.TrimSpace(he.DOM.Find(selector).Text())
}

// ChildrenText returns the stripped text content of all the matching
// elements.
func (he *HTMLElement) ChildrenText(selector string) []string {
	var res []string
	he.Each(selector, func(_ int, h *HTMLElement) bool {
		text := h.Text()
		if text == "" {
			return false
		}

		res = append(res, strings.TrimSpace(text))
		return false
	})
	return res
}

// ChildAttr returns the stripped text content of the first matching
// element's attribute.
func (he *HTMLElement) ChildAttr(selector, attrName string) string {
	if attr, ok := he.DOM.Find(selector).Attr(attrName); ok {
		return strings.TrimSpace(attr)
	}
	return ""
}

// ChildrenAttr returns the stripped text content of all the matching
// element's attributes.
func (he *HTMLElement) ChildrenAttr(selector, attrName string) []string {
	var res []string
	he.Each(selector, func(_ int, h *HTMLElement) bool {
		if attr := h.Attr(attrName); attr != "" {
			res = append(res, strings.TrimSpace(attr))
		}
		return false
	})
	return res
}

// Each iterates over the elements matched by the first argument
// and calls the callback function on every HTMLElement match.
//
// The for loop will break when the `callback` returns `true`.
func (he *HTMLElement) Each(selector string, callback func(int, *HTMLElement) bool) {
	i := 0
	if he == nil {
		panic(ErrNilElement)
	}

	found := he.DOM.Find(selector)
	if found == nil {
		return
	}

	found.Each(func(_ int, s *goquery.Selection) {
		for _, n := range s.Nodes {
			if callback(i, NewHTMLElementFromSelectionNode(s, n, i)) {
				break
			}
			i++
		}
	})
}

// Child returns the numth matched child element.
// num starts at 1, not at 0.
func (he *HTMLElement) Child(selector string, num int) *HTMLElement {
	if he == nil {
		panic(ErrNilElement)
	}

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

// FirstChild returns the first child element that matches the selector.
func (he *HTMLElement) FirstChild(selector string) *HTMLElement {
	return he.Child(selector, 1)
}

// LastChild returns the last child element that matches the selector.
func (he *HTMLElement) LastChild(selector string) *HTMLElement {
	return he.Child(selector, -1)
}

// Parent returns the direct parent element.
func (he *HTMLElement) Parent() *HTMLElement {
	// If the current element is <html> tag, return nil
	if he.Name == "html" {
		return nil
	}

	s := he.DOM.Parent()
	return NewHTMLElementFromSelectionNode(s, s.Nodes[0], 0)
}

// Parents returns all parent elements.
func (he *HTMLElement) Parents() []*HTMLElement {
	parents := make([]*HTMLElement, 0)

	for {
		var parent = he.Parent()
		if parent == nil {
			break
		}
		parents = append(parents, parent)
		he = parent
	}

	return parents
}

// FindChildByText returns the first child element matching the target text.
func (he *HTMLElement) FindChildByText(selector, text string) *HTMLElement {
	var target *HTMLElement
	he.Each(selector, func(i int, h *HTMLElement) bool {
		if h.Node.FirstChild != nil && h.Node.FirstChild.Type == html.TextNode && h.Node.FirstChild.Data == text {
			target = h
			return true
		}
		return false
	})
	return target
}

// FindChildByStripedText returns the first child element matching the stripped text.
func (he *HTMLElement) FindChildByStripedText(selector, text string) *HTMLElement {
	var target *HTMLElement
	he.Each(selector, func(i int, h *HTMLElement) bool {
		if h.Node.FirstChild != nil && h.Node.FirstChild.Type == html.TextNode && tools.Strip(h.Node.FirstChild.Data) == text {
			target = h
			return true
		}
		return false
	})
	return target
}

// Children returns all child elements matching the selector
func (he *HTMLElement) Children(selector string) []*HTMLElement {
	children := make([]*HTMLElement, 0, 3)
	he.Each(selector, func(i int, h *HTMLElement) bool {
		children = append(children, h)
		return false
	})
	return children
}

// FindChildrenByText returns all the child elements matching the target text.
func (he *HTMLElement) FindChildrenByText(selector, text string) []*HTMLElement {
	targets := make([]*HTMLElement, 0, 3)
	he.Each(selector, func(i int, h *HTMLElement) bool {
		if h.Node.FirstChild != nil && h.Node.FirstChild.Type == html.TextNode && h.Node.FirstChild.Data == text {
			targets = append(targets, h)
		}
		return false
	})
	return targets
}

// FindChildrenByStripedText returns all the child elements matching the stripped text.
func (he *HTMLElement) FindChildrenByStripedText(selector, text string) []*HTMLElement {
	targets := make([]*HTMLElement, 0, 3)
	he.Each(selector, func(i int, h *HTMLElement) bool {
		if h.Node.FirstChild != nil && h.Node.FirstChild.Type == html.TextNode && tools.Strip(h.Node.FirstChild.Data) == text {
			targets = append(targets, h)
		}
		return false
	})
	return targets
}
