/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: mysql.go
 * @Created: 2021-07-24 22:24:29
 * @Modified: 2021-07-30 22:04:45
 */

package dao

import (
	"strings"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type MySQL struct {
    Host,Port,Database,Username,Password string
    DB  *gorm.DB
}

func (m *MySQL) uri() string {
    var s strings.Builder
    s.WriteString(m.Username)
    s.WriteString(":")
    s.WriteString(m.Password)
    s.WriteString("@")
    s.WriteString("tcp(")
    s.WriteString(m.Host)
    s.WriteString(":")
    s.WriteString(m.Port)
    s.WriteString(")/")
    s.WriteString(m.Database)
    s.WriteString("?charset=utf8mb4&parseTime=True&loc=Local")
    return s.String()
}

func (m *MySQL) Init() error {
    uri := m.uri()
    if uri == "" {
        return URIRequiredError
    }
    db, err := gorm.Open(mysql.Open(uri), &gorm.Config{
        PrepareStmt: true,
        Logger:      logger.Default.LogMode(logger.Silent),
    })
    if err != nil {
        return err
    }
    m.DB = db
    return nil
}

func (m *MySQL) AutoMigrate(models ...interface{}) error {
    return m.DB.AutoMigrate(models...)
}

func (m *MySQL) Insert(model interface{}) error {
    return m.DB.Create(model).Error
}

func (m *MySQL) InsertMany(models []interface{}) error {
    return m.DB.Create(models).Error
}

/*****  查询方法太多了，有必要全列出来吗？  *****/

// Select 用主键查询
func (m *MySQL) Select(model interface{}, pk interface{}) error {
    return m.DB.First(model, pk).Error
}

// SelectMany 用多个主键查询
func (m *MySQL) SelectMany(model interface{}, pks []interface{}) error {
    return m.DB.Find(model, pks...).Error
}

// SelectAll 获取表的全部记录
func (m *MySQL) SelectAll(models []interface{}) error {
    return m.DB.Find(models).Error
}

func (m *MySQL) SelectOneWithWhere(model interface{}, where interface{}, args ...interface{}) error {
    return m.DB.Where(where, args...).First(model).Error
}

func (m *MySQL) SelectAllWithWhere(models []interface{}, where interface{}, args ...interface{}) error {
    return m.DB.Where(where, args...).Find(models).Error
}

// Update 更新一列，只更新非零值字段
func (m *MySQL) Update(model interface{}, column string, val interface{}) error {
    return m.DB.Model(model).Update(column, val).Error
}

// Updates 更新多列，只更新非零值字段
func (m *MySQL) Updates(model interface{}, where interface{}) error {
    return m.DB.Model(model).Updates(where).Error
}

// ForcedUpdate 更新所有字段，即使字段是零值，此举会覆盖原记录的所有字段
func (m *MySQL) ForcedUpdate(model interface{}) error {
    return m.DB.Save(model).Error
}

// Delete 根据主键删除
func (m *MySQL) Delete(model interface{}) error {
    return m.DB.Delete(model).Error
}

// DeleteWithWhere 根据条件删除
func (m *MySQL) DeleteWithWhere(model interface{}, where string, args ...interface{}) error {
    return m.DB.Where(where, args...).Delete(model).Error
}

func (m *MySQL) Truncate(model interface{}) error {
    return m.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(model).Error
}
