from typing import List

from algosdk.v2client.algod import AlgodClient
from algosdk import account

from ..account import Account


accountList: List[Account] = []


def getTemporaryAccount(client: AlgodClient) -> Account:
    global accountList

    # if account list is empty
    if len(accountList) == 0:
        # crete 16 new accounts
        sks = [account.generate_account()[0] for i in range(16)]
        accountList = [Account(sk) for sk in sks]

        # TODO load accounts with funds

    return accountList.pop()
