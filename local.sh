alias simd=./simapp/build/simd

for arg in "$@"
do
    case $arg in
        -r|--reset)
        rm -rf .vaulty
        shift
        ;;
    esac
done

if ! [ -f .vaulty/data/priv_validator_state.json ]; then
  simd init validator --chain-id "vaulty-1" --home .vaulty &> /dev/null

  simd keys add validator --home .vaulty --keyring-backend test &> /dev/null
  simd genesis add-genesis-account validator 1000000ustake --home .vaulty --keyring-backend test
  simd genesis add-genesis-account provlabs17trszny8xuz2vn8vnqx6edt5c72nq4hprcxwcz 1000000uusdc --home .vaulty --keyring-backend test

  TEMP=.vaulty/genesis.json
  touch $TEMP && jq '.app_state.staking.params.bond_denom = "ustake"' .vaulty/config/genesis.json > $TEMP && mv $TEMP .vaulty/config/genesis.json

  simd genesis gentx validator 1000000ustake --chain-id "vaulty-1" --home .vaulty --keyring-backend test 
  simd genesis collect-gentxs --home .vaulty 

  sed -i '' 's/timeout_commit = "5s"/timeout_commit = "1s"/g' .vaulty/config/config.toml
fi

simd start --home .vaulty
