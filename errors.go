/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: errors.go
 * @Created: 2021-07-23 09:22:36
 * @Modified: 2021-07-31 09:19:56
 */

package predator

import (
	"errors"
	"regexp"
)

var (
	InvalidProxyError    = errors.New("the proxy ip should contain the protocol")
	UnknownProtocolError = errors.New("only support http and socks5 protocol")
	ProxyExpiredError    = errors.New("the proxy ip has expired")
	OnlyOneProxyIPError  = errors.New("unable to delete the only proxy ip")
	CustomProxyIPError   = errors.New("custom proxy ip is invalid")
	EmptyProxyPoolError  = errors.New("after deleting the invalid proxy, the current proxy ip pool is empty")
	NoCacheSet           = errors.New("No cache set")
)

func isProxyInvalid(err error) (string, bool) {
	if err.Error()[:26] == "cannot connect to proxy ip" {
		re := regexp.MustCompile(`cannot connect to proxy ip \[ (.+?) \] -> .+?`)
		return re.FindAllStringSubmatch(err.Error(), 1)[0][1], true
	}

	return "", false
}
