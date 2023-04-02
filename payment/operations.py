from typing import Tuple
import base64

from algosdk.v2client.algod import AlgodClient
from algosdk.future import transaction
from algosdk.logic import get_application_address
from algosdk import encoding, util

from .account import Account
from .contracts import approval_program, clear_state_program
from .util import (
    waitForTransaction,
    fullyCompileContract,
    getAppGlobalState,
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

    globalSchema = transaction.StateSchema(num_uints=4, num_byte_slices=2)
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
    client.send_transaction(signedTransactAppTxn)
    waitForTransaction(client, signedTransactAppTxn.get_txid())

    # return Alice and Bob's balances
    newGlobalState = getAppGlobalState(client, appID)
    aliceBalance = newGlobalState[b"alice_balance"]
    bobBalance = newGlobalState[b"bob_balance"]
    return (aliceBalance, bobBalance)

def signState(
        client: AlgodClient,
        appID: int,
        sender: Account,
) -> None:
    """Signs the current state of the payment app.
    Args:
        client: An algod client.
        appID: The ID of the payment app.
        sender: The account that will sign the state.
    Returns:
        None
    """
    suggestedParams = client.suggested_params()

    data = b"data"
    signedBytes = util.sign_bytes(data, sender.getPrivateKey())
    pub_key = sender.getPublicKey().encode()

    base64_string = "FS8tEk1_Qah59zXfvmv6ZZsIDM1a4mgNbT1XmrUHqWI"

    # Add padding characters to the end of the string
    while len(base64_string) % 4 != 0:
        base64_string += "="

    print(base64_string)

    app_args = [
        b"loadState",
        base64.b64decode("78or4J-Q7ZXUCQ4Snc6rPO34-HxVhi3ih7QSO7dIoIewSt6aEZWoXTIn2rYSYaLf641bbRDP1QhJ72MguHWeAw"),
        base64.b64decode("FS8tEk1_Qah59zXfvmv6ZZsIDM1a4mgNbT1XmrUHqWI=="),
    ]

    signStateAppTxn = transaction.ApplicationCallTxn(
        sender=sender.getAddress(),
        index=appID,
        on_complete=transaction.OnComplete.NoOpOC,
        app_args=app_args,
        sp=suggestedParams,
    )
    signedSignStateAppTxn = signStateAppTxn.sign(sender.getPrivateKey())
    client.send_transaction(signedSignStateAppTxn)
    waitForTransaction(client, signedSignStateAppTxn.get_txid())

    print("State updated")