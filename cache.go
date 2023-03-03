/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   cache.go
 * @Created At:  2021-11-24 20:39:11
 * @Modified At: 2023-03-03 22:18:14
 * @Modified By: thepoy
 */

package predator

import (
	"fmt"
	"net/url"
)

type Cache interface {
	// 是否开启压缩。压缩后能减小数据量，但压缩过程会耗时。
	// 如果原数据长度很长，压缩耗时要比查询耗时低得多，此时开启压缩功能是最佳选择。
	// 但如果原数据长度较短，压缩或不压缩，整体耗时区别不大。
	// 是否开启压缩，需要自行测试抉择。
	Compressed(yes bool)
	// 初始化，用来迁移数据库 / 表，和一些与数据库有关的前期准备工作
	Init() error
	// 当前请求是否已缓存过，如果缓存过，则返回缓存中的响应
	IsCached(key string) ([]byte, bool)
	// 将没有缓存过的请求保存到缓存中
	Cache(key string, val []byte) error
	// 清除全部缓存
	Clear() error
}

type CacheModel struct {
	Key   string `gorm:"primaryKey"`
	Value []byte
}

func (CacheModel) TableName() string {
	return "predator-cache"
}

type cacheFieldType uint8

const (
	// A key or field from URL query parameters
	queryParam cacheFieldType = iota
	// A key or field from request body parameters
	requestBodyParam
)

type CacheField struct {
	code    cacheFieldType
	Field   string
	prepare func(string) string
}

func (cf CacheField) String() string {
	return fmt.Sprintf("%d-%s", cf.code, cf.Field)
}

func addQueryParamCacheField(params url.Values, field CacheField) (string, string, error) {
	if val := params.Get(field.Field); val != "" {
		return field.String(), val, nil
	} else {
		// 如果设置了 cachedFields，但 url 查询参数中却没有某个 field，则报异常退出
		return "", "", fmt.Errorf("there is no such field [%s] in the query parameters: %v", field.Field, params.Encode())
	}
}

func NewQueryParamField(field string) CacheField {
	return CacheField{queryParam, field, nil}
}

func NewQueryParamFieldWithPrepare(field string, prepare func(string) string) CacheField {
	return CacheField{queryParam, field, prepare}
}

func NewRequestBodyParamField(field string) CacheField {
	return CacheField{requestBodyParam, field, nil}
}

func NewRequestBodyParamFieldWithPrepare(field string, prepare func(string) string) CacheField {
	return CacheField{requestBodyParam, field, prepare}
}
