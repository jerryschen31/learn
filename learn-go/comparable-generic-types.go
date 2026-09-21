package main

import (
	"fmt"
)

// comparable and any are more generic types that are used to create generic data structures and associated methods

// Index returns the index of x in s, or -1 if not found.
// T is any type that fulfills the built-in constraint "comparable" - which is types that have an == and a few other comparison operators
// here, if T was of type "any" this would fail because this is too loose of a constraint and Go does not know if == is a safe operator on an "any" type
func Index[T comparable](s []T, x T) int {
	// i and v are index and value respectively, over the slice s
	for i, v := range s {
		// v and x are type T, which has the comparable
		// constraint, so we can use == here.
		if v == x {
			return i
		}
	}
	return -1
}

// List represents a singly-linked list that holds values of any type (generic types)
// So basic data structures are a classic use-case of objects and functions that accept generic and comparable types
type List[T any] struct {
	next *List[T]
	val  T
}

func main() {
	// Index works on a slice of ints
	si := []int{10, 20, 15, -10}
	fmt.Println(Index(si, 15))

	// Index also works on a slice of strings
	ss := []string{"foo", "bar", "baz"}
	fmt.Println(Index(ss, "hello"))
}
