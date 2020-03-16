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
relayer tx full-path ibc0 ibc1 -o 3s

echo
echo "Initial balances:"
echo "ibc0 balance: $(relayer q balance ibc0)"
echo "ibc1 balance: $(relayer q balance ibc1)"
echo 
echo "sending 10n0token from ibc0 to ibc1..."
relayer tx raw xfer ibc0 ibc1 10transfer/ibconexfer/n0token true $(relayer keys show ibc1 testkey -a) -d
echo
echo "ibc0 balance: $(relayer q balance ibc0)"
echo "ibc1 balance: $(relayer q balance ibc1)"
echo
echo "sending 10n0token back to ibc0 from ibc1..."
relayer tx raw xfer ibc1 ibc0 10transfer/ibconexfer/n0token false $(relayer keys show ibc0 testkey -a) -d
echo
echo "ibc0 balance: $(relayer q balance ibc0)"
echo "ibc1 balance: $(relayer q balance ibc1)"