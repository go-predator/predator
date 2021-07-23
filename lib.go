/*
 * @Author: thepoy
 * @Email: email@example.com
 * @File Name: lib.go
 * @Created: 2021-07-23 14:55:04
 * @Modified: 2021-07-23 14:56:58
 */

package predator

import (
	"math/rand"
	"time"
)

func Shuffle(pool []string) []string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	ret := make([]string, len(pool))
	perm := r.Perm(len(pool))
	for i, randIndex := range perm {
		ret[i] = pool[randIndex]
	}
	return ret
}
