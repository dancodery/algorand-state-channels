#!/usr/bin/env python
import json
import pandas as pd
import matplotlib.pyplot as plt

ON_CHAIN_TRANSACTION_FEE = 1000
ON_CHAIN_TRANSACTION_DELAY = 3.3

with open("results/experiment1_adjusted.json", "r") as evaluation_results_file:
    evaluation_results = json.load(evaluation_results_file)

payment_data = evaluation_results["payments"]

df_all = pd.DataFrame(payment_data).T

# convert to numeric
df_all["transaction_fees"] = pd.to_numeric(df_all['transaction_fees'])
df_all["execution_time"] = pd.to_numeric(df_all['execution_time'])
df_all['payment_amount'] = df_all.index.astype(int)                                                             # add column with payment amount
df_all['fee_savings'] = df_all['payment_amount'] * ON_CHAIN_TRANSACTION_FEE - df_all['transaction_fees']            # add column fee savings
df_all['time_savings'] = ON_CHAIN_TRANSACTION_DELAY * df_all['payment_amount'] - df_all['execution_time']           # add column time savings

# sort the dataframe by payment amount
df_all = df_all.sort_values('payment_amount')

df_1_20 = df_all[df_all['payment_amount'].between(1, 20)]
df_10_200 = df_all[df_all['payment_amount'].between(10, 200) & (df_all['payment_amount'] % 10 == 0)]


########## 1. plot transaction fees for 1 to 20 payments
plt.figure(figsize=(10, 6))
plt.plot(df_1_20['payment_amount'], df_1_20['fee_savings'], marker='o')

plt.xlabel('Amount of Payments')
plt.ylabel('Transaction Fee Savings (microAlgo)')
plt.legend(title=f"Dispute Window: {evaluation_results['dispute_window']}, Dispute Probability: {evaluation_results['dispute_probability']}")

plt.xticks(range(1, 21))
plt.axhline(0, color='gray', linestyle='--')
plt.grid(True)

plt.savefig('results/transaction_fees_savings_graph_1_20.pdf')
plt.show()


########## 2. plot execution time for 1 to 20 payments
plt.figure(figsize=(10, 6))
plt.plot(df_1_20['payment_amount'], df_1_20['time_savings'], marker='o')
# plt.plot(df['payment_amount'], df['execution_time'], marker='o')

plt.xlabel('Amount of Payments')
plt.ylabel('Execution Time Savings (seconds)')
# plt.ylabel('Execution Time (seconds)')
plt.legend(title=f"Dispute Window: {evaluation_results['dispute_window']}, Dispute Probability: {evaluation_results['dispute_probability']}")

plt.xticks(range(1, 21))
plt.axhline(0, color='gray', linestyle='--')
plt.grid(True)

# plt.savefig('results/transaction_time_graph.pdf')
plt.savefig('results/transaction_time_savings_graph_1_20.pdf')
plt.show()


########## 3. plot transaction fees for 10 to 200 payments
plt.figure(figsize=(10, 6))
plt.plot(df_10_200['payment_amount'], df_10_200['fee_savings'], marker='o')

plt.xlabel('Amount of Payments')
plt.ylabel('Transaction Fee Savings (microAlgo)')
plt.legend(title=f"Dispute Window: {evaluation_results['dispute_window']}, Dispute Probability: {evaluation_results['dispute_probability']}")

plt.xticks(range(10, 201, 10))
plt.axhline(0, color='gray', linestyle='--')
plt.grid(True)

plt.savefig('results/transaction_fees_savings_graph_10_200.pdf')
plt.show()


########## 4. plot execution time for 10 to 200 payments
plt.figure(figsize=(10, 6))
plt.plot(df_10_200['payment_amount'], df_10_200['time_savings'], marker='o')

plt.xlabel('Amount of Payments')
plt.ylabel('Execution Time Savings (seconds)')
plt.legend(title=f"Dispute Window: {evaluation_results['dispute_window']}, Dispute Probability: {evaluation_results['dispute_probability']}")

plt.xticks(range(10, 201, 10))
plt.axhline(0, color='gray', linestyle='--')
plt.grid(True)

plt.savefig('results/transaction_time_savings_graph_10_200.pdf')
plt.show()

