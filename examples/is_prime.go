package main

import (
	"fmt"
	"math"
)

type Integer interface {
	~int | ~int8 | ~uint | ~uint8
}

func abs[T Integer](x T) T {
	if x < 0 {
		return -x
	}
	return x
}

func IsPrime[T Integer](x T) bool {
	x = abs(x)
	if x < 2 {
		return false
	}
	if x == 2 {
		return true
	}
	for i := T(2); i <= T(math.Sqrt(float64(x))); i++ {
		if x%i == 0 {
			return false
		}
	}
	return true
}

func main() {
	n := int8(57)
	isPrime := IsPrime(n)
	if isPrime {
		fmt.Printf("%v is prime\n", n)
	} else {
		fmt.Printf("%v is not prime\n", n)
	}
}
