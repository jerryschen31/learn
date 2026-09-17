package main

import (
	"fmt"
	"time"
)

func increment( n int, c chan int ) {
	defer close(c) // closes channel once this increment function is done
	for i := 0; i < n; i++ {
		fmt.Println("In increment(), about to send ", i)
		c <- i
		time.Sleep( 20 * time.Millisecond )
	}
	fmt.Println("Done with increment()")
}

func main() {
	c := make( chan int, 10 )
	cmax := 3*cap(c) // cap is the buffer capacity of channel c
	fmt.Println("Channel open between increment() and main()")
	go increment( cmax, c )
	for i := range c {  // this will block when channel is empty, and will continually pull values from the buffered channel until the channel is closed
		fmt.Println("In main(), received ", i)
	}
	fmt.Println("Done with main()")
}
