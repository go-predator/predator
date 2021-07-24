/*
 * @Author: thepoy
 * @Email: email@example.com
 * @File Name: read.go
 * @Created: 2021-07-24 08:56:04
 * @Modified: 2021-07-24 13:18:22
 */

package context

import (
	"sync"
)

type rcontext struct {
	sync.Map
}

func (r *rcontext) GetAny(key string) interface{} {
	val, ok := r.Load(key)
	if ok {
		return val
	}
	return nil
}

func (r *rcontext) Get(key string) string {
	val := r.GetAny(key)
	if val == nil {
		return ""
	}
	return val.(string)
}

func (r *rcontext) Put(key string, val interface{}) {
	r.Store(key, val)
}

func (r *rcontext) GetAndDelete(key string) interface{} {
	val, ok := r.LoadAndDelete(key)
	if ok {
		return val
	}
	return nil
}

func (r *rcontext) Delete(key string) {
	r.GetAndDelete(key)
}

func (r *rcontext) ForEach(f func(key string, val interface{}) interface{}) []interface{} {
	// 因为 sync.Map 不能使用 len 方法计算长度，所以此处创建一个中间变量 temp 用来记录
	temp := make(map[string]interface{})

	r.Range(func(key, value interface{}) bool {
		temp[key.(string)] = value
		return true
	})

	result := make([]interface{}, 0, len(temp))
	for k, v := range temp {
		result = append(result, f(k, v))
	}
	return result
}

func (r *rcontext) Clear() {
	temp := make(map[string]interface{})
	r.Range(func(key, value interface{}) bool {
		temp[key.(string)] = value
		return true
	})

	for k := range temp {
		r.Delete(k)
	}
}

func (r *rcontext) Length() int {
	l := 0
	r.Range(func(key, value interface{}) bool {
		l++
		return true
	})
	return l
}
