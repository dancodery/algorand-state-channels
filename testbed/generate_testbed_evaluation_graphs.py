#!/usr/bin/env python
import json
import pandas as pd
import matplotlib.pyplot as plt


with open("results/experiment1_adjusted.json", "r") as evaluation_results_file:
    evaluation_results = json.load(evaluation_results_file)

payment_data = evaluation_results["payments"]

df = pd.DataFrame(payment_data).T

# convert to numeric
df["transaction_fees"] = pd.to_numeric(df['transaction_fees'])
df["execution_time"] = pd.to_numeric(df['execution_time'])

# add numerical column for payment amount
df['payment_amount'] = df.index.astype(int)

# sort the dataframe by payment amount
df = df.sort_values('payment_amount')

# plot transaction fees
plt.figure(figsize=(10, 6))
plt.plot(df['payment_amount'], df['transaction_fees'], marker='o')

plt.xlabel('Amount of Payments')
plt.ylabel('Transaction Fees Savings')
plt.legend(title=f"Dispute Window: {evaluation_results['dispute_window']}, Dispute Probability: {evaluation_results['dispute_probability']}")

plt.xticks(range(1, 21))

plt.savefig('results/transaction_fees_savings_graph.pdf')
plt.show()

# plot execution time
plt.figure(figsize=(10, 6))
plt.plot(df['payment_amount'], df['execution_time'], marker='o')

plt.xlabel('Amount of Payments')
plt.ylabel('Execution Time (seconds)')
plt.legend(title=f"Dispute Window: {evaluation_results['dispute_window']}, Dispute Probability: {evaluation_results['dispute_probability']}")

plt.xticks(range(1, 21))

plt.savefig('results/transaction_time_savings_graph.pdf')
plt.show()
