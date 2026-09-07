package main

import "fmt"

func main() {
	// f()
	f2()
	fmt.Println("Returned normally from f.")
}

func f() {
	defer func() {
		// recover() picks up the value passed into panic(), cancels the "panic" (so panic msg is not output), and resumes normal execution from just after this defer
		if r := recover(); r != nil {
			fmt.Println("Recovered in f", r)
		}
	}() // trailing parentheses execute this function immediately (immediately invoked function expression)
	// the defer still pushes the execution of this function to after f() returns (defer of a func needs the () so that it actually executes

	fmt.Println("Calling g.")
	g(0)
	fmt.Println("Returned normally from g.")
}

func g(i int) {
	if i > 3 {
		fmt.Println("Panicking!")
		panic(fmt.Sprintf("%v", i)) // Sprintf() returns the formatted output as a string (instead of stdout like in Printf())
		// panic(msg) is like raise Exception(msg) in Python or throw <error> in Javascript / Java / C++
	}
	defer fmt.Println("Defer in g", i)
	fmt.Println("Printing in g", i)
	g(i+1)
}

func f2() {
        fmt.Println("Calling g.")
        g(0)
        fmt.Println("Returned normally from g.")
}

// STDOUT USING f()
// Calling g
// Printing in g 0
// Printing in g 1
// Printing in g 2
// Printing in g 3
// Panicking!
// Defer in g 3
// Defer in g 2
// Defer in g 1
// Defer in g 0
// Recovered in f 4
// Returned normally from f.

// STDOUT USING f2()
// Calling g.
// Printing in g 0
// Printing in g 1
// Printing in g 2
// Printing in g 3
// Panicking!
// Defer in g 3
// Defer in g 2
// Defer in g 1
// Defer in g 0
// panic: 4
//
// goroutine 1 [running]:
// main.g(0x4)
//         /Users/jerry/gh/public/learn/learn-go/panic.go:28 +0x168
// main.g(0x3)
//         /Users/jerry/gh/public/learn/learn-go/panic.go:33 +0xd0
// main.g(0x2)
//         /Users/jerry/gh/public/learn/learn-go/panic.go:33 +0xd0
// main.g(0x1)
//         /Users/jerry/gh/public/learn/learn-go/panic.go:33 +0xd0
// main.g(0x0)
//         /Users/jerry/gh/public/learn/learn-go/panic.go:33 +0xd0
// main.f2()
//         /Users/jerry/gh/public/learn/learn-go/panic.go:38 +0x54
// main.main()
//         /Users/jerry/gh/public/learn/learn-go/panic.go:7 +0x1c
// exit status 2
