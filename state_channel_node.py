#!/usr/bin/env python
import argparse
import threading
import socket
import os

from functools import singledispatchmethod

from payment.testing.setup import getAlgodClient

from payment.testing.resources import (
	getTemporaryAccount)


class StateChannelNode:
    _instance = None
    
    server_running = False
    algod_client = None
    algo_account = None

    MESSAGE = "EMPTY"


    @classmethod
    def get_instance(cls):
        if cls._instance is None:
            cls._instance = cls()
        return cls._instance

    @singledispatchmethod
    @classmethod
    def _get_instance_with_args(cls, *args, **kwargs):
        if cls._instance is None:
            cls._instance = cls(*args, **kwargs)
        return cls._instance
    
    def some_method(self):
        print("Singleton method called")

    def handle_connection(self, client_socket, client_address):
        print(f"Connection from {client_address} has been established!")

        data = client_socket.recv(1024)
        print("Received data:", data.decode("utf-8"))

        # Send a response
        response = "Hello back from the payment channel node!"
        client_socket.send(response.encode("utf-8")) 


    def start_tcp_server(self, port: int):
        if self.server_running:
            print("Server is already running!")
            return
        
        self.server_running = True
        print(f"Starting the payment channel node on port {port}...")

        # get the algod client and create a temporary account
        # self.algod_client = getAlgodClient()
        # self.algo_account = getTemporaryAccount(self.algod_client)

        # print(f"Account address: {self.algo_account.getAddress()}")
        
        # start the tcp server
        server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        server_socket.bind(("", port)) # accept any ip address
        server_socket.listen(3) # maximum number of queued connections
        print("Listening for incoming connections...")

        while self.server_running:
            # accept incoming connection
            client_socket, client_address = server_socket.accept()
            threading.Thread(target=self.handle_connection, args=(client_socket, client_address)).start()


    def stop_tcp_server(self):
        if not self.server_running:
            print("Server is not running.")
            return
        
        self.server_running = False
        print("Stopping the payment channel node.")


    def send_message(
            self,
            host: str = "localhost",
            port: int = 28547,
        ):
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
            print(f"Connecting to {host}:{port}...")
            sock.connect((host, port))
            sock.sendall(self.MESSAGE.encode("utf-8"))

            # Receive data from the server and shut down
            data = sock.recv(1024)
            print("Received data:", data.decode("utf-8"))


    def open_channel(
            self,
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
        self.send_message(host=partner_ip, port=partner_port)


    def run_command_interpreter(self):
        # Create the argument parser
        parser = argparse.ArgumentParser(description="Payment Channel Node CLI")

        subparsers = parser.add_subparsers(dest="command")

        # Add runserver command
        runserver_parser = subparsers.add_parser("runserver")
        runserver_parser.add_argument("--port", required=False, type=int, help="Port number to run the server on")

        # Add stop command
        stop_parser = subparsers.add_parser("stop") 

        # Add openchannel command and its arguments
        openchannel_parser = subparsers.add_parser("openchannel")
        openchannel_parser.add_argument("--partner_ip", required=True, type=str, help="IP address of the channel partner")
        openchannel_parser.add_argument("--partner_port", required=False, type=int, help="Port number of the channel partner") # Partner account address
        openchannel_parser.add_argument("--funding", required=True, type=int, help="Funding amount in microAlgos")
        openchannel_parser.add_argument("--penalty_reserve", required=True, type=int, help="Penalty Reserve amount in microAlgos for cheating")
        openchannel_parser.add_argument("--dispute_window", required=True, type=int, help="Dispute Time Window in rounds")

        # Parse the command line arguments
        args = parser.parse_args()

        # Execute the command
        if args.command == "runserver":
            if args.port is None:
                args.port = 28547
            self.MESSAGE = "SERVER_STARTED"
            tcp_server_thread = threading.Thread(target=self.start_tcp_server, args=(args.port,))
            tcp_server_thread.start()

        elif args.command == "stop":
            self.MESSAGE = "SERVER_STOPPED"
            self.stop_tcp_server()

        elif args.command == "openchannel":
            if args.partner_port is None:
                args.partner_port = 28547
            self.open_channel(
                partner_ip=args.partner_ip,
                partner_port=args.partner_port,
                funding=args.funding,
                penalty_reserve=args.penalty_reserve,
                dispute_window=args.dispute_window,
            )
        else:
            parser.print_help()


if __name__ == "__main__":
    node = StateChannelNode.get_instance()
    node.run_command_interpreter()
   
