/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: api.go
 * @Created: 2021-07-24 22:19:44
 * @Modified: 2021-07-31 09:26:34
 */

package cache

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
	return "cache"
}
