package main

import "fmt"

type Num interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}

func add[T Num](a, b T) T {
	return a + b
}

func main() {
	fmt.Println(add(33, 4))
}
