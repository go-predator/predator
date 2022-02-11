/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: write.go
 * @Created: 2021-07-24 08:56:16
 * @Modified:  2022-02-11 09:15:46
 */

package context

import (
	"bytes"
	"fmt"
	"strings"
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
	// 不需要释放内存，而是应该复用内存，频繁地申请内存是不必要的
	for k := range w.m {
		delete(w.m, k)
	}
	w.l.Unlock()
}

func (w *wcontext) Length() int {
	w.l.RLock()
	defer w.l.RUnlock()

	return len(w.m)
}

func (w *wcontext) Bytes() []byte {
	w.l.RLock()
	defer w.l.RUnlock()

	var b bytes.Buffer
	b.WriteByte('{')
	i := 0
	for k, v := range w.m {
		if i > 0 {
			b.WriteString(`, `)
		}
		b.WriteByte('"')
		b.WriteString(k)
		b.WriteString(`": "`)
		b.WriteString(fmt.Sprint(v))
		b.WriteByte('"')
		i++
	}
	b.WriteByte('}')
	return b.Bytes()
}

func (w *wcontext) String() string {
	w.l.RLock()
	defer w.l.RUnlock()

	var s strings.Builder
	s.WriteByte('{')
	i := 0
	for k, v := range w.m {
		if i > 0 {
			s.WriteString(`, `)
		}
		s.WriteByte('"')
		s.WriteString(k)
		s.WriteString(`": "`)
		s.WriteString(fmt.Sprint(v))
		s.WriteByte('"')
		i++
	}
	s.WriteByte('}')
	return s.String()
}
