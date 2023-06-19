import base64
import hashlib
from nacl.signing import SigningKey, VerifyKey
from nacl.exceptions import BadSignatureError
from typing import List, Dict, Any, Optional, Union
from base64 import b64decode
from algosdk.v2client.algod import AlgodClient
from algosdk import encoding

from pyteal import compileTeal, Mode, Expr


class PendingTxnResponse:
    def __init__(self, response: Dict[str, Any]) -> None:
        self.poolError: str = response["pool-error"]
        self.txn: Dict[str, Any] = response["txn"]

        self.applicationIndex: Optional[int] = response.get("application-index")
        self.assetIndex: Optional[int] = response.get("asset-index")
        self.closeRewards: Optional[int] = response.get("close-rewards")
        self.closingAmount: Optional[int] = response.get("closing-amount")
        self.confirmedRound: Optional[int] = response.get("confirmed-round")
        self.globalStateDelta: Optional[Any] = response.get("global-state-delta")
        self.localStateDelta: Optional[Any] = response.get("local-state-delta")
        self.receiverRewards: Optional[int] = response.get("receiver-rewards")
        self.senderRewards: Optional[int] = response.get("sender-rewards")

        self.innerTxns: List[Any] = response.get("inner-txns", [])
        self.logs: List[bytes] = [b64decode(l) for l in response.get("logs", [])]


def waitForTransaction(
        client: AlgodClient,
        txID: str,
        timeout: int = 10
) -> PendingTxnResponse:
    lastStatus = client.status()
    lastRound = lastStatus["last-round"]
    startRound = lastRound

    while lastRound < startRound + timeout:
        pending_txn = client.pending_transaction_info(txID)

        if pending_txn.get("confirmed-round", 0) > 0:
            return PendingTxnResponse(pending_txn)
        
        if pending_txn["pool-error"]:
            raise Exception("Pool error: {}".format(pending_txn["pool-error"]))
        
        lastStatus = client.status_after_block(lastRound + 1)

        lastRound += 1

    raise Exception(
        "Transaction {} not comfirmed after {} rounds".format(txID, timeout)
    )


def fullyCompileContract(client: AlgodClient, contract: Expr) -> bytes:
    teal = compileTeal(contract, mode=Mode.Application, version=7)
    response = client.compile(teal)
    return b64decode(response["result"])


def decodeState(stateArray: List[Any]) -> Dict[bytes, Union[int, bytes]]:
    state: Dict[bytes, Union[int, bytes]] = dict()

    for pair in stateArray:
        key = b64decode(pair["key"])

        value = pair["value"]
        valueType = value["type"]

        if valueType == 2:
            # value is uint64
            value = value.get("uint", 0)
        elif valueType == 1:
            # value is byte array
            value = b64decode(value.get("bytes", ""))
        else:
            raise Exception(f"Unexpected state type: {valueType}")

        state[key] = value

    return state


def getAppGlobalState(
    client: AlgodClient, appID: int
) -> Dict[bytes, Union[int, bytes]]:
    appInfo = client.application_info(appID)
    return decodeState(appInfo["params"]["global-state"])


def getBalances(client: AlgodClient, account: str) -> Dict[int, int]:
    balaces: Dict[int, int] = dict()

    accountInfo = client.account_info(account)

    # set key 0 to Algo balance
    balaces[0] = accountInfo["amount"]

    assets: List[Dict[str, Any]] = accountInfo.get("assets", [])
    for asset in assets:
        assetID = asset["asset-id"]
        amount = asset["amount"]
        balaces[assetID] = amount

    return balaces

def sha3_256(data):
    """
    Compute SHA3-256 hash of the input data.

    Args:
        data (bytes): data to hash

    Returns:
        bytes: hash
    """
    return hashlib.sha3_256(data).digest()

def signBytes(to_sign, private_key):
    """
    Sign arbitrary bytes without prepending with "MX".

    Args:
        to_sign (bytes): bytes to sign
        private_key (str): base64 encoded private key

    Returns:
        str: base64 signature
    """
    private_key = base64.b64decode(private_key)
    signing_key = SigningKey(private_key[: 32])
    signed = signing_key.sign(to_sign)
    signature = base64.b64encode(signed.signature).decode()
    return signature    


def verifyBytes(message, signature, public_key):
    """
    Verify the signature of a message.

    Args:
        message (bytes): message that was signed
        signature (str): base64 signature
        public_key (str): base32 address

    Returns:
        bool: whether or not the signature is valid
    """
    verify_key = VerifyKey(encoding.decode_address(public_key))
    try:
        verify_key.verify(message, base64.b64decode(signature))
        return True
    except (BadSignatureError, ValueError, TypeError):
        return False
