package main

type Set[K comparable] map[K]struct{}

func MakeSetFromSlice[K comparable](a []K) Set[K] {
	set := Set[K]{}
	for _, elem := range a {
		set[elem] = struct{}{}
	}
	return set
}

func getFirstLast[T any](a []T) (T, T) {
	return a[0], a[len(a)-1]
}
