/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: element.go
 * @Created: 2021-07-27 20:35:31
 * @Modified:  2021-11-14 22:27:31
 */

package html

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
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

// ChildText returns the concatenated and stripped text content of the matching
// elements.
func (he *HTMLElement) ChildText(selector string) string {
	return strings.TrimSpace(he.DOM.Find(selector).Text())
}

// ChildrenText returns the stripped text content of all the matching
// elements.
func (he *HTMLElement) ChildrenText(selector string) []string {
	var res []string
	he.DOM.Find(selector).Each(func(_ int, s *goquery.Selection) {
		res = append(res, strings.TrimSpace(s.Text()))
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
	he.DOM.Find(selector).Each(func(_ int, s *goquery.Selection) {
		if attr, ok := s.Attr(attrName); ok {
			res = append(res, strings.TrimSpace(attr))
		}
	})
	return res
}

// Each iterates over the elements matched by the first argument
// and calls the callback function on every HTMLElement match.
//
// The for loop will break when the `callback` returns `true`.
func (he *HTMLElement) Each(selector string, callback func(int, *HTMLElement) bool) {
	i := 0
	he.DOM.Find(selector).Each(func(_ int, s *goquery.Selection) {
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
		panic("`HTMLElement` instance is nil")
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
