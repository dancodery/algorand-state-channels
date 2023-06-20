from typing import Tuple
import base64
import socket
import math
from hashlib import sha256

from algosdk.v2client.algod import AlgodClient
from algosdk import transaction
from algosdk.logic import get_application_address
from algosdk import encoding

from .account import Account
from .payment_contract import approval_program, clear_state_program
from .util import (
    waitForTransaction,
    fullyCompileContract,
    getAppGlobalState,
    sha3_256,
    signBytes,
    verifyBytes,
)

APPROVAL_PROGRAM = b""
CLEAR_STATE_PROGRAM = b""

# TCP settings
HOST = "localhost"
PORT = 28547


def getContracts(client: AlgodClient) -> Tuple[bytes, bytes]:
    """Get the compiled TEAL contracts for the payment.
    Args:
        client: An algod client that has the ability to compile TEAL programs.
    Returns:
        A tuple of 2 byte strings. The first is the approval program, and the
        second is the clear state program.
    """
    global APPROVAL_PROGRAM
    global CLEAR_STATE_PROGRAM

    if len(APPROVAL_PROGRAM) == 0:
        APPROVAL_PROGRAM = fullyCompileContract(client, approval_program())
        CLEAR_STATE_PROGRAM = fullyCompileContract(client, clear_state_program())

    return APPROVAL_PROGRAM, CLEAR_STATE_PROGRAM


def createPaymentApp(
    client: AlgodClient,
    sender: Account,
    counterparty: str,
    penalty_reserve=100_000,
    dispute_window=1000, # in rounds
) -> int:
    """Create payment smart contract
    Args:
        client: An algod client.
        sender: The account that will create the payment application.
        counterparty: The Algo address of the counterparty.
        penalty_reserve: The penalty reserve in microAlgos for penalizing old state commitments.
    Returns:
        The ID of the newly created payment app.
    """
    approval, clear = getContracts(client)

    globalSchema = transaction.StateSchema(num_uints=8, num_byte_slices=2)
    localSchema = transaction.StateSchema(num_uints=0, num_byte_slices=0)

    app_args = [
        encoding.decode_address(counterparty),
        penalty_reserve.to_bytes(8, "big"),
        dispute_window.to_bytes(8, "big"),
    ]

    txn = transaction.ApplicationCreateTxn(
        sender=sender.getAddress(),
        on_complete=transaction.OnComplete.NoOpOC,
        approval_program=approval,
        clear_program=clear,
        global_schema=globalSchema,
        local_schema=localSchema,
        app_args=app_args,
        sp=client.suggested_params(),
    )
    signedTxn = txn.sign(sender.getPrivateKey())

    client.send_transaction(signedTxn)
    response = waitForTransaction(client, signedTxn.get_txid())
    assert response.applicationIndex is not None and response.applicationIndex > 0
    return response.applicationIndex


def setupPaymentApp(
        client: AlgodClient,
        appID: int,
        funder: Account,
        channelCapacity: int,
) -> None:
    """Funds the Payment contract with the required Algo coins.
    Args:
        client: An algod client.
        appID: The ID of the payment app.
        funder: The account that will fund the payment app.
        channelCapacity: The amount of Algo coins to fund the payment app with.
    Returns:
        None
    """
    appAddr = get_application_address(appID)

    suggestedParams = client.suggested_params()

    fundAppTxn = transaction.PaymentTxn(
        sender=funder.getAddress(),
        receiver=appAddr,
        amt=channelCapacity,
        sp=suggestedParams,
    )
    callFundTxn = transaction.ApplicationCallTxn(
        sender=funder.getAddress(),
        index=appID,
        on_complete=transaction.OnComplete.NoOpOC,
        app_args=[b"fund"],
        sp=suggestedParams,
    )

    transaction.assign_group_id([fundAppTxn, callFundTxn])

    signedFundAppTxn = fundAppTxn.sign(funder.getPrivateKey())
    signedCallFundTxn = callFundTxn.sign(funder.getPrivateKey())

    client.send_transactions([signedFundAppTxn, signedCallFundTxn])

    waitForTransaction(client, signedFundAppTxn.get_txid())


def transact(
        client: AlgodClient,
        appID: int,
        sender: Account,
        amount: int,  
) -> Tuple[int, int]:
    """Creates an on chain transaction to log new balances for alice and bob.
    Args:
        client: An algod client.
        appID: The ID of the payment app.
        sender: The account that will send the Algo coins to the other party.
        amount: The amount of Algo coins to send to the other party.
    Returns:
        A tuple of 2 integers. The first is the new balance of Alice, and the second is the new balance of Bob.
    """    
    suggestedParams = client.suggested_params()

    transactAppTxn = transaction.ApplicationCallTxn(
        sender=sender.getAddress(),
        index=appID,
        on_complete=transaction.OnComplete.NoOpOC,
        app_args=[b"transact", amount.to_bytes(8, "big")],
        sp=suggestedParams,
    )
    signedTransactAppTxn = transactAppTxn.sign(sender.getPrivateKey())
    client.send_transaction(signedTransactAppTxn) # type: ignore
    waitForTransaction(client, signedTransactAppTxn.get_txid())

    # return Alice and Bob's balances
    newGlobalState = getAppGlobalState(client, appID)
    aliceBalance = newGlobalState[b"latest_alice_balance"]
    bobBalance = newGlobalState[b"latest_bob_balance"]
    return (aliceBalance, bobBalance) # type: ignore

def increaseBudgetSignAndSendTransaction(
        client: AlgodClient,
        appID: int,
        sender: Account,
        txn: transaction.Transaction,
        amount: int,
        ) -> None:
    """ Increase the budget of txn by amount, signs and sends the transaction
    Args:
        client: An algod client.    
        appID: The ID of the payment app.
        sender: The account that will sign the transactions.
        txn: The transaction that will be prepended to the group of transactions.
        amount: The amount of Algo coins to increase the budget by.
    Returns:
        None
    """
    suggestedParams = client.suggested_params()
    amountOfTransactions = math.ceil(amount / 700) # round up to nearest 700; increase budget by 700 per transaction
    transactions = [txn]

    # loop once for each transaction
    for i in range(amountOfTransactions):
        increaseBudgetAppTxn = transaction.ApplicationCallTxn(
            sender=sender.getAddress(),
            index=appID,
            on_complete=transaction.OnComplete.NoOpOC,
            app_args=[b"increaseBudget", str(i).encode()],
            sp=suggestedParams,
        )
        transactions.append(increaseBudgetAppTxn)
    
    transaction.assign_group_id(transactions)

    signedTxns = []
    for txn in transactions:
        signedTxns.append(txn.sign(sender.getPrivateKey()))

    client.send_transactions(signedTxns)
    waitForTransaction(client, signedTxns[0].get_txid())


def signState(
        client: AlgodClient,
        appID: int,
        alice: Account,
        bob: Account,
        alice_balance: int,
        bob_balance: int,
        algorand_port = 4161,
) -> Tuple[int, int]:
    """Signs the current state of the payment app.
    Args:
        client: An algod client.
        appID: The ID of the payment app.
        alice: The account of Alice.
        bob: The account of Bob.
    Returns:
        A tuple of 2 integers. The first is the new balance of Alice, and the second is the new balance of Bob.
    """
    suggestedParams = client.suggested_params()
    timestamp = 1685318789

    data_raw = (algorand_port).to_bytes(8, "big") + b"," \
                + (appID).to_bytes(8, "big") + b"," \
                + (alice_balance).to_bytes(8, "big") + b"," \
                + (bob_balance).to_bytes(8, "big") + b"," \
                + (timestamp).to_bytes(8, "big")
    data_hashed = sha3_256(data_raw)

    alice_signed_bytes = signBytes(data_hashed, alice.getPrivateKey())
    bob_signed_bytes = signBytes(data_hashed, bob.getPrivateKey())

    # alice_pub_key = alice.getAddress()
    # bob_pub_key = bob.getAddress()

    # print(data.hex())
    # print(base64.b64decode(signed_bytes).hex())
    # print(encoding.decode_address(pub_key).hex(), "\n")
    # if verifyBytes(data, signed_bytes, pub_key):
    #     print("Signature is valid")
    # else:
    #     print("Signature is invalid")

    app_args = [
        b"loadState",
        # BEGIN signed values
        (algorand_port).to_bytes(8, "big"), # algorand_port
        (alice_balance).to_bytes(8, "big"), # alice_balance
        (bob_balance).to_bytes(8, "big"), # bob_balance
        (timestamp).to_bytes(8, "big"), # timestamp
        # END signed values
        base64.b64decode(alice_signed_bytes),
        base64.b64decode(bob_signed_bytes),
    ]
    signStateAppTxn = transaction.ApplicationCallTxn(
        sender=alice.getAddress(),
        index=appID,
        on_complete=transaction.OnComplete.NoOpOC,
        app_args=app_args,
        sp=suggestedParams,
    )    
    increaseBudgetSignAndSendTransaction(client, appID, alice, signStateAppTxn, 3930) # 1x Sha3_256 a 130 + 2x Ed25519Verify a 1900
    
    # return Alice and Bob's balances
    newGlobalState = getAppGlobalState(client, appID)
    aliceBalance = newGlobalState[b"latest_alice_balance"]
    try:
        bobBalance = newGlobalState[b"latest_bob_balance"]
    except KeyError:
        bobBalance = 0
    return (aliceBalance, bobBalance)


def alice_process():
    # start the tcp server
    server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    server_socket.bind((HOST, PORT))
    server_socket.listen(3) # maximum number of queued connections

    print("Alice payment node is running and waiting for Bob's payment requests...")


def payment_node_process(
        participant_name: str,
        is_creator: bool,
    ):
    # start the tcp server
    server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    server_socket.bind((HOST, PORT))
    server_socket.listen(3) # maximum number of queued connections

    print(f"{participant_name} {'(payment channel creator)' if is_creator else ''} node is running and waiting for requests...")

    while True:
        # accept incoming connection
        client_socket, client_address = server_socket.accept()
        print(f"{participant_name}: Connection from {client_address} has been established!")
