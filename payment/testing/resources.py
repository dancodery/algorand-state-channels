from typing import List

from algosdk.v2client.algod import AlgodClient
from algosdk import transaction
from algosdk import account

from ..account import Account
from ..util import waitForTransaction
from .setup import getGenesisAccounts


FUNDING_AMOUNT = 10_000_000_000

accountList: List[Account] = []


def getTemporaryAccount(client: AlgodClient) -> Account:
    global accountList

    # if account list is empty
    if len(accountList) == 0:
        # crete 16 new accounts
        accountList = [Account() for i in range(16)]

        genesisAccounts = getGenesisAccounts()
        suggestedParams = client.suggested_params()

        txns: List[transaction.Transaction] = []
        # load accounts with funds
        for i, a in enumerate(accountList):
            fundingAccount = genesisAccounts[i % len(genesisAccounts)]
            txns.append(
                transaction.PaymentTxn(
                    sender=fundingAccount.getAddress(),
                    receiver=a.getAddress(),
                    amt=FUNDING_AMOUNT,
                    sp=suggestedParams,
                )
            )

        txns = transaction.assign_group_id(txns)
        signedTxns = [
            txn.sign(genesisAccounts[i % len(genesisAccounts)].getPrivateKey())
            for i, txn in enumerate(txns)
        ]

        client.send_transactions(signedTxns)

        waitForTransaction(client, signedTxns[0].get_txid())

    return accountList.pop()
