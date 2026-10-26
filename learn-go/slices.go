package main

import "fmt"

func main() {
	a := make([]string, 2, 4)            // ["","", _, _] -> 4 is the capacity of the slice; 2 is the current length - default value is empty string
	a[0], a[1] = "x", "y"                // ["x", "y", _, _]
	b := append(a, "rule-B")             // writes into slot 3 of a's array
	c := append(a, "rule-C")             // ALSO writes into slot 3, same array
	fmt.Println(a[0], a[1])              // "x" "y"
	fmt.Println(b[2])                    // "rule-C"  ← b's rule was overwritten
	fmt.Println(c[2])                    // also "rule-C"
}
