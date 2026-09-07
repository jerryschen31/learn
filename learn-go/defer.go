package main

import "fmt"

func c() (i int) {
	i = 1
	// this executes i++ at the end of the function AFTER the return statement, so i gets incremented as the final return value
    	defer func() { i++ }()
	i = 5
    	return i
}

func main() {
	defer fmt.Println("world")
	j := -1
	defer fmt.Println(j) // defers the current value j = -1
	j++
	
	// at the end of the main() function, the last iteration prints out first
	for i := 0; i < 5; i++ {
		defer fmt.Println(i)
	}
	
	fmt.Println("hello")
	fmt.Println("c() returned value of i is ", c())
}
