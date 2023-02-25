/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   map.go
 * @Created At:  2023-02-20 20:47:10
 * @Modified At: 2023-02-20 20:59:37
 * @Modified By: thepoy
 */

package predator

func ResetMap[V any](m map[string]V) {
	if m != nil {
		for k := range m {
			delete(m, k)
		}
	}
}
