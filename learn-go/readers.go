package main

import (
	"fmt"
	"io"
	"strings"
	// "os"
)

func main() {
	r := strings.NewReader("Hello, Reader!")
	b := make([]byte, 8)
	// io.Copy(os.Stdout, r) // this reads the entire stream of bytes r and copies to Stdout
	for {
		n, err := r.Read(b) // reads 8 bytes at a time from r (a mock stream source)
		fmt.Printf("n = %v err = %v b = %v\n", n, err, b)
		fmt.Printf("\tb[:n] = %q\n", b[:n])
		if err == io.EOF { // err will hit EOF after the end of the stream (i.e. once entire stream is read)
			break
		}
	}
}
