package main

import (
	"fmt"
	"sync"
)

// Counter has a Mutex so goroutines can lock the counter before incrementing
type Counter struct {
	mu sync.Mutex
	v  map[byte]int
}

// SafeIncrement() safely increments using a lock before each increment
func (c *Counter) SafeIncrement( key byte, wg *sync.WaitGroup ) {
	// when this goroutine is done, it signals back to the wait group
	defer wg.Done()
	// Lock so that only the goroutine that locks can access the map
	c.mu.Lock()
	// instead of c.mu.Unlock() below, could also do defer c.mu.Unlock() here
	c.v[key]++
	// Release the lock so another goroutine can access the map
	c.mu.Unlock()
}

// NonSafeIncrement() does not use the lock before each increment, so races may occur when multiple goroutines try to increment the same value
func (c *Counter) NonSafeIncrement( key byte, wg *sync.WaitGroup ) {
	defer wg.Done()
	// updating the hash map without a lock is really bad - multiple goroutines may be messing with the hash map and Go will throw a panic() if it catches a potentially corruptive concurrent operation on the hash map
	c.v[key]++
}

func main() {
	c := Counter{ v: make(map[byte]int) }
	cmax := 100

	fmt.Println("Incrementing ", cmax, " times")
	var wg sync.WaitGroup
	wg.Add(cmax)
	for i := 0; i < cmax; i++ {
		go c.SafeIncrement( 'a', &wg )
	}
	fmt.Println("Waiting for safe increments to finish")
	wg.Wait() // this blocks until all increments are done
	fmt.Println("Safe increment final value: ", c.v['a'] )

	wg.Add(cmax)
	for i := 0; i < cmax; i++ {
		go c.NonSafeIncrement( 'b', &wg )
	}
	fmt.Println("Waiting for non-safe increments to finish")
	wg.Wait()
	fmt.Println("NonSafe increment final value: ", c.v['b'] )
}
