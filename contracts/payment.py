#!/usr/bin/env python3
from pyteal import *
import os

FILENAME = "payment"


def approval_program():
    # Keys for the global key-value state of the smart contract
    # Convenient to define keys here s.t. they can be reused when needed
    alice_key = Bytes("alice")
    bob_key = Bytes("bob")
    penalty_reserve = Bytes("penalty_reserve")
    alice_balance = Int(0)
    bob_balance = Int(0)

    def rebalance(sender_balance, recipient_balance, payment_amount):
        Assert((App.globalGet(sender_balance) - payment_amount) >= App.globalGet(penalty_reserve)),

        App.globalPut(sender_balance, App.globalGet(sender_balance) - Int(payment_amount)),
        App.globalPut(recipient_balance, App.globalGet(recipient_balance) + Int(payment_amount)),
        Approve()

    # Initialization
    on_create = Seq(
        # Set alice to sender of initial tx
        App.globalPut(alice_key, Txn.sender()),
        App.globalPut(bob_key, Txn.application_args[0]),
        App.globalPut(penalty_reserve, Txn.application_args[1]),
        Approve()
    )

    amount = Txn.application_args[0],
    on_funding = Seq(
        Assert(Txn.sender() == App.globalGet(alice_key)),

        # Alice funds the channel
        InnerTxnBuilder.Begin(),
        InnerTxnBuilder.SetFields(
            {
                TxnField.type_enum: TxnType.Payment,
                TxnField.sender: Txn.sender(),
                TxnField.amount: amount,
                TxnField.receiver: Global.current_application_address(),
            }
        ),
        InnerTxnBuilder.Submit(),

        # update Alice's balance
        App.globalPut(alice_balance, amount),
        Approve(),
    )

    on_transacting = Seq(
        Assert(Or(Txn.sender() == App.globalGet(alice_key),
                  Txn.sender() == App.globalGet(bob_key))),
        If(Txn.sender() == App.globalGet(alice_key)).Then(
            rebalance(alice_balance, bob_balance, amount)
        ),

        If(Txn.sender() == App.globalGet(bob_key)).Then(
            rebalance(bob_balance, alice_balance, amount)
        )
    )

    # NoOp call
    # Implements payment logic
    # methods, send transaction, load latest state, settle dispute
    on_call_method = Txn.application_args[0]
    on_call = Seq(
        Cond(
            [on_call_method == Bytes("fund"), on_funding],
            [on_call_method == Bytes("transact"), on_transacting],
        )
    )

    # Only the owner should be allowed to delete the application
    on_delete = Cond(
        [Txn.sender() == App.globalGet(alice_key), Approve()]
    )

    program = Cond(
        [Txn.application_id() == Int(0), on_create],
        [Txn.on_completion() == OnComplete.DeleteApplication, on_delete],
        [Txn.on_completion() == OnComplete.UpdateApplication, Reject()],  # Update app is not implemented yet
        [Txn.on_completion() == OnComplete.CloseOut, Reject()],  # CloseOut is not implemented yet
        [Txn.on_completion() == OnComplete.OptIn, Reject()],  # CloseOut is not implemented yet
        [Txn.on_completion() == OnComplete.NoOp, on_call],
    )
    return program


# Clear state program always succeeds and does nothing else
def clear_state_program():
    return Approve()


# Compiles PyTEAL code to TEAL, .teal files are placed into ./build
if __name__ == "__main__":
    os.makedirs("build", exist_ok=True)
    approval_file = "build/{filename}_approval.teal".format(FILENAME)
    with open(approval_file, "w") as f:
        compiled = compileTeal(approval_program(), mode=Mode.Application, version=5)
        f.write(compiled)

    clear_state_file = "build/{filename}_clear_state.teal".format(FILENAME)
    with open(clear_state_file, "w") as f:
        compiled = compileTeal(clear_state_program(), mode=Mode.Application, version=5)
        f.write(compiled)
