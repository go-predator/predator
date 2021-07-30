/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: sqlite.go
 * @Created: 2021-07-24 22:20:47
 * @Modified: 2021-07-30 21:57:51
 */

package cache

import (
	"errors"

	"github.com/thep0y/predator/dao"
	"github.com/thep0y/predator/tools"
	"gorm.io/gorm"
)

type SqliteCache struct {
	URI        string
	db         *dao.Sqlite
	compressed bool
}

func (sc *SqliteCache) Init() error {
	if sc.URI == "" {
		sc.URI = "predator-cache.sqlite"
	}
	sc.db = &dao.Sqlite{
		URI: sc.URI,
	}
	err := sc.db.Init()
	if err != nil {
		return err
	}

	err = sc.db.AutoMigrate(&CacheModel{})
	if err != nil {
		return err
	}
	return nil
}

func (sc *SqliteCache) IsCached(key string) ([]byte, bool) {
	var cache CacheModel
	err := sc.db.SelectOneWithWhere(&cache, "`key` = ?", key)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false
		}
		panic(err)
	}

	if cache.Value != nil {
		if sc.compressed {
			dec, err := tools.Decompress(cache.Value)
			if err != nil {
				panic(err)
			}
			return dec, true
		}
		return cache.Value, true
	}
	return nil, false
}

func (sc *SqliteCache) Cache(key string, val []byte) error {
	// 这里不能用 CheckCache，因为 value 值很长，获取 Value 和解压过程耗时较长
	var count int
	err := sc.db.DB.Model(&CacheModel{}).Select("COUNT(*)").Where("`key` = ?", key).Scan(&count).Error
	if err != nil {
		return err
	}

	if count == 0 {
		if sc.compressed {
			val = tools.Compress(val)
		}
		return sc.db.Insert(&CacheModel{
			Key:   key,
			Value: val,
		})
	}

	return nil
}

func (sc *SqliteCache) Clear() error {
	return sc.db.Truncate(&CacheModel{})
}

func (sc *SqliteCache) Compressed(yes bool) {
	sc.compressed = yes
}
