package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
)

type P2PRequest struct {
	Command string
	// Args    []string
	Args [][]byte
}

type P2PResponse struct {
	Message string
	Data    [][]byte
}

func sendRequest(recipient_ip string, request P2PRequest) (response P2PResponse, err error) {
	// connect to peer server
	conn, err := net.Dial("tcp", recipient_ip+":"+strconv.Itoa(DEFAULT_PEER_PORT))
	if err != nil {
		fmt.Fprintf(os.Stdout, "Did not connect to peer server: %v\n", err)
	}
	defer conn.Close()

	json_request, err := json.Marshal(request)
	if err != nil {
		fmt.Fprintf(os.Stdout, "Error marshalling request: %v\n", err)
		return P2PResponse{}, err
	}

	// print json_request to stdout
	fmt.Fprintf(os.Stdout, string(json_request)+"\n")

	// send request to peer server
	_, err = conn.Write(json_request)
	if err != nil {
		fmt.Fprintf(os.Stdout, "Error sending request: %v\n", err)
		return P2PResponse{}, err
	}

	// read response from peer server
	responseData := make([]byte, 1024)
	n, err := conn.Read(responseData)
	if err != nil {
		fmt.Fprintf(os.Stdout, "Error reading response: %v\n", err)
		return P2PResponse{}, err
	}

	var server_response P2PResponse
	err = json.Unmarshal(responseData[:n], &server_response)
	if err != nil {
		fmt.Fprintf(os.Stdout, "Error unmarshalling server_response: %v\n", err)
		return P2PResponse{}, err
	}

	// return response
	return server_response, nil
}
