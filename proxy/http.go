/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: http.go
 * @Created: 2021-07-23 09:22:36
 * @Modified: 2021-10-12 09:44:12
 */

package proxy

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
)

func HttpProxy(proxyAddr, addr string, timeout time.Duration) (net.Conn, error) {
	var auth string
	proxyAddr = strings.Split(proxyAddr, "//")[1]
	if strings.Contains(proxyAddr, "@") {
		split := strings.Split(proxyAddr, "@")
		auth = base64.StdEncoding.EncodeToString([]byte(split[0]))
		proxyAddr = split[1]
	}
	var conn net.Conn
	var err error
	if timeout == 0 {
		conn, err = fasthttp.Dial(proxyAddr)
	} else {
		conn, err = fasthttp.DialTimeout(proxyAddr, timeout)
	}
	if err != nil {
		return nil, fmt.Errorf("cannot connect to proxy ip [ %s ] -> %s", proxyAddr, err)
	}

	req := "CONNECT " + addr + " HTTP/1.1\r\n"
	if auth != "" {
		req += "Proxy-Authorization: Basic " + auth + "\r\n"
	}
	req += "\r\n"

	if _, err := conn.Write([]byte(req)); err != nil {
		return nil, err
	}

	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(res)

	res.SkipBody = true

	if err := res.Read(bufio.NewReader(conn)); err != nil {
		conn.Close()
		return nil, err
	}
	if res.Header.StatusCode() != 200 {
		conn.Close()
		return nil, fmt.Errorf("could not connect to proxy: %s status code: %d", proxyAddr, res.Header.StatusCode())
	}
	return conn, nil
}
