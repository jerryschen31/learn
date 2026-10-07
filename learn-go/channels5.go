package main

import "fmt"
import "time"

func main() {
  fmt.Println("starting main() - creating channels")
  ch := make(chan int)
  done := make(chan int)

  // this goroutine is started and does not block the main routine
  go func() {
    <-ch // this will block until the channel is closed (which happens in the main goroutine after the 3-second sleep), or a value is received on the channel (e.g., ch<-5)
    fmt.Println("channel closed")
    done<-5 // tell main we're finished (send a value onto the done channel, which is another way to unblock)
  }()
  
  time.Sleep(3 * time.Second)
  close(ch)
  <-done // blocks until done is closed
  fmt.Println("We are done!")
 }
