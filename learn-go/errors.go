package main

import (
	"fmt"
	"strconv"
	"time"
)

type MyError struct {
	When time.Time
	What string
}

func (e *MyError) Error() string {
	return fmt.Sprintf("at %v, %s",
		e.When, e.What)
}

// error is a special built-in interface type - underlying type (here, a MyError struct) should define an Error() function
func NewError() error {
	return &MyError{
		time.Now(),
		"it didn't work",
	}
}




func main() {
	if err := NewError(); err != nil {
		fmt.Println(err)
	}

	n1 := "42"
	n2 := "blah"
	i1, err := strconv.Atoi(n1)
	if err != nil {
    		fmt.Printf("couldn't convert number: %v\n", err)
    		return
	}
	fmt.Println("Converted integer:", i1)
	_, err2 := strconv.Atoi(n2)
	if err2 != nil {
		fmt.Printf("couldn't convert number: %v\n", err2)
		return
	}
}
