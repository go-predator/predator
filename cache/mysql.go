/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: mysql.go
 * @Created: 2021-07-24 22:22:52
 * @Modified: 2021-07-30 22:07:09
 */

package cache

import (
    "errors"

    "github.com/thep0y/predator/dao"
    "github.com/thep0y/predator/tools"
    "gorm.io/gorm"
)

type MySQLCache struct {
    Host,Port,Database,Username,Password string
    db         *dao.MySQL
    compressed bool
}

func (mc *MySQLCache) Init() error {
    mc.db = &dao.MySQL{
        Host: mc.Host,
        Port: mc.Port,
        Database: mc.Database,
        Username: mc.Username,
        Password: mc.Password,
    }
    err := mc.db.Init()
    if err != nil {
        return err
    }

    err = mc.db.AutoMigrate(&CacheModel{})
    if err != nil {
        return err
    }
    return nil
}

func (mc *MySQLCache) IsCached(key string) ([]byte, bool) {
    var cache CacheModel
    err := mc.db.SelectOneWithWhere(&cache, "`key` = ?", key)
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, false
        }
        panic(err)
    }

    if cache.Value != nil {
        if mc.compressed {
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

func (mc *MySQLCache) Cache(key string, val []byte) error {
    // 这里不能用 CheckCache，因为 value 值很长，获取 Value 和解压过程耗时较长
    var count int
    err := mc.db.DB.Model(&CacheModel{}).Select("COUNT(*)").Where("`key` = ?", key).Scan(&count).Error
    if err != nil {
        return err
    }

    if count == 0 {
        if mc.compressed {
            val = tools.Compress(val)
        }
        return mc.db.Insert(&CacheModel{
            Key:   key,
            Value: val,
        })
    }

    return nil
}

func (mc *MySQLCache) Clear() error {
    return mc.db.Truncate(&CacheModel{})
}

func (mc *MySQLCache) Compressed(yes bool) {
    mc.compressed = yes
}
