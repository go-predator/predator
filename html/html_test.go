/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: html_test.go (c) 2021
 * @Created:  2021-10-10 14:59:49
 * @Modified: 2021-10-10 15:58:39
 */

package html

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

var body = []byte(`<body><div id="barrierfree_container">
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
</body>`)

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
