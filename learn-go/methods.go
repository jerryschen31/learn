package main

import (
	"fmt"
)

type TwoNumbers struct {
	X int
	Y int
}

// methods for structs are declared like this:
// (n TwoNumbers) is called a "receiver" argument, with the receiver n of type TwoNumbers
// so the struct this method Add() belongs to is written within the receiver argument
// Remember: a method is just a function with a receiver argument
// Note that this creates a COPY of the struct for operation within this method
func (n TwoNumbers) Add() int {
	fmt.Printf( "In Add(): %p\n", &n )
	return n.X + n.Y
}

// This is equivalent to the above
func Add2( n TwoNumbers ) int {
	fmt.Printf( "In Add2(): %p\n", &n )
	return n.X + n.Y
}

// This passes a pointer to the struct that was passed, avoiding making a potentially expensive copy if we wanted to mutate the original struct anyway
func (n *TwoNumbers) Add3() int {
	fmt.Printf( "In Add3(): %p\n", n )
	return n.X + n.Y
}

// non-struct types can also have methods
type MyFloat float64

func (f MyFloat) Abs() float64 {
	if f < 0 {
		return float64(-f)
	} else {
		return float64(f)
	}
}

func main() {
	num := TwoNumbers{ 3, 5 }
	fmt.Printf( "In main(): %p\n", &num )
	fmt.Println( num )
	fmt.Println( num.Add() )
	fmt.Println( Add2( num ) )
	
	fmt.Println( num.Add3() )

	num2 := MyFloat( -5 )
	fmt.Println( num2.Abs() )
}

// % go run methods.go
// In main(): 0x1c69d59f80c0
// {3 5}
// In Add(): 0x1c69d59f80f0 // note that Add() created a copy of the struct, different address than the original struct in main()
// 8
// In Add2(): 0x1c69d59f8100
// 8
// In Add3(): 0x1c69d59f80c0 // Add3() has the same address as the struct in main() because it's a pointer to the struct
// 8
// 5
