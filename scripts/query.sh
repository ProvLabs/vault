#!/bin/bash

SIMD_BIN="./simapp/build/simd"
HOME_DIR="--home ./.vaulty"

# Pass all arguments from the script directly to the 'tx' command
# "$@" expands to all positional parameters, each as a separate word.
# This is crucial for commands like 'vault create-vault arg1 arg2'
CMD="$SIMD_BIN q $@ $HOME_DIR"
echo $CMD
$CMD
