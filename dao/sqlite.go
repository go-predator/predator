/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: sqlite.go
 * @Created: 2021-07-24 22:24:15
 * @Modified: 2021-07-30 21:58:08
 */

package dao

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Sqlite struct {
	URI string
	DB  *gorm.DB
}

func (s *Sqlite) Init() error {
	db, err := gorm.Open(sqlite.Open(s.URI), &gorm.Config{
		PrepareStmt: true,
		Logger:      logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return err
	}
	s.DB = db
	return nil
}

func (s *Sqlite) AutoMigrate(models ...interface{}) error {
	return s.DB.AutoMigrate(models...)
}

func (s *Sqlite) Insert(model interface{}) error {
	return s.DB.Create(model).Error
}

func (s *Sqlite) InsertMany(models []interface{}) error {
	return s.DB.Create(models).Error
}

/*****  查询方法太多了，有必要全列出来吗？  *****/

// Select 用主键查询
func (s *Sqlite) Select(model interface{}, pk interface{}) error {
	return s.DB.First(model, pk).Error
}

// SelectMany 用多个主键查询
func (s *Sqlite) SelectMany(model interface{}, pks []interface{}) error {
	return s.DB.Find(model, pks...).Error
}

// SelectAll 获取表的全部记录
func (s *Sqlite) SelectAll(models []interface{}) error {
	return s.DB.Find(models).Error
}

func (s *Sqlite) SelectOneWithWhere(model interface{}, where interface{}, args ...interface{}) error {
	return s.DB.Where(where, args...).First(model).Error
}

func (s *Sqlite) SelectAllWithWhere(models []interface{}, where interface{}, args ...interface{}) error {
	return s.DB.Where(where, args...).Find(models).Error
}

// Update 更新一列，只更新非零值字段
func (s *Sqlite) Update(model interface{}, column string, val interface{}) error {
	return s.DB.Model(model).Update(column, val).Error
}

// Updates 更新多列，只更新非零值字段
func (s *Sqlite) Updates(model interface{}, where interface{}) error {
	return s.DB.Model(model).Updates(where).Error
}

// ForcedUpdate 更新所有字段，即使字段是零值，此举会覆盖原记录的所有字段
func (s *Sqlite) ForcedUpdate(model interface{}) error {
	return s.DB.Save(model).Error
}

// Delete 根据主键删除
func (s *Sqlite) Delete(model interface{}) error {
	return s.DB.Delete(model).Error
}

// DeleteWithWhere 根据条件删除
func (s *Sqlite) DeleteWithWhere(model interface{}, where string, args ...interface{}) error {
	return s.DB.Where(where, args...).Delete(model).Error
}

func (s *Sqlite) Truncate(model interface{}) error {
	return s.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(model).Error
}
