from payment.testing.setup import getAlgodClient
from payment.testing.resources import (
    getTemporaryAccount)

if __name__ == "__main__":
    client = getAlgodClient()

    print("Generating temporary accounts...")
    alice = getTemporaryAccount(client)
    bob = getTemporaryAccount(client)

    print("Alice (funding account):", alice.getAddress())
    print("Bob (channel partner account):", bob.getAddress(), "\n")

