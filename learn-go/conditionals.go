package main

import (
	"fmt"
)

func squared_less_than_20( x int ) bool {
	if y := x * x; y < 20 {
		return true
	} else {
		return false
	}
}

func print_n_times( s string, n int ) {
	for i := 0; i < n; i++ {
		fmt.Println(s)
	}
	return
}

func how_many_digits( n int ) {
	if n < 0 {
		n = -n
	}
	switch {
	case n<10:
		fmt.Println(1)
	case n>=10 && n<100:
		fmt.Println(2)
	case n>=100 && n<1000:
		fmt.Println(3)
	case n>=1000 && n<10000:
		fmt.Println(4)
	case n>=10000 && n<100000:
		fmt.Println(5)
	case n>=100000:
		fmt.Println(6)
	default:
		fmt.Println("default 'else' of switch-case")
	}
	return
}


func main() {
	a := 3
	b := 5
	fmt.Printf("%v less than 20? %v\n", a, squared_less_than_20( a ))
	fmt.Printf("%v less than 20? %v\n", b, squared_less_than_20( b ))

	a = a + 3
	fmt.Printf("%v less than 20? %v\n", a, squared_less_than_20( a ))

	s := "hello"
	n := 5
	print_n_times( s, n )

	how_many_digits(99)
}
