/*
 * @Author: thepoy
 * @Email: email@example.com
 * @File Name: api.go
 * @Created: 2021-07-24 08:55:30
 * @Modified: 2021-07-24 13:23:05
 */

package context

import (
	"fmt"
	"sync"
)

type Context interface {
	// Get 通过 key 在上下文中获取一个字符串
	Get(key string) string
	// GetAny 通过 key 在上下文中获取一个任意类型
	GetAny(key string) interface{}
	// Put 向上下文中传入一个 key: value
	Put(key string, val interface{})
	// GetAndDelete 获取并删除一个 key
	GetAndDelete(key string) interface{}
	// Delete 在上下文中删除指定的 key
	Delete(key string)
	// ForEach 将上下文中的全部 key 和 value 用传
	// 入的函数处理后返回一个处理结果的切片
	ForEach(func(key string, val interface{}) interface{}) []interface{}
	// Clear 清空一个上下文
	Clear()
	// Length 返回上下文的长度
	Length() int
}

// 上下文类型
type CtxOp int

const (
	// 以读为主的上下文，
	// 适用于读操作远多于写的场景
	ReadOp CtxOp = iota
	// 适用于读写各半或写多于读的场景
	WriteOp
)

// NewContext 返回一个上下文实例
func NewContext(ops ...CtxOp) (Context, error) {
	if len(ops) > 1 {
		return nil, fmt.Errorf("only 1 op can be passed in as most, but you passed %d ops", len(ops))
	}

	var op CtxOp
	if len(ops) == 0 {
		op = WriteOp
	} else {
		op = ops[0]
	}

	switch op {
	case ReadOp:
		return &rcontext{}, nil
	case WriteOp:
		return &wcontext{
			m: make(map[string]interface{}),
			l: &sync.RWMutex{},
		}, nil
	default:
		return nil, fmt.Errorf("unkown op: %d", op)
	}
}
