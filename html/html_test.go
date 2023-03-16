/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   html_test.go
 * @Created At:  2021-10-10 14:59:49
 * @Modified At: 2023-03-16 17:56:13
 * @Modified By: thepoy
 */

package html

import (
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"golang.org/x/net/html"
)

var body = []byte(`
<html>
<body>
    <div id="barrierfree_container">
        <div class="top_box">
            <div class="cf top">
                <ul class="fl top_l">
                    <li class="hide fl top_l01"><a href="" target="_blank">login</a></li>
                    <li class="hide fl top_separte"></li>
                    <li class="fl top_l02"><a href="#" target="_blank">email</a></li>
                    <li class="fl top_separte"></li>
                    <li class="fl top_l03"><a href="#" target="_blank">tg</a></li>
                    <li class="fl top_separte"></li>
                    <li id="showImg01" class="fl" style="position:relative;"><img id="showImg02" src="#"></li>
                </ul>
            </div>
        </div>
        <div id="content">
            <p>it is a P
                <span>it is a SPAN
                    <a>it is a A</a>
                </span>
            </p>
            <p>this is a P element with a long length text, and it should work in TestHtmlString</p>
        </div>
    </div>
	<div>
		<p>Some text</p>
		<div>More text</div>
	</div>
	<span>is a block span
		<span>it is a span in the block span</span>
	</span>
	<a>is a block a
		<span>it is a span in the block a</span>
	</a>
</body>
</html>`)

func TestGetParent(t *testing.T) {
	Convey("test to find the parent element", t, func() {
		doc, err := ParseHTML(body)
		So(err, ShouldBeNil)

		imgSelection := doc.Find("#showImg02")
		img := NewHTMLElementFromSelectionNode(imgSelection, imgSelection.Nodes[0], 0)
		So(img.Name, ShouldEqual, "img")
		So(img.Attr("id"), ShouldEqual, "showImg02")

		Convey("find the immediate parent element", func() {
			parent := img.Parent()
			So(parent.Name, ShouldEqual, "li")
			So(parent.Attr("id"), ShouldEqual, "showImg01")
		})

		Convey("find all parent elements", func() {
			parents := img.Parents()
			So(len(parents), ShouldEqual, 7)
			So(parents[len(parents)-1].Name, ShouldEqual, "html")
		})
	})
}

func BlockTexts(n *html.Node) []string {
	var texts []string
	var stack []*html.Node
	stack = append(stack, n)

	for len(stack) > 0 {
		curr := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		switch curr.Type {
		case html.ElementNode:
			if isBlock(curr) {
				for c := curr.FirstChild; c != nil; c = c.NextSibling {
					stack = append(stack, c)
				}
			} else {
				stack = append(stack, curr.FirstChild)
			}
		case html.TextNode:
			text := strings.TrimSpace(curr.Data)
			if text != "" {
				texts = append(texts, text)
			}
		case html.DocumentNode, html.DoctypeNode:
			// Do nothing.
		default:
			// Ignore all other node types.
		}
	}

	return texts
}

func TestHtmlText(t *testing.T) {
	Convey("test to parse texts of a element", t, func() {
		doc, err := ParseHTML(body)
		So(err, ShouldBeNil)

		e := NewHTMLElementFromSelectionNode(doc.Selection, doc.Nodes[0], 0)

		So(e, ShouldNotBeNil)

		excepted := `login
email
tg
it is a P
it is a SPAN
it is a A
this is a P element with a long length text, and it should work in TestHtmlString
Some text
More text
is a block span
it is a span in the block span
is a block a
it is a span in the block a`

		So(strings.Join(e.Texts(), "\n"), ShouldEqual, excepted)
	})

	Convey("test to parse texts whose `p` and `span` are replaced with their texts of a element", t, func() {
		doc, err := ParseHTML(body)
		So(err, ShouldBeNil)

		e := NewHTMLElementFromSelectionNode(doc.Selection, doc.Nodes[0], 0)

		So(e, ShouldNotBeNil)

		excepted := `login
email
tg
it is a Pit is a SPANit is a A
this is a P element with a long length text, and it should work in TestHtmlString
Some text
More text
is a block spanit is a span in the block span
is a block ait is a span in the block a`

		So(strings.Join(e.BlockTexts(), "\n"), ShouldEqual, excepted)
	})
}

func TestHtmlString(t *testing.T) {
	Convey("test to stringnify a element", t, func() {
		doc, err := ParseHTML(body)
		So(err, ShouldBeNil)

		e := NewHTMLElementFromSelectionNode(doc.Selection, doc.Nodes[0], 0)

		So(e, ShouldNotBeNil)

		excepted := `<div class="top_box">...</div>`
		e1 := e.FirstChild(".top_box")
		So(e1.String(), ShouldEqual, excepted)

		excepted = "<p>thi...ing</p>"
		e2 := e.Child("#content p", 2)
		So(e2.String(), ShouldEqual, excepted)

		excepted = `<li class="hide fl top_separte"></li>`
		e3 := e.FirstChild(".top_separte")
		So(e3.String(), ShouldEqual, excepted)

	})
}
