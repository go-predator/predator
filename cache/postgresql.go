/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: postgresql.go
 * @Created: 2021-07-24 22:23:09
 * @Modified: 2021-10-12 09:45:08
 */

package cache

import (
	"errors"

	"github.com/go-predator/predator/dao"
	"github.com/go-predator/predator/tools"
	"gorm.io/gorm"
)

type PostgreSQLCache struct {
	Host, Port, Database, Username, Password, SSLMode, TimeZone string
	db                                                          *dao.PostgreSQL
	compressed                                                  bool
}

func (pc *PostgreSQLCache) Init() error {
	pc.db = &dao.PostgreSQL{
		Host:     pc.Host,
		Port:     pc.Port,
		Database: pc.Database,
		Username: pc.Username,
		Password: pc.Password,
		SSLMode:  pc.SSLMode,
		TimeZone: pc.TimeZone,
	}
	err := pc.db.Init()
	if err != nil {
		return err
	}

	err = pc.db.AutoMigrate(&CacheModel{})
	if err != nil {
		return err
	}
	return nil
}

func (pc *PostgreSQLCache) IsCached(key string) ([]byte, bool) {
	var cache CacheModel
	err := pc.db.SelectOneWithWhere(&cache, `"key" = ?`, key)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false
		}
		panic(err)
	}

	if cache.Value != nil {
		if pc.compressed {
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

func (pc *PostgreSQLCache) Cache(key string, val []byte) error {
	// 这里不能用 CheckCache，因为 value 值很长，获取 Value 和解压过程耗时较长
	var count int
	err := pc.db.DB.Model(&CacheModel{}).Select("COUNT(*)").Where(`"key" = ?`, key).Scan(&count).Error
	if err != nil {
		return err
	}

	if count == 0 {
		if pc.compressed {
			val = tools.Compress(val)
		}
		return pc.db.Insert(&CacheModel{
			Key:   key,
			Value: val,
		})
	}

	return nil
}

func (pc *PostgreSQLCache) Clear() error {
	return pc.db.Truncate(&CacheModel{})
}

func (pc *PostgreSQLCache) Compressed(yes bool) {
	pc.compressed = yes
}
