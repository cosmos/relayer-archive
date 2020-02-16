package relayer

import (
	"fmt"
	"path"
	"time"

	"github.com/cosmos/cosmos-sdk/client/context"
	ckeys "github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keys"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/tendermint/tendermint/libs/log"

	sdk "github.com/cosmos/cosmos-sdk/types"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
)

// NewChain returns a new instance of Chain
// NOTE: It does not by default create the verifier. This needs a working connection
// and blocks running the app if NewChain does this by default.
func NewChain(key, chainID, rpcAddr, accPrefix string,
	counterparties []Counterparty, gas uint64, gasAdj float64,
	gasPrices string, defaultDenom, memo, homePath string,
	liteCacheSize int, trustingPeriod string, dir string, cdc *codec.Codec,
) (*Chain, error) {
	keybase, err := keys.NewKeyring(chainID, "test", keysDir(homePath), nil)
	if err != nil {
		return &Chain{}, err
	}

	client, err := rpcclient.NewHTTP(rpcAddr, "/websocket")
	if err != nil {
		return &Chain{}, err
	}

	gp, err := sdk.ParseDecCoins(gasPrices)
	if err != nil {
		return &Chain{}, err
	}

	tp, err := time.ParseDuration(trustingPeriod)
	if err != nil {
		return nil, fmt.Errorf("failed to parse duration (%s) for chain %s", trustingPeriod, chainID)
	}

	return &Chain{
		Key: key, ChainID: chainID, RPCAddr: rpcAddr, AccountPrefix: accPrefix, Counterparties: counterparties, Gas: gas,
		GasAdjustment: gasAdj, GasPrices: gp, DefaultDenom: defaultDenom, Memo: memo, Keybase: keybase,
		Client: client, Cdc: cdc, TrustingPeriod: tp, HomePath: homePath}, nil
}

// Chain represents the necessary data for connecting to and indentifying a chain and its counterparites
type Chain struct {
	Key            string         `yaml:"key"`
	ChainID        string         `yaml:"chain-id"`
	RPCAddr        string         `yaml:"rpc-addr"`
	AccountPrefix  string         `yaml:"account-prefix"`
	Counterparties []Counterparty `yaml:"counterparties"`
	Gas            uint64         `yaml:"gas,omitempty"`
	GasAdjustment  float64        `yaml:"gas-adjustment,omitempty"`
	GasPrices      sdk.DecCoins   `yaml:"gas-prices,omitempty"`
	DefaultDenom   string         `yaml:"default-denom,omitempty"`
	Memo           string         `yaml:"memo,omitempty"`
	TrustingPeriod time.Duration  `yaml:"trusting-period"`
	HomePath       string

	Keybase keys.Keybase
	Client  *rpcclient.HTTP
	Cdc     *codec.Codec

	address sdk.AccAddress
	logger  log.Logger
}

// Chains is a collection of Chain
type Chains []*Chain

// Exists Returns true if the chain is configured
func (c Chains) Exists(chainID string) bool {
	for _, chain := range c {
		if chainID == chain.ChainID {
			return true
		}
	}
	return false
}

type ErrAddressNotSet error

// GetChain returns the configuration for a given chain
func (c Chains) GetChain(chainID string) (*Chain, error) {
	for _, chain := range c {
		if chainID == chain.ChainID {
			addr, err := chain.GetAddress()
			if err != nil {
				return chain, fmt.Errorf("address failed to set: %w", err)
			}
			chain.address = addr
			return chain, nil
		}
	}
	return &Chain{}, fmt.Errorf("chain with ID %s is not configured", chainID)
}

// GetChains returns a map chainIDs to their chains
func (c Chains) GetChains(chainIDs ...string) (map[string]*Chain, error) {
	out := make(map[string]*Chain)
	for _, cid := range chainIDs {
		chain, err := c.GetChain(cid)
		if err != nil {
			return out, err
		}
		out[cid] = chain
	}
	return out, nil
}

func (c *Chain) BuildAndSignTx(datagram []sdk.Msg) ([]byte, error) {
	info, err := c.Keybase.Get(c.Key)
	if err != nil {
		return nil, err
	}

	// Fetch account and sequence numbers for the account
	acc, err := auth.NewAccountRetriever(c).GetAccount(info.GetAddress())
	if err != nil {
		return nil, err
	}

	txBldr := auth.NewTxBuilder(
		auth.DefaultTxEncoder(c.Cdc), acc.GetAccountNumber(),
		acc.GetSequence(), c.Gas, c.GasAdjustment, false, c.ChainID,
		c.Memo, sdk.NewCoins(), c.GasPrices).WithKeybase(c.Keybase)

	return txBldr.BuildAndSign(c.Key, ckeys.DefaultKeyPass, datagram)
}

// BroadcastTxCommit takes the marshaled transaction bytes and broadcasts them
func (c *Chain) BroadcastTxCommit(txBytes []byte) (sdk.TxResponse, error) {
	return context.CLIContext{Client: c.Client}.BroadcastTxCommit(txBytes)
}

// KeysDir returns the path to the keys for this chain
func keysDir(home string) string {
	return path.Join(home, "keys")
}

func liteDir(home string) string {
	return path.Join(home, "lite")
}

// GetAddress returns the sdk.AccAddress associated with the configred key
func (c *Chain) GetAddress() (sdk.AccAddress, error) {
	if c.address != nil {
		return c.address, nil
	}
	// Signing key for src chain
	srcAddr, err := c.Keybase.Get(c.Key)
	if err != nil {
		return nil, err
	}
	c.address = srcAddr.GetAddress()
	return srcAddr.GetAddress(), nil
}

// MustGetAddress used for brevity
func (c *Chain) MustGetAddress() sdk.AccAddress {
	srcAddr, err := c.Keybase.Get(c.Key)
	if err != nil {
		panic(err)
	}
	return srcAddr.GetAddress()
}
