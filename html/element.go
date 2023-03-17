/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   element.go
 * @Created At:  2021-07-27 20:35:31
 * @Modified At: 2023-03-17 11:39:15
 * @Modified By: thepoy
 */

package html

import (
	"errors"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-predator/tools"
	"golang.org/x/net/html"
)

var (
	// ErrNilElement is returned when a nil element is passed to a function
	ErrNilElement = errors.New("the current element is nil")
)

// HTMLElement is a struct representing an HTML element.
type HTMLElement struct {
	// Name is the name of the tag.
	Name string

	// DOM is the goquery parsed DOM object of the page. DOM is relative
	// to the current HTMLElement.
	DOM *goquery.Selection

	// Index stores the position of the current element within
	// all the elements matched by an OnHTML callback.
	Index int

	Node *html.Node
}

// String returns the HTML representation of the HTMLElement.
func (he HTMLElement) String() string {
	var s strings.Builder

	s.WriteByte('<')
	s.WriteString(he.Name)

	for _, attr := range he.Node.Attr {
		s.WriteByte(' ')
		s.WriteString(attr.Key)

		if len(attr.Val) == 0 {
			continue
		}

		s.WriteByte('=')
		s.WriteByte('"')
		s.WriteString(attr.Val)
		s.WriteByte('"')
	}

	s.WriteByte('>')

	if fc := he.Node.FirstChild; fc != nil {
		if fc.Type == html.TextNode {
			text := strings.TrimSpace(fc.Data)
			runes := []rune(text)
			n := len(runes)
			if n == 0 {
				s.WriteString("...")
			} else if n > 10 {
				s.WriteRune(runes[0])
				s.WriteRune(runes[1])
				s.WriteRune(runes[2])
				s.WriteString("...")
				s.WriteRune(runes[n-3])
				s.WriteRune(runes[n-2])
				s.WriteRune(runes[n-1])
			} else {
				for i := 0; i < n; i++ {
					s.WriteRune(runes[i])
				}
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

	// A stack to store the nodes to visit
	stack := []*html.Node{he.Node}

	// A slice to store the texts
	var texts []string

	// Loop until the stack is empty
	for len(stack) > 0 {
		// Pop a node from the stack
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		// If the node is a text node, append its data to the texts slice
		if node.Type == html.TextNode {
			text := strings.TrimSpace(node.Data)
			if text != "" {
				texts = append(texts, text)
			}
			continue
		}

		// If the node is a <br> tag, append a newline character to the texts slice
		if node.Type == html.ElementNode && node.Data == "br" {
			texts = append(texts, "")
			continue
		}

		// Add the node's children to the stack
		for c := node.LastChild; c != nil; c = c.PrevSibling {
			stack = append(stack, c)
		}
	}

	return texts
}

const (
	defaultBlockElements  = "audio canvas embed iframe img math object picture svg video address article aside blockquote details dialog dd div dl dt fieldset figcaption figure footer form h1 h2 h3 h4 h5 h6 header hr li main nav ol p pre section table ul"
	defaultInlineElements = "a abbr acronym audio b bdo big br button canvas cite code dfn em i img input kbd label map object output q samp script select small span strong sub sup textarea time tt var video"
)

var (
	defaultBlockElementSet  = createElementsSet(strings.Fields(defaultBlockElements))
	defaultInlineElementSet = createElementsSet(strings.Fields(defaultInlineElements))
)

func createElementsSet(elements []string) *sync.Map {
	elementSet := &sync.Map{}
	for _, e := range elements {
		elementSet.Store(e, struct{}{})
	}
	return elementSet
}

// BlockTexts returns the texts of all block-level elements in the the current.
//
// If an inline element is found inside a block-level element, its text is also included as part of the block-level element's text.
//
// If the parent of an inline element is not a block-level element, its text is included as a separate element in the slice.
func (h *HTMLElement) BlockTexts() []string {
	if h == nil {
		return nil
	}

	texts := []string{}

	stack := []*html.Node{h.Node}

	// Iterate over the stack until it is empty
	for len(stack) > 0 {
		// Pop the last node from the stack
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		// If the node is a text node, add its text to the current element
		if node.Type == html.TextNode {
			if text := tools.Strip(node.Data); text != "" {
				// Append to the last element of texts
				if len(texts) > 0 && isInlineElement(node.Parent) {
					texts[len(texts)-1] += text
				} else {
					texts = append(texts, text)
				}
			}
			continue
		}

		// If the node is an element node, handle it appropriately
		if node.Type == html.ElementNode {
			texts = handleElementNode(node, texts)
		}

		// Add the node's children to the stack in reverse order
		for c := node.LastChild; c != nil; c = c.PrevSibling {
			stack = append(stack, c)
		}
	}

	// Remove any empty elements from the slice
	for i := len(texts) - 1; i >= 0; i-- {
		texts[i] = tools.Strip(texts[i])
		if texts[i] == "" {
			texts = append(texts[:i], texts[i+1:]...)
		}
	}

	return texts
}

func handleElementNode(node *html.Node, texts []string) []string {
	if isBlockElement(node) || node.Parent == nil || node.Parent.Data == "body" {
		if len(texts) > 0 && isInlineElement(node.Parent) {
			texts[len(texts)-1] = tools.Strip(texts[len(texts)-1])
		}
		texts = append(texts, "")
	} else {
		if node.Data == "br" {
			texts = append(texts, "\n")
		}
	}
	return texts
}

func isBlockElement(n *html.Node, blockElements ...string) bool {
	var blockElementsSet *sync.Map

	if len(blockElements) == 0 {
		blockElementsSet = defaultBlockElementSet
	} else {
		blockElementsSet = createElementsSet(blockElements)
	}

	_, ok := blockElementsSet.Load(n.Data)
	return ok
}

func isInlineElement(n *html.Node, inlineElements ...string) bool {
	var inlineElementSet *sync.Map

	if len(inlineElements) == 0 {
		inlineElementSet = defaultInlineElementSet
	} else {
		inlineElementSet = createElementsSet(inlineElements)
	}

	_, ok := inlineElementSet.Load(n.Data)
	return ok
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
