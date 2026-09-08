package main

import "fmt"

type Person struct {
	Name string
	Age  int
}

func (p Person) String() string {
	return fmt.Sprintf("%v (%v years oollld)", p.Name, p.Age)
}

type Person2 struct {
	Name string
	Age int
}

type IPAddr [4]byte

func (ip IPAddr) String() string {
	return fmt.Sprintf("%v.%v.%v.%v",ip[0],ip[1],ip[2],ip[3])
}


func main() {
	// Person satisfies the Stringer interface because it has a String() method
	a := Person{"Arthur Dent", 42}
	z := Person{"Zaphod Beeblebrox", 9001}
	// This prints the strings using the defined String() method above
	fmt.Println(a)
	fmt.Println(z)
	b := Person2{"Jerry Chen", 45}
	// This just prints the struct itself because Person2 does not have a String() method and does not satisfy the Stringer interface that Println expects
	fmt.Println(b)

	hosts := map[string]IPAddr{
		"loopback":  {127, 0, 0, 1},
		"googleDNS": {8, 8, 8, 8},
	}
	for name, ip := range hosts {
		fmt.Printf("%v: %v\n", name, ip)
	}
}
