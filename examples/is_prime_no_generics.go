package main

import (
	"fmt"
	"math"
)

func absInt8(x int8) int8 {
	if x < 0 {
		return -x
	}
	return x
}

func IsPrimeInt8(x int8) bool {
	x = absInt8(x)
	if x < 2 {
		return false
	}
	if x == 2 {
		return true
	}
	for i := int8(2); i <= int8(math.Sqrt(float64(x))); i++ {
		if x%i == 0 {
			return false
		}
	}
	return true
}

func main() {
	n := int8(57)
	isPrime := IsPrimeInt8(n)
	if isPrime {
		fmt.Printf("%v is prime\n", n)
	} else {
		fmt.Printf("%v is not prime\n", n)
	}
}
