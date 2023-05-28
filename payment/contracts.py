#!/usr/bin/env python3
from pyteal import *
from algosdk import encoding
import os

FILENAME = "payment"


def approval_program():
	# Keys for the global key-value state of the smart contract
	# Convenient to define keys here s.t. they can be reused when needed
	alice_address = Bytes("alice_address")      # creator and funder of the smart contract
	bob_address = Bytes("bob_address")          # counterparty
	
	alice_balance = Bytes("alice_balance")      # part of state; value variable during execution
	bob_balance = Bytes("bob_balance")          # part of state; value variable during execution

	penalty_reserve = Bytes("penalty_reserve")  # used to penalize expired transaction commitments

	# alice_pubkey = Bytes("alice_pubkey")        # public key of the creator
	# bob_pubkey = Bytes("bob_pubkey")            # public key of the counterparty
	# TODO add dispute timeout

	# for debugging purposes
	alice_signed = Bytes("alice_signed")        # used to check if Alice has signed the transaction

	signed_data = Bytes("signed_data")
	signature = Bytes("signature")
	public_key = Bytes("public_key")

	# closes the channel and pays out the funds to the respective parties
	@Subroutine(TealType.none)
	def closeAccountTo(beneficiary: Expr) -> Expr:
		Seq(
			# pay Alice
			InnerTxnBuilder.Begin(),
			InnerTxnBuilder.SetFields(
				{
					TxnField.type_enum: TxnType.Payment,
					TxnField.amount: App.globalGet(alice_balance) - Global.min_txn_fee(),
					TxnField.receiver: App.globalGet(alice_address),
				}
			),
			InnerTxnBuilder.Submit(),

			# pay Bob
			InnerTxnBuilder.Begin(),
			InnerTxnBuilder.SetFields(
				{
					TxnField.type_enum: TxnType.Payment,
					TxnField.amount: App.globalGet(bob_balance) - Global.min_txn_fee(),
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
		# The arguments contain bob address, and penalty reserve
		Assert(Txn.application_args.length() == Int(2)),
		# Set alice to sender of initial tx
		App.globalPut(alice_address, Txn.sender()),
		App.globalPut(bob_address, Txn.application_args[0]),
		App.globalPut(penalty_reserve, Btoi(Txn.application_args[1])),
		Approve()
	)

	# on_register_public_key = Seq(
	#     Assert(
	#         Or(
	#             Txn.sender() == App.globalGet(alice_address)),
	#             Txn.sender() == App.globalGet(bob_address),
	#     ),

	#     If(Txn.sender() == App.globalGet(alice_address)).Then(
	#         App.globalPut(alice_pubkey, Txn.application_args[0])
	#     ),
	#     If(Txn.sender() == App.globalGet(bob_address)).Then(
	#         App.globalPut(bob_pubkey, Txn.application_args[0])
	#     ),
	#     Approve(),
	# )
 
	# funds the smart contract with the amount sent in the first transaction of the group
	funding_txn_index = Txn.group_index() - Int(1)
	on_funding = Seq(
		Assert(
			And(
				Gtxn[funding_txn_index].sender() == Txn.sender(),
				Gtxn[funding_txn_index].sender() == App.globalGet(alice_address),
				Gtxn[funding_txn_index].type_enum() == TxnType.Payment,
				Gtxn[funding_txn_index].amount() > App.globalGet(penalty_reserve) + Global.min_txn_fee(),
				Gtxn[funding_txn_index].receiver() == Global.current_application_address(),
			)
		),

		App.globalPut(alice_balance, Gtxn[funding_txn_index].amount()),
		Approve(),
	)

	# transacts amount of funds from one party to the other
	amount = Btoi(Txn.application_args[1])
	on_transacting = Seq(
		Assert(Or(Txn.sender() == App.globalGet(alice_address),
				  Txn.sender() == App.globalGet(bob_address))),
		If(Txn.sender() == App.globalGet(alice_address)).Then(
			rebalance(alice_balance, bob_balance, amount)
		),

		If(Txn.sender() == App.globalGet(bob_address)).Then(
			rebalance(bob_balance, alice_balance, amount)
		),
		Approve(),
	)

	on_increaseBudget = Seq(
		Approve(),
	)

	# loads the signed state of the smart contract
	on_loadState = Seq(
		Assert(
			Or(
				Txn.sender() == App.globalGet(alice_address),
				Txn.sender() == App.globalGet(bob_address)
			)   
		),    

		# App.globalPut(signed_data, Txn.application_args[1]),
		# App.globalPut(signature, Txn.application_args[2]),  # 64 bytes
		# App.globalPut(public_key, Txn.application_args[3]),  # 32 bytes

		If (Txn.sender() == App.globalGet(alice_address)).Then(
				# data: The data signed by the public key. Must evaluate to bytes.
				# sig: The proposed 64-byte signature of the data. Must evaluate to bytes.
				# key: The 32 byte public key that produced the signature. Must evaluate to bytes.


			# https://pyteal.readthedocs.io/en/stable/crypto.html
			If (Ed25519Verify_Bare(	# cost: 1900, takes 3 arguments: data, sig, key
						Txn.application_args[1], # [alice_balance, bob_balance]
						Txn.application_args[2],
						App.globalGet(alice_address), # has to be comitted on chain
					)
			).Then(
				App.globalPut(alice_signed, Int(1))
			)
		),

		# verify signature
		# EcdsaVerify(bytes("data"), Txn.application_args[0]),
		Approve(),  
	)

	# NoOp call
	# Implements payment logic
	# methods, send transaction, load latest state, settle dispute
	on_call_method = Txn.application_args[0]
	on_call = Seq(
		Cond(
			[on_call_method == Bytes("fund"), on_funding],
			[on_call_method == Bytes("transact"), on_transacting],
			[on_call_method == Bytes("increaseBudget"), on_increaseBudget],
			[on_call_method == Bytes("loadState"), on_loadState],
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
    approval_file = f"build/{FILENAME}_approval.teal"
    with open(approval_file, "w") as f:
        compiled = compileTeal(approval_program(), mode=Mode.Application, version=7)
        f.write(compiled)

    clear_state_file = f"build/{FILENAME}_clear_state.teal"
    with open(clear_state_file, "w") as f:
        compiled = compileTeal(clear_state_program(), mode=Mode.Application, version=7)
        f.write(compiled)
