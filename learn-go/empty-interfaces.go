package main

import "fmt"

func main() {
	var i interface{}
	describe(i)

	i = 42
	describe(i)

	i = "hello"
	describe(i)
}

func describe(i interface{}) {
	fmt.Printf("(%v, %T)\n", i, i)
}

// An empty interface may hold values of any type. (Every type implements at least zero methods.)
// any is an alias for interface{}, and the two are completely equivalent.
