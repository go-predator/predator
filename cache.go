/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   cache.go
 * @Created At:  2021-11-24 20:39:11
 * @Modified At: 2023-03-16 09:39:30
 * @Modified By: thepoy
 */

package predator

import (
	"fmt"
	"net/url"
)

type Cache interface {
	// Compressed specifies whether compression should be enabled. Compression reduces the data size, but adds processing overhead.
	// If the original data is long, enabling compression can be the best choice because the compression overhead is lower than the query overhead.
	// However, if the original data is short, there may be little difference in overall processing time between compressed and uncompressed data.
	// Whether to enable compression should be determined through testing.
	Compressed(yes bool)

	// Init is used to migrate databases/tables and perform some database-related preparations before caching.
	Init() error

	// IsCached checks whether the current request has been cached, and returns the cached response if it exists.
	IsCached(key string) ([]byte, bool)

	// Cache saves the uncached request to the cache.
	Cache(key string, val []byte) error

	// Clear clears all cache entries.
	Clear() error
}

// CacheModel represents the cache model which maps to the database table.
type CacheModel struct {
	Key   string `gorm:"primaryKey"`
	Value []byte
}

// TableName specifies the table name for the CacheModel in the database.
func (CacheModel) TableName() string {
	return "predator-cache"
}

// cacheFieldType is an enum type that defines the possible types of cache fields
type cacheFieldType uint8

const (
	// queryParam represents a key or field from URL query parameters
	queryParam cacheFieldType = iota
	// requestBodyParam represents a key or field from request body parameters
	requestBodyParam
)

// CacheField is a struct that holds the information of a cache field,
// including its type, name and an optional prepare function to modify its value before caching
type CacheField struct {
	code    cacheFieldType
	Field   string
	prepare func(val string) string
}

func (cf CacheField) String() string {
	return fmt.Sprintf("%d-%s", cf.code, cf.Field)
}

// addQueryParamCacheField is a helper function that adds a cache field to the cache key based on a query parameter.
// It returns the cache field name and value, or an error if the query parameter is not found in the URL parameters.
func addQueryParamCacheField(params url.Values, field CacheField) (string, string, error) {
	if val := params.Get(field.Field); val != "" {
		return field.String(), val, nil
	} else {
		return "", "", fmt.Errorf("there is no such field [%s] in the query parameters: %v", field.Field, params.Encode())
	}
}

// NewQueryParamField creates a new CacheField instance for a query parameter.
func NewQueryParamField(field string) CacheField {
	return CacheField{queryParam, field, nil}
}

// NewQueryParamFieldWithPrepare creates a new CacheField instance for a query parameter with a prepare function.
// The prepare function is used to transform the parameter value before it is included in the cache key.
func NewQueryParamFieldWithPrepare(field string, prepare func(string) string) CacheField {
	return CacheField{queryParam, field, prepare}
}

// NewRequestBodyParamField creates a new CacheField instance for a request body parameter.
func NewRequestBodyParamField(field string) CacheField {
	return CacheField{requestBodyParam, field, nil}
}

// NewRequestBodyParamFieldWithPrepare creates a new CacheField instance for a request body parameter with a prepare function.
// The prepare function is used to transform the parameter value before it is included in the cache key.
func NewRequestBodyParamFieldWithPrepare(field string, prepare func(string) string) CacheField {
	return CacheField{requestBodyParam, field, prepare}
}
