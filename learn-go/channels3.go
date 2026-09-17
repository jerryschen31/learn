package main

import (
	"fmt"
	"time"
)

func increment(c chan int, quit chan int) {
	i := 0
	for {
		select {
		case c <- i:
			i++
		case q := <- quit: // <- quit PULLS a value off the channel, and assigns it to q
			fmt.Println("Received on quit channel: ", q)
			return
		default:
			// This default case executes when the other cases cannot execute on a particular "select check" CPU cycle
			fmt.Println("No channel ready, executing default case...")
			time.Sleep( 1 * time.Millisecond )
		}
	} 
}

func main() {
	c := make( chan int )
	quit := make( chan int )

	go func() {
		for i := 0; i < 10; i++ {
			fmt.Println("In main(), received on c channel:  ", <-c)
		}
		quit <- 0
	}()
	increment(c, quit)
}

