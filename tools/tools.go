/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: tools.go
 * @Created: 2021-07-23 14:55:04
 * @Modified:  2021-11-07 18:02:57
 */

package tools

import (
	"math/rand"
	"time"
)

// Shuffle 洗牌算法，主要用于在代理池中等概率选择每个代理
func Shuffle(pool []string) []string {
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(pool), func(i, j int) {
		pool[i], pool[j] = pool[j], pool[i]
	})
	return pool
}
