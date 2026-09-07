package main

import (
	"fmt"
	"math"
)

// defining new types is common in Go
type MathFunc func(float64, float64) float64

func compute(fn func(float64, float64) float64, x float64, y float64) float64 {
	return fn(x, y)
}

// notice definition of custom type MathFunc cleans up the argument list
func compute2( fn MathFunc, x float64, y float64) float64 {
	return fn(x, y)
}

func main() {
	// hypotenuse is the third side of a triangle with other side lengths x and y
	hypotenuse := func( x float64, y float64 ) float64 {
		return math.Sqrt( x*x + y*y )
	}

	// notice that hypotenuse is a function passed as an argument
	fmt.Println( compute(hypotenuse, 3, 4) )
	fmt.Println( compute2(hypotenuse, 3, 4) )
}
