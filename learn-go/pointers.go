package main

import "fmt"

// the argument p is a pointer to an int
func f( p *int ) {
	fmt.Println("Pointer address for p: ", p)
	fmt.Println("Value at address: ", *p)
	return
}

func main() {
	i := 5
	f( &i ) // & is "the (memory) address of" operator
}

