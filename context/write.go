/*
 * @Author: thepoy
 * @Email: email@example.com
 * @File Name: write.go
 * @Created: 2021-07-24 08:56:16
 * @Modified: 2021-07-24 13:09:17
 */

package context

import (
	"sync"
)

type wcontext struct {
	m map[string]interface{}
	l *sync.RWMutex
}

func (w *wcontext) GetAny(key string) interface{} {
	w.l.RLock()
	defer w.l.RUnlock()
	if v, ok := w.m[key]; ok {
		return v
	}
	return nil
}

func (w *wcontext) Get(key string) string {
	val := w.GetAny(key)
	if val == nil {
		return ""
	}
	return val.(string)
}

func (w *wcontext) Put(key string, val interface{}) {
	w.l.Lock()
	w.m[key] = val
	w.l.Unlock()
}

func (w *wcontext) GetAndDelete(key string) interface{} {
	w.l.Lock()
	defer w.l.Unlock()

	v, ok := w.m[key]
	if !ok {
		return nil
	}

	delete(w.m, key)

	return v
}

func (w *wcontext) Delete(key string) {
	w.GetAndDelete(key)
}

// ForEach 将上下文中的全部 key 和 value 用传入的函数处理后返回一个处理结果的切片
func (w *wcontext) ForEach(f func(key string, val interface{}) interface{}) []interface{} {
	w.l.RLock()
	defer w.l.RUnlock()

	result := make([]interface{}, 0, len(w.m))
	for k, v := range w.m {
		result = append(result, f(k, v))
	}
	return result
}

func (w *wcontext) Clear() {
	w.l.Lock()
	w.m = make(map[string]interface{})
	w.l.Unlock()
}

func (w *wcontext) Length() int {
	w.l.RLock()
	defer w.l.RUnlock()

	return len(w.m)
}
