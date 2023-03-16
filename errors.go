/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   errors.go
 * @Created At:  2022-02-17 15:30:54
 * @Modified At: 2023-03-16 09:45:15
 * @Modified By: thepoy
 */

package predator

import (
	"errors"
)

var (
	// ErrRequestFailed is an error indicating that the request has failed.
	ErrRequestFailed = errors.New("request failed")

	// ErrTimeout is an error indicating that the request has timed out, and it is recommended to try a new proxy if you are using a proxy pool.
	ErrTimeout = errors.New("timeout, and it is recommended to try a new proxy if you are using a proxy pool")

	// ErrInvalidCacheTypeCode is an error indicating that the cache type code is invalid.
	ErrInvalidCacheTypeCode = errors.New("invalid cache type code")

	// ErrNotAllowedCacheFieldType is an error indicating that only query parameters are allowed as cached fields in GET requests.
	ErrNotAllowedCacheFieldType = errors.New("only query parameters are allowed as cached fields in `GET` requests")

	// ErrNoCache is an error indicating that no cache has been configured.
	ErrNoCache = errors.New("no cache configured")

	// ErrInvalidResponseStatus is an error indicating that if the HTTP status code is 302, there must be a valid Location field in the response header.
	ErrInvalidResponseStatus = errors.New("if the http status code is `302`, there must be a valid `Location` field in the response header")

	// ErrEmptyProxyPool is an error indicating that the proxy pool is empty.
	ErrEmptyProxyPool = errors.New("the proxy pool is empty")

	// ErrEmptyString is an error indicating that a string can not be empty.
	ErrEmptyString = errors.New("can not be empty")
)
