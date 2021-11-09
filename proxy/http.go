/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: http.go
 * @Created: 2021-07-23 09:22:36
 * @Modified:  2021-11-08 23:14:24
 */

package proxy

import (
	"bufio"
	"encoding/base64"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
)

func HttpProxyDialerWithTimeout(proxyAddr string, timeout time.Duration) fasthttp.DialFunc {
	var auth string
	pAddr := strings.Split(proxyAddr, "//")[1]
	if strings.Contains(pAddr, "@") {
		split := strings.Split(pAddr, "@")
		auth = base64.StdEncoding.EncodeToString([]byte(split[0]))
		pAddr = split[1]
	}

	return func(addr string) (net.Conn, error) {
		var conn net.Conn
		var err error
		if timeout == 0 {
			conn, err = fasthttp.Dial(pAddr)
		} else {
			conn, err = fasthttp.DialTimeout(pAddr, timeout)
		}
		if err != nil {
			return nil, ProxyErr{
				Code: ErrUnableToConnectCode,
				Args: map[string]string{
					"proxy": pAddr,
					"error": err.Error(),
				},
				Msg: "cannot connect to proxy",
			}
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
			return nil, ProxyErr{
				Code: ErrUnableToConnectCode,
				Args: map[string]string{
					"proxy":       pAddr,
					"status_code": strconv.Itoa(res.Header.StatusCode()),
				},
				Msg: "cannot connect to proxy",
			}
		}
		return conn, nil
	}
}
