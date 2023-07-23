#!/usr/bin/env python
import json
import pandas as pd
import matplotlib.pyplot as plt

ON_CHAIN_TRANSACTION_FEE = 1000
ON_CHAIN_TRANSACTION_DELAY = 3.3

with open("results/experiment1_adjusted.json", "r") as evaluation_results_file:
    evaluation_results = json.load(evaluation_results_file)

payment_data = evaluation_results["payments"]

df = pd.DataFrame(payment_data).T

# convert to numeric
df["transaction_fees"] = pd.to_numeric(df['transaction_fees'])
df["execution_time"] = pd.to_numeric(df['execution_time'])
df['payment_amount'] = df.index.astype(int)                                                             # add column with payment amount
df['fee_savings'] = df['payment_amount'] * ON_CHAIN_TRANSACTION_FEE - df['transaction_fees']            # add column fee savings
df['time_savings'] = ON_CHAIN_TRANSACTION_DELAY * df['payment_amount'] - df['execution_time']           # add column time savings

# sort the dataframe by payment amount
df = df.sort_values('payment_amount')


########## 1. plot transaction fees
plt.figure(figsize=(10, 6))
plt.plot(df['payment_amount'], df['fee_savings'], marker='o')

plt.xlabel('Amount of Payments')
plt.ylabel('Transaction Fee Savings (microAlgo)')
plt.legend(title=f"Dispute Window: {evaluation_results['dispute_window']}, Dispute Probability: {evaluation_results['dispute_probability']}")

plt.xticks(range(1, 21))
plt.axhline(0, color='gray', linestyle='--')
plt.grid(True)

plt.savefig('results/transaction_fees_savings_graph.pdf')
plt.show()


########## 2. plot execution time
plt.figure(figsize=(10, 6))
plt.plot(df['payment_amount'], df['time_savings'], marker='o')
# plt.plot(df['payment_amount'], df['execution_time'], marker='o')

plt.xlabel('Amount of Payments')
plt.ylabel('Execution Time Savings (seconds)')
# plt.ylabel('Execution Time (seconds)')
plt.legend(title=f"Dispute Window: {evaluation_results['dispute_window']}, Dispute Probability: {evaluation_results['dispute_probability']}")

plt.xticks(range(1, 21))
plt.axhline(0, color='gray', linestyle='--')
plt.grid(True)

plt.savefig('results/transaction_time_graph.pdf')
# plt.savefig('results/transaction_time_savings_graph.pdf')
plt.show()


# plot transaction fees for 10000 to 200000 payments
# plt.figure(figsize=(10, 6))