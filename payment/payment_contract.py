#!/usr/bin/env python3
from pyteal import *
from algosdk import encoding
import os

FILENAME = "payment"


def approval_program():
	app_id = Bytes("app_id")						# uint: stores the app_id of the smart contract
													# , needed for protecting against replay attacks

	alice_address = Bytes("alice_address")			# byte_slice: creator and funder of the smart contract												
	bob_address = Bytes("bob_address")				# byte_slice: counterparty
	
	timeout = Bytes("timeout")              		# uint: part of general state; value fixed during execution

	latest_state_timestamp = Bytes("latest_timestamp")			# uint: part of general state; when the latest transaction was signed by alice and bob
	total_deposit = Bytes("total_deposit")						# uint: part of application specific state; value set by funding transaction
	latest_alice_balance = Bytes("latest_alice_balance")      	# uint: part of application specific state; value variable during execution
	latest_bob_balance = Bytes("latest_bob_balance")          	# uint: part of application specific state; value variable during execution

	penalty_reserve = Bytes("penalty_reserve")					# uint: used to penalize expired transaction commitments
	dispute_window = Bytes("dispute_window")					# uint: window in which a dispute can be raised

	# closes the channel and pays out the funds to the respective parties
	@Subroutine(TealType.none)
	def closeAccountTo(beneficiary: Expr) -> Expr:
		Seq(
			# pay Alice
			InnerTxnBuilder.Begin(),
			InnerTxnBuilder.SetFields(
				{
					TxnField.type_enum: TxnType.Payment,
					TxnField.amount: App.globalGet(latest_alice_balance) - Global.min_txn_fee(),
					TxnField.receiver: App.globalGet(alice_address),
				}
			),
			InnerTxnBuilder.Submit(),

			# pay Bob
			InnerTxnBuilder.Begin(),
			InnerTxnBuilder.SetFields(
				{
					TxnField.type_enum: TxnType.Payment,
					TxnField.amount: App.globalGet(latest_bob_balance) - Global.min_txn_fee(),
					TxnField.receiver: App.globalGet(bob_address),
				}
			),
			InnerTxnBuilder.Submit(),
		)

		# pay leftover to
		return If(Balance(Global.current_application_address()) != Int(0)).Then(
			Seq(
				InnerTxnBuilder.Begin(),
				InnerTxnBuilder.SetFields(
					{
						TxnField.type_enum: TxnType.Payment,
						TxnField.close_remainder_to: beneficiary,
					}
				),
				InnerTxnBuilder.Submit(),
			)
		)

	# rebalances the channel by moving funds from one party to the other
	def rebalance(sender_balance, recipient_balance, payment_amount):
		return Seq(
			Assert((App.globalGet(sender_balance) - payment_amount) >= App.globalGet(penalty_reserve)),

			App.globalPut(sender_balance, App.globalGet(sender_balance) - payment_amount),
			App.globalPut(recipient_balance, App.globalGet(recipient_balance) + payment_amount),
			Approve()
		)

	# Initialization
	on_create = Seq(
		# can be called by anyone
		#
		Assert(Txn.application_args.length() == Int(3)),
		# Set alice to sender of initial tx
		App.globalPut(app_id, Global.current_application_id()),
		App.globalPut(alice_address, Txn.sender()),
		App.globalPut(bob_address, Txn.application_args[0]),
		App.globalPut(penalty_reserve, Btoi(Txn.application_args[1])),
		App.globalPut(dispute_window, Btoi(Txn.application_args[2])),
		Approve()
	)
 
	# funds the smart contract with the amount sent in the first transaction of the group
	funding_txn_index = Txn.group_index() - Int(1)
	on_funding = Seq(
		# can only be called by alice
		Assert(
			And(
				Gtxn[funding_txn_index].sender() == Txn.sender(),
				Gtxn[funding_txn_index].sender() == App.globalGet(alice_address),
				Gtxn[funding_txn_index].type_enum() == TxnType.Payment,
				Gtxn[funding_txn_index].amount() > App.globalGet(penalty_reserve) + Global.min_txn_fee(),
				Gtxn[funding_txn_index].receiver() == Global.current_application_address(),
			)
		),

		App.globalPut(latest_alice_balance, Gtxn[funding_txn_index].amount()),
		App.globalPut(total_deposit, Gtxn[funding_txn_index].amount()),
		Approve(),
	)

	# transacts amount of funds from one party to the other
	# amount = Btoi(Txn.application_args[1])
	# on_transacting = Seq(
	# 	Assert(Or(Txn.sender() == App.globalGet(alice_address),
	# 			  Txn.sender() == App.globalGet(bob_address))),
	# 	If(Txn.sender() == App.globalGet(alice_address)).Then(
	# 		rebalance(latest_alice_balance, latest_bob_balance, amount)
	# 	),

	# 	If(Txn.sender() == App.globalGet(bob_address)).Then(
	# 		rebalance(latest_bob_balance, latest_alice_balance, amount)
	# 	),
	# 	Approve(),
	# )

	on_increaseBudget = Seq(
		# can be called by anyone
		Approve(),
	)

	algorand_port = Txn.application_args[1]
	alice_balance = Txn.application_args[2]
	bob_balance = Txn.application_args[3]
	timestamp = Txn.application_args[4]
	alice_signature = Txn.application_args[5] # 64 bytes
	bob_signature = Txn.application_args[6] # 64 bytes
	hash = Sha3_256( # cost: 130, takes 1 argument: data
			Concat(
				algorand_port,
				Bytes(","),
				Itob(App.globalGet(app_id)),
				Bytes(","),
				alice_balance, 
				Bytes(","),
				bob_balance,
				Bytes(","),
				timestamp
			)) # in bytes "alice_balance, bob_balance"
	on_initiateChannelClosing = Seq(
		# can only be called by alice or bob
		Assert(
			Or(
				Txn.sender() == App.globalGet(alice_address),
				Txn.sender() == App.globalGet(bob_address)
			)
		),
		If (And(
				# https://pyteal.readthedocs.io/en/stable/crypto.html
				Ed25519Verify_Bare(	# cost: 1900, takes 3 arguments: data, sig 64 bytes, key 32 bytes
					hash,
					alice_signature, # signature
					App.globalGet(alice_address), # has to be comitted on chain
				),
				Ed25519Verify_Bare(
					hash,
					bob_signature,
					App.globalGet(bob_address),
				),
				Btoi(alice_balance) + Btoi(bob_balance) == App.globalGet(total_deposit),
			)
		).Then(
			App.globalPut(timeout, Global.round() + App.globalGet(dispute_window)),		# set timeout
			App.globalPut(latest_state_timestamp, Btoi(timestamp)), 					# store state timestamp
			App.globalPut(latest_alice_balance, Btoi(alice_balance)),					# store latest balances of alice
			App.globalPut(latest_bob_balance, Btoi(bob_balance)),						# store latest balances of bob
		),
		Approve(),
	)

	algorand_port = Txn.application_args[1]
	alice_balance = Txn.application_args[2]
	bob_balance = Txn.application_args[3]
	timestamp = Txn.application_args[4]
	alice_signature = Txn.application_args[5] # 64 bytes
	bob_signature = Txn.application_args[6] # 64 bytes
	on_raiseDispute = Seq(
		If (And(
				# https://pyteal.readthedocs.io/en/stable/crypto.html
				Ed25519Verify_Bare(	# cost: 1900, takes 3 arguments: data, sig 64 bytes, key 32 bytes
					hash,
					alice_signature, # signature
					App.globalGet(alice_address), # has to be comitted on chain
				),
				Ed25519Verify_Bare(
					hash,
					bob_signature,
					App.globalGet(bob_address),
				),
				Btoi(alice_balance) + Btoi(bob_balance) == App.globalGet(total_deposit),
				Btoi(latest_state_timestamp) < Btoi(timestamp),
			)
		).Then(
		# 	# App.globalPut(latest_state_timestamp, Btoi(timestamp)), 						# store state timestamp
		# 	# App.globalPut(timeout, Global.round() + App.globalGet(dispute_window)),			# set timeout
		# 	# App.globalPut(latest_alice_balance, Btoi(alice_balance)),				# store latest balances of alice
			App.globalPut(latest_bob_balance, Btoi(bob_balance)),					# store latest balances of bob


		),
		Approve(),  
	)


	on_finalizeChannelClosing = Seq(
		# can be called by anyone
		# 
		If (Global.round() > App.globalGet(timeout)).Then(
			# timeout has passed
			# send funds to alice
			InnerTxnBuilder.Begin(),
			InnerTxnBuilder.SetFields(
				{
					TxnField.type_enum: TxnType.Payment,
					TxnField.amount: App.globalGet(latest_alice_balance) - Global.min_txn_fee(),
					TxnField.receiver: App.globalGet(alice_address),
				}
			),
			InnerTxnBuilder.Submit(),

			InnerTxnBuilder.Begin(),
			InnerTxnBuilder.SetFields(
				{
					TxnField.type_enum: TxnType.Payment,
					TxnField.amount: App.globalGet(latest_bob_balance) - Global.min_txn_fee(),
					TxnField.receiver: App.globalGet(bob_address),
				}
			),
			InnerTxnBuilder.Submit(),
		),
		Approve(),
	)

	# NoOp call
	# Implements payment logic
	# methods, send transaction, load latest state, settle dispute
	on_call_method = Txn.application_args[0]
	on_call = Seq(
		Cond(
			[on_call_method == Bytes("fund"), on_funding],
			# [on_call_method == Bytes("transact"), on_transacting],
			[on_call_method == Bytes("increaseBudget"), on_increaseBudget],
			[on_call_method == Bytes("initiateChannelClosing"), on_initiateChannelClosing],
			[on_call_method == Bytes("raiseDispute"), on_raiseDispute],
			[on_call_method == Bytes("finalizeChannelClosing"), on_finalizeChannelClosing],
			# [on_call_method == Bytes("updateState"), on_updateState],
			# [on_call_method == Bytes("updateStateWithTimeout"), on_updateStateWithTimeout],

		)
	)

	# Only the owner is allowed to delete the application
	on_delete = Seq(
		# [Txn.sender() == App.globalGet(alice_address), Approve()]
		Reject(),
	)

	program = Cond(
		[Txn.application_id() == Int(0), on_create],                        # run once on creation
		[Txn.on_completion() == OnComplete.UpdateApplication, Reject()],    # Update app is not implemented
		[Txn.on_completion() == OnComplete.CloseOut, Reject()],             # CloseOut is not implemented
		# [Txn.on_completion() == OnComplete.OptIn, on_register_public_key],                # OptIn is not implemented yet
		[Txn.on_completion() == OnComplete.NoOp, on_call],                  # calls on_call for fund, transact, updatestate, etc.
		[Txn.on_completion() == OnComplete.DeleteApplication, on_delete],   # calls on_delete
	)
	return program


# Clear state program always succeeds and does nothing else
def clear_state_program():
	return Approve()


# Compiles PyTEAL code to TEAL, .teal files are placed into ./build
if __name__ == "__main__":
    os.makedirs("build", exist_ok=True)
    approval_file = f"build_contracts/{FILENAME}_approval.teal"
    with open(approval_file, "w") as f:
        compiled = compileTeal(approval_program(), mode=Mode.Application, version=7)
        f.write(compiled)

    clear_state_file = f"build_contracts/{FILENAME}_clear_state.teal"
    with open(clear_state_file, "w") as f:
        compiled = compileTeal(clear_state_program(), mode=Mode.Application, version=7)
        f.write(compiled)
