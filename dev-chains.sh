#/bin/bash -e

RELAYER_DIR="$GOPATH/src/github.com/cosmos/relayer"
RELAYER_CONF="$HOME/.relayer"
GAIA_CONF="$RELAYER_DIR/data"

# Ensure user understands what will be deleted
if ([[ -d $RELAYER_CONF ]] || [[ -d $GAIA_CONF ]]) && [[ ! "$1" == "skip" ]]; then
  read -p "$0 will delete $RELAYER_CONF and $GAIA_CONF folder. Do you wish to continue? (y/n): " -n 1 -r
  echo
  if [[ ! $REPLY =~ ^[Yy]$ ]]; then
      exit 1
  fi
fi

cd $RELAYER_DIR
rm -rf $RELAYER_CONF &> /dev/null
bash two-chains.sh "local" "skip"
bash config-relayer.sh "skip"
sleep 2
relayer tx full-path ibc0 ibc1 -d -o 3s

echo "Initial balances:"
relayer q balance ibc0
relayer q balance ibc1

# Send the n0token
relayer tx raw xfer ibc0 ibc1 10transfer/testchannelid/n0token true $(relayer keys show ibc1 testkey -a) -d

echo "Balances post-first-packet:"
relayer q balance ibc0
relayer q balance ibc1

# Send the n0token back
relayer tx raw xfer ibc1 ibc0 10transfer/testchannelid/n0token false $(relayer keys show ibc1 testkey -a) -d

echo "Balances post-second-packet:"
relayer q balance ibc0
relayer q balance ibc1
