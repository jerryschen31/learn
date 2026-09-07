package main

import (
	"fmt"
	"math/cmplx"
)

var (
	V	bool	= false
	MaxInt	uint64	= 1<<64-1
	z	complex128 = cmplx.Sqrt(-5 + 2i)
	z2	complex64  = 3 + 5i
	p	*complex64 = &z2
	i	int	= 32
	f	float32 = float32(i)
)

const Pi float32 = 3.14

func main() {
	fmt.Printf("Type: %T Value: %v\n", V, V)
	fmt.Printf("Type: %T Value: %v\n", MaxInt, MaxInt)
	fmt.Printf("Type: %T Value: %v\n", z, z)
	fmt.Printf("Type: %T Value: %v\n", z2, z2)
	fmt.Printf("Address: %p Experiment: %#v\n", p, p)
	fmt.Printf("Type: %T Value: %v\n", i, i)
	fmt.Printf("Type: %T Value: %v\n", f, f)
	fmt.Printf("Type: %T Value: %v\n", Pi, Pi)
}

