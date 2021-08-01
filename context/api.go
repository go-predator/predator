/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: api.go (c) 2021
 * @Created: 2021-07-24 08:55:30
 * @Modified: 2021-08-01 10:11:23
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
	// String 将上下文转换为 json(非标准) 字符串
	String() string
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

var ctxPool sync.Pool

// AcquireCtx returns an empty Context instance from context pool.
//
// The returned Context instance may be passed to ReleaseCtx when it is
// no longer needed. This allows Context recycling, reduces GC pressure
// and usually improves performance.
func AcquireCtx(ops ...CtxOp) (Context, error) {
	if len(ops) > 1 {
		return nil, fmt.Errorf("only 1 op can be passed in as most, but you passed %d ops", len(ops))
	}
	v := ctxPool.Get()
	if v == nil {
		return NewContext(ops...)
	}
	return v.(Context), nil
}

// ReleaseCtx returns ctx acquired via AcquireCtx to Context pool.
//
// It is forbidden accessing ctx and/or its' members after returning
// it to Context pool.
func ReleaseCtx(ctx Context) {
	ctx.Clear()
	ctxPool.Put(ctx)
}

// NewContext returns a new Context instance
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
