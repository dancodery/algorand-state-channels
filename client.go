package main

import "fmt"

// dial function connects to a server
// conn, err := net.Dial("tcp", "golang.org:80")
// if err != nil {
// 	// handle error
// }
// fmt.Fprintf(conn, "GET / HTTP/1.0\r\n\r\n")
// status, err := bufio.NewReader(conn).ReadString('\n')

func notifyOpenChannel(recipient_ip string, app_id uint64) {
	fmt.Printf("notifyOpenChannel: %v, %v\n", recipient_ip, app_id)

	// print DEFAULT_GRPC_PORT
	fmt.Printf("DEFAULT_GRPC_PORT: %v\n", DEFAULT_GRPC_PORT)
}
