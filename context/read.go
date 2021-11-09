/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: read.go
 * @Created: 2021-07-24 08:56:04
 * @Modified:  2021-11-09 10:33:38
 */

package context

import (
	"bytes"
	"fmt"
	"strings"
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
	r.Map.Delete(key)
}

func (r *rcontext) ForEach(f func(key string, val interface{}) interface{}) []interface{} {
	result := make([]interface{}, 0, r.Length())
	r.Range(func(key, value interface{}) bool {
		result = append(result, f(key.(string), value))
		return true
	})
	return result
}

func (r *rcontext) Clear() {
	r.Range(func(key, value interface{}) bool {
		r.Map.Delete(key)
		return true
	})
}

func (r *rcontext) Length() int {
	l := 0
	r.Range(func(key, value interface{}) bool {
		l++
		return true
	})
	return l
}

func (r *rcontext) Bytes() []byte {
	var b bytes.Buffer
	b.WriteByte('{')
	i := 0
	r.Range(func(key, value interface{}) bool {
		if i > 0 {
			b.WriteString(`, `)
		}
		b.WriteByte('"')
		b.WriteString(key.(string))
		b.WriteString(`": "`)
		b.WriteString(fmt.Sprint(value))
		b.WriteByte('"')
		i++
		return true
	})
	b.WriteByte('}')
	return b.Bytes()
}

func (r *rcontext) String() string {
	var s strings.Builder
	s.WriteByte('{')
	i := 0
	r.Range(func(key, value interface{}) bool {
		if i > 0 {
			s.WriteString(`, `)
		}
		s.WriteByte('"')
		s.WriteString(key.(string))
		s.WriteString(`": "`)
		s.WriteString(fmt.Sprint(value))
		s.WriteByte('"')
		i++
		return true
	})
	s.WriteByte('}')
	return strings.ReplaceAll(s.String(), ", }", "}")
}
