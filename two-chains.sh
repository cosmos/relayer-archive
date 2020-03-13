#!/bin/bash

GAIA_DIR="$GOPATH/src/github.com/cosmos/gaia"
RELAYER_DIR="$GOPATH/src/github.com/cosmos/relayer"
GAIA_BRANCH=ibc-alpha
GAIA_CONF=$(mktemp -d)

read -p "two-chains.sh will delete your ~/.relayer folder. Do you wish to continue? (y/n): " -n 1 -r
if [[ ! $REPLY =~ ^[Yy]$ ]]
then
    exit 1
fi

sleep 1

echo "Killing existing gaiad instances..."
killall gaiad

set -e

echo "Building Gaia..."
cd $GAIA_DIR
git checkout $GAIA_BRANCH &> /dev/null
make install &> /dev/null

echo "Building Relayer..."
cd $RELAYER_DIR
go build -o $GOBIN/relayer main.go

echo "Generating relayer configurations..."
rm -rf $HOME/.relayer
relayer config init
relayer chains add -f demo/ibc0.json
relayer chains add -f demo/ibc1.json
relayer paths add ibc0 ibc1 -f demo/path.json

echo "Generating gaia configurations..."
cd $GAIA_CONF && mkdir ibc-testnets && cd ibc-testnets
echo -e "\n" | gaiad testnet -o ibc0 --v 1 --chain-id ibc0 --node-dir-prefix n --keyring-backend test &> /dev/null
echo -e "\n" | gaiad testnet -o ibc1 --v 1 --chain-id ibc1 --node-dir-prefix n --keyring-backend test &> /dev/null

if [ "$(uname)" = "Linux" ]; then
  sed -i 's/"leveldb"/"goleveldb"/g' ibc0/n0/gaiad/config/config.toml
  sed -i 's/"leveldb"/"goleveldb"/g' ibc1/n0/gaiad/config/config.toml
  sed -i 's#"tcp://0.0.0.0:26656"#"tcp://0.0.0.0:26556"#g' ibc1/n0/gaiad/config/config.toml
  sed -i 's#"tcp://0.0.0.0:26657"#"tcp://0.0.0.0:26557"#g' ibc1/n0/gaiad/config/config.toml
  sed -i 's#"localhost:6060"#"localhost:6061"#g' ibc1/n0/gaiad/config/config.toml
  sed -i 's#"tcp://127.0.0.1:26658"#"tcp://127.0.0.1:26558"#g' ibc1/n0/gaiad/config/config.toml
else
  sed -i '' 's/"leveldb"/"goleveldb"/g' ibc0/n0/gaiad/config/config.toml
  sed -i '' 's/"leveldb"/"goleveldb"/g' ibc1/n0/gaiad/config/config.toml
  sed -i '' 's#"tcp://0.0.0.0:26656"#"tcp://0.0.0.0:26556"#g' ibc1/n0/gaiad/config/config.toml
  sed -i '' 's#"tcp://0.0.0.0:26657"#"tcp://0.0.0.0:26557"#g' ibc1/n0/gaiad/config/config.toml
  sed -i '' 's#"localhost:6060"#"localhost:6061"#g' ibc1/n0/gaiad/config/config.toml
  sed -i '' 's#"tcp://127.0.0.1:26658"#"tcp://127.0.0.1:26558"#g' ibc1/n0/gaiad/config/config.toml
fi;

gaiacli config --home ibc0/n0/gaiacli/ chain-id ibc0 &> /dev/null
gaiacli config --home ibc1/n0/gaiacli/ chain-id ibc1 &> /dev/null
gaiacli config --home ibc0/n0/gaiacli/ output json &> /dev/null
gaiacli config --home ibc1/n0/gaiacli/ output json &> /dev/null
gaiacli config --home ibc0/n0/gaiacli/ node http://localhost:26657 &> /dev/null
gaiacli config --home ibc1/n0/gaiacli/ node http://localhost:26557 &> /dev/null

echo "Starting Gaiad instances..."
gaiad --home ibc0/n0/gaiad start --pruning=nothing > ibc0.log 2>&1 &
gaiad --home ibc1/n0/gaiad start --pruning=nothing > ibc1.log 2>&1 & 
# gaiad --home ibc0/n0/gaiad start --pruning=nothing --tx_index.index_all_tags=true > ibc0.log 2>&1 &
# gaiad --home ibc1/n0/gaiad start --pruning=nothing --tx_index.index_all_tags=true > ibc1.log 2>&1 & 

echo "Set the following env to use gaiacli with the running chains:"
echo 
echo "export GAIA=$GAIA_CONF"
echo
echo "Key Seeds for importing into gaiacli or relayer:"
SEED0=$(jq -r '.secret' ibc0/n0/gaiacli/key_seed.json)
SEED1=$(jq -r '.secret' ibc1/n0/gaiacli/key_seed.json)
echo "  ibc0 -> $SEED0"
echo "  ibc1 -> $SEED1"
echo
echo "NOTE: Below are account addresses for each chain. They are also validator addresses:"
echo "  ibc0 address: $(relayer keys restore ibc0 testkey "$SEED0" -a)"
echo "  ibc1 address: $(relayer keys restore ibc1 testkey "$SEED1" -a)"
echo
echo "Creating configured path between ibc0 and ibc1..."
sleep 8
relayer lite init ibc0 -f
relayer lite init ibc1 -f
sleep 5
relayer tx full-path ibc0 ibc1
echo
echo "Ready to send msgs..."
