#!/bin/bash

SIMD_BIN="./simapp/build/simd"
KEYRING_BACKEND="--keyring-backend test"
HOME_DIR="--home ./.vaulty"
CHAIN_ID="--chain-id vaulty-1"
YES_FLAG="--yes"

# Pass all arguments from the script directly to the 'tx' command
# "$@" expands to all positional parameters, each as a separate word.
# This is crucial for commands like 'vault create-vault arg1 arg2'
CMD="$SIMD_BIN tx $@ $KEYRING_BACKEND $HOME_DIR $CHAIN_ID $YES_FLAG"
echo $CMD
COMMAND_OUTPUT=$($CMD)
TX_HASH=$(echo "$COMMAND_OUTPUT" | grep 'txhash:' | awk '{ print $2 }' | tr -d '\r')

if [ -z "$TX_HASH" ]; then
  echo "ERROR: Failed to obtain transaction hash from submission output."
  echo "Full submission output:"
  echo "$COMMAND_OUTPUT"
  exit 1 # Indicate failure
fi

# Call the wait-tx.sh script with the extracted transaction hash
# The exit status of wait-tx.sh will be propagated as the exit status of tx.sh
./scripts/wait-tx.sh "$TX_HASH"
