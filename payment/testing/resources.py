from typing import List, Optional

from algosdk.v2client.algod import AlgodClient
from algosdk import transaction
from algosdk import account

from ..account import Account
from ..util import waitForTransaction
from .setup import getGenesisAccounts


FUNDING_AMOUNT = 10_000_000_000

accountList: List[Account] = []


def getTemporaryAccount(client: AlgodClient, seed_phrase: Optional[str] = None) -> Account:
    if seed_phrase:
        account = Account.FromMnemonic(seed_phrase)
    else:
        account = Account()

    genesisAccounts = getGenesisAccounts()
    suggestedParams = client.suggested_params()

    txn = transaction.PaymentTxn(
        sender=genesisAccounts[0].getAddress(),
        receiver=account.getAddress(),
        amt=FUNDING_AMOUNT,
        sp=suggestedParams,
    )
    signedTxn = txn.sign(genesisAccounts[0].getPrivateKey())

    client.send_transaction(signedTxn)

    waitForTransaction(client, signedTxn.get_txid())

    return account

    ##### OLD CODE #####
    # global accountList

    # # if account list is empty
    # if len(accountList) == 0:
    #     # crete 16 new accounts
    #     accountList = [Account() for i in range(16)]

    #     genesisAccounts = getGenesisAccounts()
    #     suggestedParams = client.suggested_params()

    #     txns: List[transaction.Transaction] = []
    #     # load accounts with funds
    #     for i, a in enumerate(accountList):
    #         fundingAccount = genesisAccounts[i % len(genesisAccounts)]
    #         txns.append(
    #             transaction.PaymentTxn(
    #                 sender=fundingAccount.getAddress(),
    #                 receiver=a.getAddress(),
    #                 amt=FUNDING_AMOUNT,
    #                 sp=suggestedParams,
    #             )
    #         )

    #     txns = transaction.assign_group_id(txns)
    #     signedTxns = [
    #         txn.sign(genesisAccounts[i % len(genesisAccounts)].getPrivateKey())
    #         for i, txn in enumerate(txns)
    #     ]

    #     client.send_transactions(signedTxns)

    #     waitForTransaction(client, signedTxns[0].get_txid())

    # return accountList.pop()
