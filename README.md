# Vault Module

A Provenance Blockchain module for managing vaults.

## Table of Contents

- [Vault Module](#vault-module)
  - [Table of Contents](#table-of-contents)
  - [Prerequisites](#prerequisites)
  - [Local Development](#local-development)
    - [Starting the Chain](#starting-the-chain)
    - [Resetting the Chain](#resetting-the-chain)
  - [Interacting with the Chain](#interacting-with-the-chain)
    - [Transactions](#transactions)
    - [Queries](#queries)
    - [Helper Scripts](#helper-scripts)
  - [Protobuf](#protobuf)
    - [Generating Protobuf Files](#generating-protobuf-files)

## Prerequisites

Before you begin, ensure you have the following installed:

- [Go](https://go.dev/doc/install) (version 1.21 or later)
- [Buf CLI](https://buf.build/docs/installation)
- [jq](https://jqlang.github.io/jq/download/)

## Local Development

The `local.sh` script provides a convenient way to run a single-node blockchain for development and testing purposes.

### Starting the Chain

To start the local chain, run:

```bash
./local.sh
```

The first time you run this, it will:

1. Initialize a new chain with `chain-id: vaulty-1`.
2. Create three accounts: `validator`, `account-1`, and `account-2`.
3. Set up genesis accounts with initial token balances.
4. Create a genesis transaction for the validator.
5. Collect the genesis transactions.
6. Start the chain.

Subsequent runs will simply start the existing chain.

### Resetting the Chain

To start from a clean slate, use the `-r` or `--reset` flag. This will remove the existing chain data before initializing and starting a new one.

```bash
./local.sh --reset
```

## Interacting with the Chain

The `scripts` directory contains several wrapper scripts to simplify interaction with the local chain via the `simd` command-line interface.

### Transactions

The `scripts/tx.sh` script is a wrapper for `simd tx ...`. It automatically includes necessary flags like `--keyring-backend`, `--home`, `--chain-id`, and `--yes`. It also waits for the transaction to be included in a block and reports the result.

**Example:** To create a vault (the exact command may vary based on module implementation):

```bash
# Get the address for account-1
ACCOUNT_1_ADDR=$(./scripts/get-key.sh account-1)

# Example: Create a public vault
./scripts/tx.sh vault create-vault $ACCOUNT_1_ADDR $ACCOUNT_1_ADDR false --from account-1
```

The script will print the command being executed, and upon completion, it will output the transaction result.

### Queries

The `scripts/query.sh` script is a wrapper for `simd q ...`. It simplifies running queries against the chain.

**Example:** To query the list of vaults (command may vary):

```bash
./scripts/query.sh vault list-vaults
```

### Helper Scripts

- `./scripts/get-key.sh <key_name>`: Retrieves the bech32 address for a given key name from the test keyring.

    ```bash
    ./scripts/get-key.sh account-2
    ```

- `./scripts/get-marker-address.sh <denom>`: Retrieves the address for a marker with the given denom. This is useful when interacting with the `marker` module.

    ```bash
    # Assuming a 'hotdogcoin' marker exists
    ./scripts/get-marker-address.sh hotdogcoin
    ```

- `./scripts/wait-tx.sh <tx_hash>`: Waits for a given transaction hash to be confirmed on-chain. This is used internally by `tx.sh`.

## Protobuf

This project uses [Buf](https://buf.build) to manage Protobuf files and generate Go code. The Protobuf definitions are located in the `proto/` directory, and the generated Go code is placed in the `api/` directory.

A `Makefile` at the root of the repository provides commands to manage Protobuf files.

### Generating Protobuf Files

To regenerate the Go code from the `.proto` files, run:

```bash
make proto-all
```
