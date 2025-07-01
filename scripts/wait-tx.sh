#!/bin/bash

# --- Configuration for Waiting ---
MAX_RETRIES=3       # Maximum number of times to retry querying the transaction
SLEEP_INTERVAL=1     # Seconds to wait between retry attempts

# --- Common Binary and Home Directory ---
# These should be consistent across your scripts or passed as arguments
SIMD_BIN="./simapp/build/simd"
HOME_DIR="--home ./.vaulty" # Necessary for 'simd q tx' if it depends on home

# --- Script Logic ---

# Check if a transaction hash was provided
if [ -z "$1" ]; then
  echo "Usage: $0 <tx_hash>"
  echo "Error: No transaction hash provided to wait for."
  exit 1
fi

TX_HASH="$1"
# echo "Waiting for transaction $TX_HASH to be included in a block (max retries: $MAX_RETRIES, sleep: ${SLEEP_INTERVAL}s)..."

retries=0
while [ $retries -lt $MAX_RETRIES ]; do
  # Query the transaction status
  # We include HOME_DIR as 'simd q tx' might need to know the chain's configuration
  QUERY_OUTPUT=$($SIMD_BIN q tx "$TX_HASH" $HOME_DIR 2>&1) # Redirect stderr to stdout

  # Check if the query was successful and contains "code: 0"
  if echo "$QUERY_OUTPUT" | grep -q "code: 0"; then
    echo "SUCCESS: Transaction $TX_HASH was successful!"
    echo "$QUERY_OUTPUT"
    exit 0 # Exit with success code
  elif echo "$QUERY_OUTPUT" | grep -q "code:"; then
    # Transaction found, but with a non-zero code (meaning it failed)
    echo "ERROR: Transaction $TX_HASH failed on-chain!"
    echo "$QUERY_OUTPUT"
    exit 1 # Exit with failure code
  else
    sleep $SLEEP_INTERVAL
    retries=$((retries + 1))
  fi
done

echo "ERROR: Transaction $TX_HASH was not successful or confirmed after $MAX_RETRIES attempts."
echo "Last query output:"
echo "$QUERY_OUTPUT"
exit 1 # Exit with failure code
