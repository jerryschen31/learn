package main

import (
	"fmt"
	"time"
)

func worker(done chan int) {
	fmt.Println("Worker: starting heavy computation...")
	time.Sleep(3 * time.Second)
	fmt.Println("Worker: Done with computation!")

	// use the channel to signal to main() that work is done and unblock
	done <- 0
}

func main() {
	// Create a channel purely for syncing sub-goroutine with main routine (i.e., subroutine signaling to main routine that work is done)
	done := make(chan int)

	// Fire off a subroutine (goroutine - separate thread) to do some work
	go worker(done)

	fmt.Println("Main: Waiting for worker goroutine to finish...")

	// blocks until worker goroutine is done
	<- done
	
	fmt.Println("Main: worker goroutine completed. Program exiting...")
}
