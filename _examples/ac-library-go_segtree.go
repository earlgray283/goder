package main

import (
	"github.com/earlgray283/ac-library-go/segtree"
)

func main() {
	a := []int{3, 5, 2, 11, 9, 6, 20, 8}
	segt := segtree.NewBySlice(
		func(a, b int) int {
			if a < b {
				return a
			}
			return b
		},
		func() int { return 1<<31 - 1 },
		a,
	)
	// [0, 2)の最小値を求める
	if got := segt.Prod(0, 2); got != 3 {
		panic("")
	}
	// [2, 5)の最小値を求める
	if got := segt.Prod(2, 5); got != 2 {
		panic("")
	}
	// {3, 5, 2, 11, 9, 6, 20, 8}
	segt.Set(0, 1)
	// {1, 5, 2, 11, 9, 6, 20, 8}
	segt.Set(2, 12)
	// {1, 5, 12, 11, 9, 6, 20, 8}

	// [0, 2)の最小値を求める
	if got := segt.Prod(0, 2); got != 1 {
		panic("")
	}
	// [2, 5)の最小値を求める
	if got := segt.Prod(2, 5); got != 9 {
		panic("")
	}
}
