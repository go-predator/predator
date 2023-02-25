/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   errors.go
 * @Created At:  2022-02-17 15:30:54
 * @Modified At: 2023-02-25 20:19:28
 * @Modified By: thepoy
 */

package predator

import (
	"errors"
)

var (
	ErrRequestFailed            = errors.New("request failed")
	ErrTimeout                  = errors.New("timeout, and it is recommended to try a new proxy if you are using a proxy pool")
	ErrInvalidCacheTypeCode     = errors.New("invalid cache type code")
	ErrNotAllowedCacheFieldType = errors.New("only query parameters are allowed as cached fields in `GET` requests")
	ErrNoCache                  = errors.New("no cache configured")
	ErrInvalidResponseStatus    = errors.New("if the http status code is `302`, there must be a valid `Location` field in the response header")
	ErrEmptyProxyPool           = errors.New("the proxy pool is empty")
)
