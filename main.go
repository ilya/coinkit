package main

import "bufio"
import "fmt"
import "log"
import "net"
import "os"
import "strconv"

import "coinkit/network"

const (
	BASE_PORT = 9000
	NODES     = 4
)

// Handles an incoming connection
func handleConnection(conn net.Conn) {
	log.Printf("handling a connection")
	for {
		message, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			conn.Close()
			break
		}
		log.Printf("got message: %s", message)
	}
}

func listen(port int) {
	log.Printf("listening on port %d", port)
	ln, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		log.Fatal(err)
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Print("incoming connection error: ", err)
		}
		go handleConnection(conn)
	}
}

func main() {
	// Usage: go run main.go <i> where i is in [0, 1, 2, ..., NODES - 1]
	if len(os.Args) < 2 {
		log.Fatal("Use an argument with a numerical id.")
	}
	id, err := strconv.Atoi(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	if id < 0 || id >= NODES {
		log.Fatalf("invalid id: %d", id)
	}

	port := BASE_PORT + id
	log.Printf("server %d starting up on port %d", id, port)

	for p := BASE_PORT; p < BASE_PORT+NODES; p++ {
		if p == port {
			continue
		}
		peer := network.NewPeer(p)
		go peer.Send("hello")
	}

	listen(port)
}
