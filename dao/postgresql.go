/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: postgresql.go
 * @Created: 2021-07-24 22:25:04
 * @Modified: 2021-07-30 22:45:20
 */

package dao

import (
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type PostgreSQL struct {
	Host, Port, Database, Username, Password, SSLMode, TimeZone string
	DB                                                          *gorm.DB
}

func (p *PostgreSQL) uri() string {
	if p.SSLMode == "" {
		p.SSLMode = "disable"
	}

	if p.TimeZone == "" {
		p.TimeZone = "Asia/Shanghai"
	}

	var s strings.Builder
	s.WriteString("host=")
	s.WriteString(p.Host)
	s.WriteString(" user=")
	s.WriteString(p.Username)
	s.WriteString(" password=")
	s.WriteString(p.Password)
	s.WriteString(" dbname=")
	s.WriteString(p.Database)
	s.WriteString(" port=")
	s.WriteString(p.Port)
	s.WriteString(" sslmode=")
	s.WriteString(p.SSLMode)
	s.WriteString(" TimeZone=")
	s.WriteString(p.TimeZone)
	return s.String()
}

func (p *PostgreSQL) Init() error {
	uri := p.uri()
	if uri == "" {
		return URIRequiredError
	}
	db, err := gorm.Open(postgres.Open(uri), &gorm.Config{
		PrepareStmt: true,
		Logger:      logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return err
	}
	p.DB = db
	return nil
}

func (p *PostgreSQL) AutoMigrate(models ...interface{}) error {
	return p.DB.AutoMigrate(models...)
}

func (p *PostgreSQL) Insert(model interface{}) error {
	return p.DB.Create(model).Error
}

func (p *PostgreSQL) InsertMany(models []interface{}) error {
	return p.DB.Create(models).Error
}

/*****  查询方法太多了，有必要全列出来吗？  *****/

// Select 用主键查询
func (p *PostgreSQL) Select(model interface{}, pk interface{}) error {
	return p.DB.First(model, pk).Error
}

// SelectMany 用多个主键查询
func (p *PostgreSQL) SelectMany(model interface{}, pks []interface{}) error {
	return p.DB.Find(model, pks...).Error
}

// SelectAll 获取表的全部记录
func (p *PostgreSQL) SelectAll(models []interface{}) error {
	return p.DB.Find(models).Error
}

func (p *PostgreSQL) SelectOneWithWhere(model interface{}, where interface{}, args ...interface{}) error {
	return p.DB.Where(where, args...).First(model).Error
}

func (p *PostgreSQL) SelectAllWithWhere(models []interface{}, where interface{}, args ...interface{}) error {
	return p.DB.Where(where, args...).Find(models).Error
}

// Update 更新一列，只更新非零值字段
func (p *PostgreSQL) Update(model interface{}, column string, val interface{}) error {
	return p.DB.Model(model).Update(column, val).Error
}

// Updates 更新多列，只更新非零值字段
func (p *PostgreSQL) Updates(model interface{}, where interface{}) error {
	return p.DB.Model(model).Updates(where).Error
}

// ForcedUpdate 更新所有字段，即使字段是零值，此举会覆盖原记录的所有字段
func (p *PostgreSQL) ForcedUpdate(model interface{}) error {
	return p.DB.Save(model).Error
}

// Delete 根据主键删除
func (p *PostgreSQL) Delete(model interface{}) error {
	return p.DB.Delete(model).Error
}

// DeleteWithWhere 根据条件删除
func (p *PostgreSQL) DeleteWithWhere(model interface{}, where string, args ...interface{}) error {
	return p.DB.Where(where, args...).Delete(model).Error
}

func (p *PostgreSQL) Truncate(model interface{}) error {
	return p.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(model).Error
}
