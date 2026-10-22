package main

import "fmt"

func main() {
	i := 1
	ch := make(chan int, 1) // channel of buffer 1
	ch <- i // this does not block since ch has a buffer of 1
	j := <-ch // this then executes - j reads off the channel and gets value 1; channel now empty
	fmt.Println("(i, j) = (", j, ", ", i, ")")

	a := 2
	b := 3
	ch<-a // this does not block - buffer 1. Earlier i=1 was already read off the channel
	ch<-b // this blocks - exceeds channel buffer capacity of 1
	fmt.Println("never reaches here...")
}

