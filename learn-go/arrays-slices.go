package main

import "fmt"

func main() {
	var a [2]string
	a[0] = "Hello"
	a[1] = "World"
	fmt.Println(a[0], a[1])
	fmt.Println(a)

	primes := [6]int{2, 3, 5, 7, 11, 13} // primes is an array
	var s []int = primes[1:4] // s is a slice
	t := primes[2:5] // t is also a slice
	// slice itself is not a separate struct - just a subset of the underlying array
	fmt.Println(primes)
	fmt.Println(s)
	fmt.Println(t)

	t[1] = 19 // this changes the element of the actual array, so slices that overlap this element will also see the change
	fmt.Println(primes)
	fmt.Println(s)
	fmt.Println(t)

	q := []int{1,2,3,4,5} // this creates an array, then builds a slice q that references this (entire) array
	fmt.Println(q)
	fmt.Println(q[1:])
	fmt.Println(q[:3])

	// a weird property of a slice is its "capacity" - which is the length from the first element of the slice until the end of the underlying array
	r := q[1:3]
	fmt.Println("slice r: %v length=%d capacity=%d\n", r, len(r), cap(r))

	// a slice can be extended out until the end of the underlying array - not sure yet why this is useful
	r = r[0:4]
	fmt.Println("slice r: %v length=%d capacity=%d\n", r, len(r), cap(r))

	// nil slice
	var ni []int
	if ni == nil {
		fmt.Println(ni, len(ni), cap(ni))
	}

	// create a array of size 5 with all zeros, using make function - slice b is the entire array of zeros
	b := make([]int, 5) // len(b) == 5
	fmt.Println("slice b: %v length=%d capacity=%d\n", b, len(b), cap(b))

	// create an array of size 5 (capacity = 5, 3rd argument) but explicity set slice length to 0 - nil slice (2nd arg)
	c := make([]int, 0, 5) // len(c) == 0, cap(c) == 5
	fmt.Println("slice c: %v length=%d capacity=%d\n", c, len(c), cap(c))

	// append - note that if memory capacity is not allocated ahead of time like in "c" above, Go needs to allocate a new array with greater capacity on some appends
	// under the hood, as array grows, it will add extra capacity so new appends won't trigger a re-allocation of memory
	var d []int
	d = append(d, 0)
	fmt.Println("slice d: %v length=%d capacity=%d\n", d, len(d), cap(d))

	d = append(d, 1, 2, 3) // can append multiple elements
	fmt.Println("slice d: %v length=%d capacity=%d\n", d, len(d), cap(d))

	d = append(d, 4, 5)
	fmt.Println("slice d: %v length=%d capacity=%d\n", d, len(d), cap(d))

	// ranges
	var nums = []int{1, 2, 4, 8, 16, 32, 64, 128}

	for element, value := range nums {
		fmt.Printf("element at index %d has value %d\n", element, value)
	}
}
