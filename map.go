/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   map.go
 * @Created At:  2023-02-20 20:47:10
 * @Modified At: 2023-02-26 09:45:34
 * @Modified By: thepoy
 */

package predator

func ResetMap[V any](m map[string]V) {
	for k := range m {
		delete(m, k)
	}
}
