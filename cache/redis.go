/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: redis.go
 * @Created: 2021-07-24 22:21:17
 * @Modified: 2021-10-12 09:45:12
 */

package cache

import (
	"context"
	"encoding/base64"
	"strings"

	"github.com/go-predator/predator/tools"
	"github.com/go-redis/redis/v8"
)

const (
	namespace = "predator-cache"
)

type RedisCache struct {
	Addr, Password string
	DB             int
	client         *redis.Client
	ctx            context.Context
	compressed     bool
}

func cacheKey(key string) string {
	var s strings.Builder
	s.WriteString(namespace)
	s.WriteString(":")
	s.WriteString(key)
	return s.String()
}

func (rc *RedisCache) Compressed(yes bool) {
	rc.compressed = yes
}

func (rc *RedisCache) Init() error {
	rdb := redis.NewClient(&redis.Options{
		Addr:     rc.Addr,
		Password: rc.Password,
		DB:       rc.DB,
	})

	rc.client = rdb
	rc.ctx = context.Background()
	return nil
}

func (rc *RedisCache) IsCached(key string) ([]byte, bool) {
	val, err := rc.client.Get(rc.ctx, cacheKey(key)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, false
		}
		panic(err)
	}
	if val != "" {
		// 将 base64 字符串转换为 []byte
		valBytes, err := base64ToBytes(val)
		if err != nil {
			panic(err)
		}
		if rc.compressed {
			dec, err := tools.Decompress(valBytes)
			if err != nil {
				panic(err)
			}
			return dec, true
		}
		return valBytes, true
	}
	return nil, false
}

func (rc *RedisCache) Cache(key string, val []byte) error {
	// 不存在则缓存
	if !rc.exists(key) {
		if rc.compressed {
			val = tools.Compress(val)
		}
		// 将 []byte 转化为 base64 存储
		valBase64 := bytesToBase64(val)
		return rc.client.Set(rc.ctx, cacheKey(key), valBase64, 0).Err()
	}
	return nil
}

func (rc *RedisCache) exists(key string) bool {
	res, err := rc.client.Exists(rc.ctx, key).Result()
	if err != nil {
		panic(err)
	}
	return res == 1
}

func (rc *RedisCache) scanKeys(cursor uint64) ([]string, uint64, error) {
	return rc.client.Scan(rc.ctx, cursor, cacheKey("*"), 20).Result()
}

func (rc *RedisCache) Clear() error {
	var err error
	var cursor uint64
	for {
		var keys []string
		keys, cursor, err = rc.scanKeys(cursor)
		if err != nil {
			return err
		}

		err = rc.client.Del(rc.ctx, keys...).Err()
		if err != nil {
			return err
		}

		if cursor == 0 {
			break
		}
	}
	return nil
}

func base64ToBytes(src string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(src)
}

func bytesToBase64(src []byte) string {
	return base64.StdEncoding.EncodeToString(src)
}
