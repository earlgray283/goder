package main

import (
	"fmt"
	"strconv"
)

// copy and translated from "github.com/samber/lo/map.go:21"
func Map[T any, R any](collection []T, iteratee func(T, int) R) []R {
	result := make([]R, len(collection))

	for i, item := range collection {
		result[i] = iteratee(item, i)
	}

	return result
}

func main() {
	a := []int{1, 2, 3, 4, 5}
	b := Map(a, func(t int, _ int) string {
		return strconv.Itoa(t)
	})
	expect := []string{"1", "2", "3", "4", "5"}
	for i := 0; i < len(a); i++ {
		if b[i] != expect[i] {
			panic(nil)
		}
	}
	fmt.Println(b, expect)
}
