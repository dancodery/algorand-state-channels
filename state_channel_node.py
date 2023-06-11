#!/usr/bin/env python
import argparse
import threading
import socket
import os

from payment.testing.setup import getAlgodClient

from payment.testing.resources import (
	getTemporaryAccount)

MESSAGE = "EMPTY"

def send_message(
        host: str = "localhost",
        port: int = 28547,
):
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        print(f"Connecting to {host}:{port}...")
        sock.connect((host, port))
        sock.sendall(MESSAGE.encode("utf-8"))

        # Receive data from the server and shut down
        data = sock.recv(1024)
        print("Received data:", data.decode("utf-8"))


def open_channel(
        partner_ip: str,
        partner_port: int,
        funding: int,
        penalty_reserve: int,
        dispute_window: int,
):
    """
    Open a payment channel with a partner account.
    """
    print(f"Opening a payment channel with {partner_ip}:{partner_port}...")
    print(f"Payment channel funding: {funding} microAlgos")
    print(f"Payment channel penalty reserve: {penalty_reserve} microAlgos")
    print(f"Payment channel dispute window: {dispute_window} rounds")
    print("Done\n")
    send_message(host=partner_ip, port=partner_port)


def handle_connection(client_socket, client_address):
    print(f"Connection from {client_address} has been established!")

    data = client_socket.recv(1024)
    print("Received data:", data.decode("utf-8"))

    # Send a response
    response = "Hello back from the payment channel node!"
    client_socket.send(response.encode("utf-8")) 


def start_tcp_server(port: int):
    print(f"Starting the payment channel node on port {port}...")

    # get the algod client and create a temporary account
    algod_client = getAlgodClient()
    algo_address = getTemporaryAccount(algod_client)

    print(f"Account address: {algo_address.getAddress()}")
    
    # start the tcp server
    server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    server_socket.bind(("", port)) # accept any ip address
    server_socket.listen(3) # maximum number of queued connections
    print("Listening for incoming connections...")

    while True:
        # accept incoming connection
        client_socket, client_address = server_socket.accept()
        threading.Thread(target=handle_connection, args=(client_socket, client_address)).start()


if __name__ == "__main__":
    # Create the argument parser
    parser = argparse.ArgumentParser(description="Payment Channel Node CLI")

    # Add openchannel command and its arguments
    subparsers = parser.add_subparsers(dest="command")

    openchannel_parser = subparsers.add_parser("openchannel")
    openchannel_parser.add_argument("--partner_ip", required=True, type=str, help="IP address of the channel partner")
    openchannel_parser.add_argument("--partner_port", required=False, type=int, help="Port number of the channel partner") # Partner account address
    openchannel_parser.add_argument("--funding", required=True, type=int, help="Funding amount in microAlgos")
    openchannel_parser.add_argument("--penalty_reserve", required=True, type=int, help="Penalty Reserve amount in microAlgos for cheating")
    openchannel_parser.add_argument("--dispute_window", required=True, type=int, help="Dispute Time Window in rounds")

    # Add runserver command
    runserver_parser = subparsers.add_parser("runserver")
    runserver_parser.add_argument("--port", required=False, type=int, help="Port number to run the server on")

    # Parse the command line arguments
    args = parser.parse_args()

    # # get singleton instance
    # node = StateChannelNode()

    # Execute the command
    if args.command == "openchannel":
        if args.partner_port is None:
            args.partner_port = 28547
        open_channel(
            partner_ip=args.partner_ip,
            partner_port=args.partner_port,
            funding=args.funding,
            penalty_reserve=args.penalty_reserve,
            dispute_window=args.dispute_window,
        )
    elif args.command == "runserver":
        if args.port is None:
            args.port = 28547
        MESSAGE = "SERVER_STARTED"
        tcp_server_thread = threading.Thread(target=start_tcp_server, args=(args.port,))
        tcp_server_thread.start()
    else:
        parser.print_help()
    
