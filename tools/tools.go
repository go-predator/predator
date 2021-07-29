/*
 * @Author: thepoy
 * @Email: thepoy@163.com
 * @File Name: tools.go
 * @Created: 2021-07-23 14:55:04
 * @Modified: 2021-07-29 14:14:32
 */

package tools

import (
	"math/rand"
	"time"
)

// Shuffle 洗牌算法，主要用于在代理池中等概率选择每个代理
func Shuffle(pool []string) []string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	ret := make([]string, len(pool))
	perm := r.Perm(len(pool))
	for i, randIndex := range perm {
		ret[i] = pool[randIndex]
	}
	return ret
}
