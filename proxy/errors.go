/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   errors.go
 * @Created At:  2023-02-25 19:55:12
 * @Modified At: 2023-02-28 14:49:55
 * @Modified By: thepoy
 */

package proxy

import (
	"errors"
	"fmt"
	"strings"
)

type ErrCode uint8

var (
	ErrEmptyProxy         = errors.New("proxy cannot be empty")
	ErrUnreachable        = errors.New("destination unreachable")
	ErrInvalidProxy       = errors.New("ip and port cannot be empty")
	ErrUnkownProxyIP      = errors.New("unkown proxy address")
	ErrUnexpectedProtocol = errors.New("unexpected protocol")
)

func (ec ErrCode) String() string {
	return fmt.Sprintf("proxy error [ %d ]", ec)
}

type ProxyErr struct {
	Proxy string
	Err   error
}

func (pe ProxyErr) Error() string {
	var s strings.Builder

	if pe.Err == nil {
		panic("err cannot be nil")
	}

	if pe.Proxy != "" {
		s.WriteString("proxy")
		s.WriteByte('=')
		s.WriteString(pe.Proxy)
		s.WriteString(", ")
	}

	s.WriteString("error")
	s.WriteByte('=')
	s.WriteString(pe.Err.Error())

	return s.String()
}

func IsProxyError(err error) (string, bool) {
	var pe ProxyErr
	ok := errors.As(err, &pe)
	if !ok {
		return "", false
	}

	return pe.Proxy, true
}

type protocol string

const (
	HTTP   protocol = "http"
	HTTPS  protocol = "https"
	SOCKS5 protocol = "socks5"
)

func UnexpectedProtocol(proxy string, wantProtocol protocol) ProxyErr {
	msg := "proxy does not support %s protocol, it probably supports %s or %s protocols"

	switch wantProtocol {
	case HTTP:
		return ProxyErr{
			Proxy: proxy,
			Err:   fmt.Errorf(msg, HTTP, HTTPS, SOCKS5),
		}
	case HTTPS:
		return ProxyErr{
			Proxy: proxy,
			Err:   fmt.Errorf(msg, HTTPS, HTTP, SOCKS5),
		}
	default:
		return ProxyErr{
			Proxy: proxy,
			Err:   fmt.Errorf(msg, SOCKS5, HTTP, HTTPS),
		}
	}
}
