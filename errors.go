/*
 * @Author:    thepoy
 * @Email:     thepoy@163.com
 * @File Name: errors.go
 * @Created:   2022-02-17 15:30:54
 * @Modified:  2022-04-22 08:57:46
 */

package predator

import (
	"fmt"
)

type PredatorError uint8

func (pe PredatorError) Error() string {
	return fmt.Sprintf("PredatorError %d: %s", pe, errorMsgs[pe])
}

func (pe PredatorError) Is(e error) bool {
	target, ok := e.(PredatorError)
	if !ok {
		return false
	}

	return target == pe
}

const (
	ErrRequestFailed PredatorError = iota
	ErrTimeout
	ErrInvalidCacheTypeCode
	ErrNotAllowedCacheFieldType
	ErrNoCache
)

var errorMsgs = []string{
	ErrRequestFailed:            "request failed",
	ErrTimeout:                  "timeout, and it is recommended to try a new proxy if you are using a proxy pool",
	ErrInvalidCacheTypeCode:     "invalid cache type code",
	ErrNotAllowedCacheFieldType: "only query parameters are allowed as cached fields in `GET` requests",
	ErrNoCache:                  "no cache configured",
}
