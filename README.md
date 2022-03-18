# Goder

## Description

Goder enables gophers to compate using generics and external packages.

## Installation

```console
$ go install github.com/earlgray283/goder@latest
```

## Usage

```console
$ goder examples/samber_lo.go
package main

import (
	"fmt"
	"strconv"
)

func main() {
	a := []int{1, 2, 3, 4, 5}
	b := MapXvlbzgba(a, func(t int, _ int) string {
		return strconv.Itoa(t)
	})
	fmt.Println(a, b)
}
func MapXvlbzgba(collection []int, iteratee func(int, int) string) []string {
	result := make([]string, len(collection))
	for i, item := range collection {
		result[i] = iteratee(item, i)
	}
	return result
}
```