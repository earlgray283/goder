package main

import (
	"fmt"

	"golang.org/x/exp/constraints"
)

func add[T constraints.Integer](
	a,
	b T,
) T {
	return a + b
}

func main() {
	fmt.Println(add(33, 4))
}
