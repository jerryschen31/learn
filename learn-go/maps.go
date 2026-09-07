package main

import "fmt"

func main() {
	m := make( map[string]int ) // map with no keys (not nil since memory is allocated
	m2 := map[string]int{}  // equivalent to m - creates a map with no keys (not nil since memory is allocated)
	var m3 map[string]int   // this defines a nil map variable m3 that hasn't been initialized to anything (so is nil)
	fmt.Println(m)
	fmt.Println(m2)
	if m3 == nil {
		fmt.Println(m3)
	}

	m["one"] = 1 // note that a new allocation of memory needs to happen since we started with an empty map
	m2["two"] = 2

	fmt.Println(m)
	fmt.Println(m2)

	m4 := make( map[string]int, 5 ) // pre-allocate memory for 5 key: value pairs in the hash map - this is more efficient
	m4["one"] = 1
	m4["two"] = 2
	m4["three"] = 3
	m4["four"] = 4
	m4["five"] = 5
	fmt.Println(m4)

	m5 := map[string]int{ "one": 1, "two": 2 } // can initialize with key: value pairs when we declare
	fmt.Println(m5)
	m5["one"] = 11
	fmt.Println(m5)
	m5["three"] = 3
	delete(m5, "three") // remove a key: value
	val, present := m["three"] // val will be zero (default value for keys not present)
	fmt.Println("The key three has value", val, ". Is it present in the hash map m5? ", present)
}
