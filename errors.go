/*
 * @Author:    thepoy
 * @Email:     thepoy@aliyun.com
 * @File Name: errors.go
 * @Created:   2022-02-17 15:30:54
 * @Modified:  2022-02-17 16:18:17
 */

package predator

import "errors"

var (
	ErrNoCacheSet               = errors.New("no cache set")
	ErrRequestFailed            = errors.New("request failed")
	ErrTimeout                  = errors.New("timeout, and it is recommended to try a new proxy if you are using a proxy pool")
	ErrInvalidCacheTypeCode     = errors.New("invalid cache type code")
	ErrNotAllowedCacheFieldType = errors.New("only query parameters are allowed as cached fields in `GET` requests")
)
