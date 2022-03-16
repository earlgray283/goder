package main

import (
	"fmt"
	"strconv"

	"github.com/samber/lo"
)

func main() {
	a := []int{1, 2, 3, 4, 5}
	b := lo.Map(a, func(t int, _ int) string {
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
