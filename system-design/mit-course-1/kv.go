package main

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"sync"
)

// rpc.Client struct looks like this:
//
// type Client struct {
//     codec ClientCodec

//     reqMutex sync.Mutex // protects following
//     request  Request

//     mutex    sync.Mutex // protects following
//     seq      uint64
//     pending  map[uint64]*Call
//     closing  bool // user has called Close
//     shutdown bool // server has told us to stop
// }

//
// Common RPC request/reply definitions
//

type PutArgs struct {
	Key   string
	Value string
}

type PutReply struct {
}

type GetArgs struct {
	Key string
}

type GetReply struct {
	Value string
}

//
// Client
//

func connect() *rpc.Client {
	client, err := rpc.Dial("tcp", ":1234")
	if err != nil {
		log.Fatal("dialing:", err)
	}
	return client
}

func get(key string) string {
	client := connect()
	args := GetArgs{key}
	reply := GetReply{}
	err := client.Call("KV.Get", &args, &reply)
	if err != nil {
		log.Fatal("error:", err)
	}
	client.Close()
	return reply.Value
}

func put(key string, val string) {
	client := connect()
	args := PutArgs{key, val}
	reply := PutReply{}
	err := client.Call("KV.Put", &args, &reply)
	if err != nil {
		log.Fatal("error:", err)
	}
	client.Close()
}

//
// Server
//

// The server() function starts an RPC server for a key-value store. It creates the in-memory storage with `kv := &KV{data: map[string]string{}}`, then creates an RPC server using `rpc.NewServer()`. The next important step is `rpcs.Register(kv)`, which tells the RPC server to expose the methods on the `KV` object so clients can call them remotely.

// After that, it opens a TCP socket with `net.Listen("tcp", ":1234")`. This is the network listening step: the program is now waiting for client connections on port 1234. If listening fails, it immediately exits with `log.Fatal("listen error:", e)`. The return value `l` is a listener object that accepts incoming connections.

// The goroutine then enters a loop calling `l.Accept()`. `Accept()` blocks until a client connects. When a connection arrives, it creates a new `conn` and starts a separate goroutine with `go rpcs.ServeConn(conn)`. That means the server can handle multiple clients at the same time without blocking the main accept loop. Each connection is served by the same RPC server instance, which decodes incoming RPC requests and dispatches them to the registered `KV` methods.

// If `Accept()` returns an error, the loop breaks and the listener is closed with `l.Close()`. In other words, this code is setting up a long-lived background RPC server: it listens for TCP connections, accepts clients, and serves each connection concurrently by invoking the methods registered on `KV`.

// Q: So net.Listen is not blocking? i.e. the server() function executes until l.Accept() (which is in its own goroutine separate from the outer server() ) ?

// Yes, that is the right mental model.

// `net.Listen(...)` itself is not the long blocking part in this design. It does one important setup step: it creates a listening socket bound to the port and returns a `net.Listener`. That call usually returns quickly, assuming the port is available.

// The actual blocking behavior happens at `l.Accept()`, which waits for the next incoming TCP connection. In this code, that call is inside a goroutine:

// ```go
// go func() {
//     for {
//         conn, err := l.Accept()
//         if err == nil {
//             go rpcs.ServeConn(conn)
//         } else {
//             break
//         }
//     }
//     l.Close()
// }()
// ```

// So the outer `server()` function does not stall there. It starts the goroutine and then returns immediately. The main goroutine can keep going, and the listener continues running in the background waiting for clients.

// In short:

// - `server()` sets up the RPC server and starts an accept loop in a goroutine.
// - `l.Accept()` blocks inside that goroutine, not in the main function.
// - `rpcs.ServeConn(conn)` is also started in its own goroutine per client, so each request can be handled concurrently.

// That is why the server can remain alive while accepting connections over time.

type KV struct {
	mu   sync.Mutex
	data map[string]string
}

func server() {
	kv := &KV{data: map[string]string{}}
	rpcs := rpc.NewServer()
	rpcs.Register(kv)
	l, e := net.Listen("tcp", ":1234")
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go func() {
		for {
			conn, err := l.Accept()
			if err == nil {
				go rpcs.ServeConn(conn)
			} else {
				break
			}
		}
		l.Close()
	}()
}

func (kv *KV) Get(args *GetArgs, reply *GetReply) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	reply.Value = kv.data[args.Key]

	return nil
}

func (kv *KV) Put(args *PutArgs, reply *PutReply) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	kv.data[args.Key] = args.Value

	return nil
}

//
// main
//

func main() {
	server()

	put("subject", "6.5840")
	fmt.Printf("Put(subject, 6.5840) done\n")
	fmt.Printf("get(subject) -> %s\n", get("subject"))
}

// More explanations:
//
// P: explain rpc.Dial in this code. What does it do?

// `rpc.Dial` is the helper that creates a client connection for Go’s RPC system. In the standard library, it does two important things: it opens a real network connection with `net.Dial(network, address)`, and then it wraps that connection in an `rpc.Client` using `NewClient(conn)`. If the TCP connection fails, it returns an error immediately; otherwise it returns a ready-to-use client.

// This is exactly the function being used in your code when you call `connect()`:

// `client, err := rpc.Dial("tcp", ":1234")`

// That means: “connect to a TCP server listening on port 1234, and build an RPC client that can send method calls to it.” In this example, the server was started with `net.Listen("tcp", ":1234")` and then `rpcs.ServeConn(conn)` handles incoming RPC requests.

// The key idea is that `rpc.Dial` is the client-side startup step. It does not send any application-specific request yet; it just establishes the connection and creates the client object that will later be used for `client.Call(...)`. Once you have that client, you can call methods such as `"KV.Get"` or `"KV.Put"` over the network.

// So in your KV example, `rpc.Dial` is the bridge between the local Go program and the remote server. It is the “open the socket and prepare the RPC client” step that makes the later calls like `client.Call("KV.Get", &args, &reply)` possible.

// P: so Dial actually opens the network connection ?

// I will answer directly from the Go RPC behavior in this code so it matches what your example is doing.

// Yes — `rpc.Dial` does open the network connection.

// In this code, the call:

// ```go
// client, err := rpc.Dial("tcp", ":1234")
// ```

// means: “create a TCP connection to port 1234 on the local machine, then wrap that connection in an RPC client.” So it is not just a name lookup or a config step; it actually opens a socket and connects to the server.

// The server side is doing the matching work:

// ```go
// l, e := net.Listen("tcp", ":1234")
// ```

// That listens for incoming TCP connections on port 1234, and then each accepted connection is passed to:

// ```go
// go rpcs.ServeConn(conn)
// ```

// which handles RPC requests over that connection.

// So the flow is:

// 1. Server calls `net.Listen("tcp", ":1234")`
// 2. Client calls `rpc.Dial("tcp", ":1234")`
// 3. A TCP connection is established
// 4. The client can then do:

// ```go
// client.Call("KV.Get", &args, &reply)
// ```

// and the server receives and processes the request.

// > In other words, `rpc.Dial` is the client-side equivalent of “connect to the RPC server,” and yes, it opens the actual network connection.
