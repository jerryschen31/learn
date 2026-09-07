package main

import "fmt"

type Vertex struct {
	X int
	Y int
}

func main() {
	v := Vertex{ 1, 2 }
	fmt.Println( v )
	fmt.Println( v.X )

	v2 := Vertex{} // X and Y get initialized to default values (0)
	v2.X = 4
	fmt.Println( v2 )

	p := &v2
	p.Y = 5        // can access a struct through a pointer
	fmt.Println( v2 )
}

