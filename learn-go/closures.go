package main

import "fmt"

// next_num is a function that returns a function type (return type is func() int)
// this returned function is called a "closure" - a function that holds a "state" variable from the outer function
func next_num() func() int {
	count := 0
	// returning an anonymous function (not a named function)
	return func() int {
		count++
		return count
	}
}

func main() {
	// when next variable is declared, the function value next_num() IS the returned inner anonymous function
	// calling next_num() executes the outer function, which evaluates to and returns the inner anonymous function and assigns it to next
	next := next_num()

	// next() actually invokes (executes) the inner anonmymous function, which increments and returns the state variable "count"
	fmt.Println( next() ) // 1 - the () executes the anonymous function that is returned by next_num()
	fmt.Println( next() ) // 2
	fmt.Println( next )   // this is the address of the anonymous function value (closure object)
	fmt.Println( next() ) // 3
	fmt.Println( next )   // this is the address of the anonymous function
}
