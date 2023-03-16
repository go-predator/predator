/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   map.go
 * @Created At:  2023-02-20 20:47:10
 * @Modified At: 2023-03-16 10:09:15
 * @Modified By: thepoy
 */

package predator

// ResetMap clears all values in the given map.
func ResetMap[V any](m map[string]V) {
	// Iterate over each key in the map.
	for k := range m {
		// Delete the key-value pair from the map.
		delete(m, k)
	}
}
