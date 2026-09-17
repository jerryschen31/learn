package main

import (
	"fmt"
	"time"
	"sync"
)

func sum( a []int, c chan int, wg *sync.WaitGroup ){
	defer wg.Done() // wg.Done() runs after this function finishes, this signals to the wait group that this goroutine is done (i.e., decrements from wg.Add(2))
	sum := 0
	for _, el := range a {
		sum += el
	}
	c <- sum
	fmt.Println("sum is done ", a )
	time.Sleep( 2 * time.Second )
}

func main() {
	s := []int{7, 3, 2, 1, 5, 6, 4, 2}

	c := make(chan int) // make(chan int, 2)
	var wg sync.WaitGroup // wait groups are used to track when goroutines are done, and wait until they're done before moving on

	wg.Add(2) // track 2 goroutines below
	go sum(s[:4], c, &wg)
	go sum(s[4:], c, &wg)

	x := <- c // this blocks until sum 1st half returns
	fmt.Println("1st half array sum is ", x)
	y := <- c // this blocks until sum 2nd half returns
	fmt.Println("2nd half array sum is ", y)

	wg.Wait() // this blocks until both goroutines are done
	
	fmt.Println("total sum is ", x+y)
}

