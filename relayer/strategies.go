package relayer

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Strategy determines which relayer strategy to use
// NOTE: To make a strategy available via config you need to add it to
// this switch statement
func Strategy(name string) RelayStrategy {
	switch name {
	case "naive":
		return NaiveRelayStrategy
	default:
		return nil
	}
}

// RelayStrategy describes the function signature for a relay strategy
type RelayStrategy func(src, dst *Chain) (*RelayMsgs, error)

// NaiveRelayStrategy returns the RelayMsgs that need to be run to relay between
// src and dst chains for all pending messages. Will also create or repair
// connections and channels
// TODO: Rethink this as we are planning on listening for events and strategies will be implemented there
func NaiveRelayStrategy(src, dst *Chain) (*RelayMsgs, error) {
	out := &RelayMsgs{Src: []sdk.Msg{}, Dst: []sdk.Msg{}}
	//   Return pending datagrams
	return out, nil
}
