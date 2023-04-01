
from algosdk.logic import get_application_address

from payment.operations import (
	createPaymentApp, 
	setupPaymentApp, 
	transact,
	signState,
)
from payment.util import (
	getBalances,
)
from payment.testing.setup import getAlgodClient
from payment.testing.resources import (
	getTemporaryAccount)


if __name__ == "__main__":
	client = getAlgodClient()

	print("Generating temporary accounts...")
	alice = getTemporaryAccount(client)
	bob = getTemporaryAccount(client)

	print("Alice (funding account):", alice.getAddress())
	print("Bob (channel counterparty account):", bob.getAddress(), "\n")

	aliceBalancesBefore = getBalances(client, alice.getAddress())
	print("Alice's balances:", aliceBalancesBefore)

	bobBalancesBefore = getBalances(client, alice.getAddress())
	print("Bob's balances:", bobBalancesBefore, "\n")

	print("Alice creates a payment smart contract with Bob's address...")
	appID = createPaymentApp(
		client=client,
		sender=alice,
		counterparty=bob.getAddress(),
		penalty_reserve=100_000,
	)
	print(
		"Done. The payment app ID is",
		appID,
		"and the payment account is",
		get_application_address(appID),
		"\n",
	)
	
	channelCapacity = 2_000_000_000
	print(f"Alice is setting up and funding the Payment Contract with {channelCapacity} microAlgos...")
	setupPaymentApp(
		client=client,
		appID=appID,
		funder=alice,
		channelCapacity=channelCapacity,
	)
	print("Done\n")

	# amount1 = 300
	# print(f"Alice is sending {amount1} microAlgos to Bob...")
	# (aliceBalance, bobBalance) = transact(
	# 	client=client,
	# 	appID=appID,
	# 	sender=alice,
	# 	amount=amount1,
	# )
	# print(f"Alice's balance is now {aliceBalance} microAlgos and Bob's balance is now {bobBalance} microAlgos\n")

	# amount2 = 50
	# print(f"Bob tries sending {amount2} microAlgos to Alice...")
	# try:
	# 	(aliceBalance, bobBalance) = transact(
	# 		client=client,
	# 		appID=appID,
	# 		sender=bob,
	# 		amount=amount2,
	# 	)
	# 	print(f"Alice's balance is still {aliceBalance} microAlgos and Bob's balance is still {bobBalance} microAlgos\n")
	# except Exception as e:
	# 	print("\n Bob's transaction failed:", e)

	# amount3 = 1_000_000_000
	# print(f"Alice is sending {amount3} microAlgos to Bob...")
	# (aliceBalance, bobBalance) = transact(
	# 	client=client,
	# 	appID=appID,
	# 	sender=alice,
	# 	amount=amount3,
	# )
	# print(f"Alice's balance is now {aliceBalance} microAlgos and Bob's balance is now {bobBalance} microAlgos\n")


	# print(f"Bob is sending {amount2} microAlgos to Alice...")
	# (aliceBalance, bobBalance) = transact(
	# 	client=client,
	# 	appID=appID,
	# 	sender=bob,
	# 	amount=amount2,
	# )
	# print(f"Alice's balance is now {aliceBalance} microAlgos and Bob's balance is now {bobBalance} microAlgos\n")


	print("Alice signs state data...")
	signState(client=client, appID=appID, sender=alice)


	



