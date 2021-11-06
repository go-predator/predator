/*
 * @Author:    thepoy
 * @Email:     2021-11-05 12:11:41
 * @File Name: errors.go
 * @Created:   2021-11-05 12:11:41
 * @Modified:  2021-11-06 23:00:05
 */

package proxy

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type ErrCode uint8

const (
	ErrWrongFormatCode ErrCode = iota
	ErrUnknownProtocolCode
	ErrProxyExpiredCode
	ErrOnlyOneProxyIPCode
	ErrUnkownProxyIPCode
	ErrIPOrPortIsNullCode
	ErrEmptyProxyPoolCode
	ErrUnableToConnectCode
)

func (ec ErrCode) String() string {
	return fmt.Sprintf("proxy error [ %d ]", ec)
}

type ProxyErr struct {
	Code ErrCode
	Args map[string]string
	Msg  string
}

func (pe ProxyErr) Error() string {
	var s strings.Builder
	s.WriteString(pe.Code.String())

	if len(pe.Msg) > 0 || len(pe.Args) > 0 {
		s.WriteByte(' ')
	}

	if pe.Msg != "" {
		s.WriteString("err=")
		s.WriteString(pe.Msg)
	}

	keys := make([]string, len(pe.Args))
	for _, k := range pe.Args {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for i, k := range keys {
		if i > 0 {
			s.WriteByte(' ')
		}
		s.WriteString(k)
		s.WriteByte('=')
		s.WriteString(pe.Args[k])
	}

	return s.String()
}

func IsProxyInvalid(err error) (string, bool) {
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
