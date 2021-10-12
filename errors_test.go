/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: errors_test.go
 * @Created: 2021-07-27 11:24:13
 * @Modified: 2021-10-12 09:43:29
 */

package predator

import (
	"fmt"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestProxyInvalid(t *testing.T) {
	ip := "14.135.152.115:45143"
	err := fmt.Errorf("cannot connect to proxy ip [ %s ] -> dial tcp4 14.135.152.115:45143: connect: connection", ip)
	Convey("代理失效", t, func() {
		invalidIP, ok := isProxyInvalid(err)
		So(ok, ShouldBeTrue)
		So(invalidIP, ShouldEqual, ip)
	})
}
