/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: api.go (c) 2021
 * @Created: 2021-07-24 22:20:09
 * @Modified: 2021-07-30 11:28:57
 */

package dao

type Database interface {
	Init() error
	// 数据库自动迁移，用于创建新表
	AutoMigrate(models ...interface{}) error
	// 插入一条记录
	Insert(model interface{}) error
	// 插入多条记录
	InsertMany(models []interface{}) error
	// 根据主键查询一条记录
	Select(model interface{}, pk interface{}) error
	// 根据主键查询多条记录
	SelectMany(model interface{}, pks []interface{}) error
	// 获取表的全部记录
	SelectAll(models []interface{}) error
	// 根据条件获取一条记录
	SelectOneWithWhere(model interface{}, where interface{}, args ...interface{}) error
	// 根据条件获取全部记录
	SelectAllWithWhere(models []interface{}, where interface{}, args ...interface{}) error
	// 更新一列，只更新非零值字段
	Update(model interface{}, column string, val interface{}) error
	// 更新多列，只更新非零值字段
	Updates(model interface{}, where interface{}) error
	// 更新所有字段，即使字段是零值，此举会覆盖原记录的所有字段
	ForcedUpdate(model interface{}) error
	// 根据主键删除
	Delete(model interface{}) error
	// 根据条件删除
	DeleteWithWhere(model interface{}, where string, args ...interface{}) error
	// 清空表
	Truncate(model interface{}) error
}

// 如查需要使用事务，需要用户自己用事务实现 Database 接口
