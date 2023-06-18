package main

import (
	"fmt"
	"log"
	"net"
)

func handleConnection(conn net.Conn) {
	defer conn.Close()

	log.Printf("Accepted connection from %s\n", conn.RemoteAddr())

	buffer := make([]byte, 1024)
	_, err := conn.Read(buffer)
	if err != nil {
		log.Printf("Failed to read data: %s\n", err)
		return
	}

	data := string(buffer)
	log.Printf("Received data: %s\n", data)

	response := "Hello back from the payment channel node!"
	_, err = conn.Write([]byte(response))
	if err != nil {
		log.Printf("Failed to write data: %s\n", err)
		return
	}
}

func startServer(port int) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("Failed to start the server: %s\n", err)
	}

	log.Printf("Server started on port%d\n", port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %s\n", err)
			continue
		}

		go handleConnection(conn)
	}
}

func main() {
	port := 28547
	startServer(port)
}
