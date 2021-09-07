/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: errors.go
 * @Created: 2021-07-23 09:22:36
 * @Modified: 2021-08-07 21:53:26
 */

package predator

import (
	"errors"
	"regexp"
)

var (
	ErrInvalidProxy    = errors.New("the proxy ip should contain the protocol")
	ErrUnknownProtocol = errors.New("only support http and socks5 protocol")
	ErrProxyExpired    = errors.New("the proxy ip has expired")
	ErrOnlyOneProxyIP  = errors.New("unable to delete the only proxy ip")
	ErrUnkownProxyIP   = errors.New("proxy is unkown")
	ErrEmptyProxyPool  = errors.New("after deleting the invalid proxy, the current proxy ip pool is empty")
	ErrNoCacheSet      = errors.New("no cache set")
)

func isProxyInvalid(err error) (string, bool) {
	if len(err.Error()) < 26 {
		return "", false
	}

	// http proxy expired or invalid
	if err.Error()[:26] == "cannot connect to proxy ip" {
		re := regexp.MustCompile(`cannot connect to proxy ip \[ (.+?) \] -> .+?`)
		return re.FindAllStringSubmatch(err.Error(), 1)[0][1], true
	}

	// socks5 proxy expired or invalid
	if err.Error()[:17] == "socks connect tcp" {
		re := regexp.MustCompile("socks connect tcp (.+?)->.+?")
		return re.FindAllStringSubmatch(err.Error(), 1)[0][1], true
	}

	return "", false
}
