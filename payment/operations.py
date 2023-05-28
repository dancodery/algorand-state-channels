from typing import Tuple
import base64
from hashlib import sha256

from algosdk.v2client.algod import AlgodClient
from algosdk import transaction
from algosdk.logic import get_application_address
from algosdk import encoding

from .account import Account
from .contracts import approval_program, clear_state_program
from .util import (
    waitForTransaction,
    fullyCompileContract,
    getAppGlobalState,
    signBytes,
    verifyBytes,
)

APPROVAL_PROGRAM = b""
CLEAR_STATE_PROGRAM = b""


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

    globalSchema = transaction.StateSchema(num_uints=4, num_byte_slices=5)
    localSchema = transaction.StateSchema(num_uints=0, num_byte_slices=0)

    app_args = [
        encoding.decode_address(counterparty),
        penalty_reserve.to_bytes(8, "big"),
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

    client.send_transaction(signedTxn) # type: ignore

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
    aliceBalance = newGlobalState[b"alice_balance"]
    bobBalance = newGlobalState[b"bob_balance"]
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
    amountOfTransactions = amount // 700 # round down to nearest 700; increase budget by 700 per transaction
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

    print("Array of signed transactions: ", signedTxns)
    client.send_transactions(signedTxns)
    waitForTransaction(client, signedTxns[0].get_txid())


def signState(
        client: AlgodClient,
        appID: int,
        alice: Account,
        bob: Account,
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

    # data_hash = sha256(b"data").digest()
    data = b"[2000000900, 100]"
    alice_signed_bytes = signBytes(data, alice.getPrivateKey())
    bob_signed_bytes = signBytes(data, bob.getPrivateKey())
    alice_pub_key = alice.getAddress()
    bob_pub_key = bob.getAddress()

    # print(data.hex())
    # print(base64.b64decode(signed_bytes).hex())
    # print(encoding.decode_address(pub_key).hex(), "\n")
    # if verifyBytes(data, signed_bytes, pub_key):
    #     print("Signature is valid")
    # else:
    #     print("Signature is invalid")

    app_args = [
        b"loadState",
        # data: The data signed by the public key. Must evaluate to bytes.
        # sig: The proposed 64-byte signature of the data. Must evaluate to bytes.
        # key: The 32 byte public key that produced the signature. Must evaluate to bytes.
        data,
        base64.b64decode(alice_signed_bytes),
        encoding.decode_address(alice_pub_key),
    ]
    signStateAppTxn = transaction.ApplicationCallTxn(
        sender=alice.getAddress(),
        index=appID,
        on_complete=transaction.OnComplete.NoOpOC,
        app_args=app_args,
        sp=suggestedParams,
    )    
    increaseBudgetSignAndSendTransaction(client, appID, alice, signStateAppTxn, 1900) # 1x Sha3_256 a 130 + 2x Ed25519Verify a 1900
    
    # return Alice and Bob's balances
    newGlobalState = getAppGlobalState(client, appID)
    aliceBalance = newGlobalState[b"alice_balance"]
    try:
        bobBalance = newGlobalState[b"bob_balance"]
    except KeyError:
        bobBalance = 0
    return (aliceBalance, bobBalance)