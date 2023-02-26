/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   write.go
 * @Created At:  2021-07-24 08:56:16
 * @Modified At: 2023-02-26 11:16:18
 * @Modified By: thepoy
 */

package context

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
)

type wcontext struct {
	m map[string]any
	l *sync.RWMutex
}

func (w *wcontext) GetAny(key string) any {
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

func (w *wcontext) Put(key string, val any) {
	w.l.Lock()
	w.m[key] = val
	w.l.Unlock()
}

func (w *wcontext) GetAndDelete(key string) any {
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
func (w *wcontext) ForEach(f func(key string, val any) any) []any {
	w.l.RLock()
	defer w.l.RUnlock()

	result := make([]any, 0, len(w.m))
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

func writeValue(s *strings.Builder, val any) {
	switch v := val.(type) {
	case string:
		s.WriteByte('"')
		s.WriteString(v)
		s.WriteByte('"')
	case fmt.Stringer:
		s.WriteByte('"')
		s.WriteString(v.String())
		s.WriteByte('"')
	case uint, uint8, uint16, uint32, uint64, int, int8, int16, int32, int64:
		fmt.Fprintf(s, "%d", v)
	default:
		s.WriteByte('"')
		s.WriteString(fmt.Sprint(v))
		s.WriteByte('"')
	}
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
		s.WriteString(`": `)
		writeValue(&s, v)
		i++
	}
	s.WriteByte('}')
	return s.String()
}
