package relayer

import (
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clientTypes "github.com/cosmos/cosmos-sdk/x/ibc/02-client/types"
	connState "github.com/cosmos/cosmos-sdk/x/ibc/03-connection/exported"
	connTypes "github.com/cosmos/cosmos-sdk/x/ibc/03-connection/types"
	chanState "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/exported"
	chanTypes "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/types"
	commitmentypes "github.com/cosmos/cosmos-sdk/x/ibc/23-commitment/types"
	"github.com/spf13/cobra"
)

var (
	defaultChainPrefix   = commitmentypes.NewMerklePrefix([]byte("ibc"))
	defaultIBCVersion    = "1.0.0"
	defaultIBCVersions   = []string{defaultIBCVersion}
	defaultUnbondingTime = time.Hour * 504 // 3 weeks in hours
)

// CreateClients creates clients for src on dst and dst on src given the configured paths
func (src *Chain) CreateClients(dst *Chain, cmd *cobra.Command) (err error) {
	clients := &RelayMsgs{Src: []sdk.Msg{}, Dst: []sdk.Msg{}}

	// Get latest src height for querying the client state
	srcH, err := src.GetLatestLiteHeight()
	if err != nil {
		return err
	}

	// Create client for dst on src if it doesn't exist
	var srcCs, dstCs clientTypes.StateResponse
	if srcCs, err = src.QueryClientState(srcH); err != nil {
		return err
	} else if srcCs.ClientState == nil {
		dstH, err := dst.UpdateLiteWithHeader()
		if err != nil {
			return err
		}
		clients.Src = append(clients.Src, src.PathEnd.CreateClient(dstH, src.GetTrustingPeriod(), src.MustGetAddress()))
	}
	// TODO: maybe log something here that the client has been created?

	// Get latest dst height for querying the client state
	dstH, err := dst.GetLatestLiteHeight()
	if err != nil {
		return err
	}

	// Create client for src on dst if it doesn't exist
	if dstCs, err = dst.QueryClientState(dstH); err != nil {
		return err
	} else if dstCs.ClientState == nil {
		srcH, err := src.UpdateLiteWithHeader()
		if err != nil {
			return err
		}
		clients.Dst = append(clients.Dst, dst.PathEnd.CreateClient(srcH, dst.GetTrustingPeriod(), dst.MustGetAddress()))
	}
	// TODO: maybe log something here that the client has been created?

	// Send msgs to both chains
	if err = clients.Send(src, dst, cmd); err != nil {
		return err
	}

	return nil
}

// CreateConnection runs the connection creation messages on timeout until they pass
// TODO: add max retries or something to this function
func (src *Chain) CreateConnection(dst *Chain, to time.Duration, cmd *cobra.Command) error {
	ticker := time.NewTicker(to)
	for ; true; <-ticker.C {
		connSteps, err := src.CreateConnectionStep(dst)
		if err != nil {
			return err
		}

		if !connSteps.Ready() {
			break
		}

		if err = connSteps.Send(src, dst, cmd); err != nil {
			return err
		}
	}

	return nil
}

// CreateConnectionStep returns the next set of messags for creating a channel
// with the given identifier between chains src and dst. If handshake hasn't started,
// CreateConnetionStep will start the handshake on src
func (src *Chain) CreateConnectionStep(dst *Chain) (*RelayMsgs, error) {
	out := &RelayMsgs{Src: []sdk.Msg{}, Dst: []sdk.Msg{}}

	if err := src.PathEnd.Validate(); err != nil {
		return nil, src.ErrCantSetPath(err)
	}

	if err := dst.PathEnd.Validate(); err != nil {
		return nil, dst.ErrCantSetPath(err)
	}

	hs, err := UpdatesWithHeaders(src, dst)
	if err != nil {
		return nil, err
	}

	// Query Connection data from src and dst
	// NOTE: We query connection at height - 1 because of the way tendermint returns
	// proofs the commit for height n is contained in the header of height n + 1
	var srcEnd, dstEnd connTypes.ConnectionResponse
	if srcEnd, err = src.QueryConnection(hs[src.ChainID].Height - 1); err != nil {
		return nil, err
	}
	if dstEnd, err = dst.QueryConnection(hs[src.ChainID].Height - 1); err != nil {
		return nil, err
	}

	// Query Client heights from chains src and dst
	var csSrc, csDst clientTypes.StateResponse
	if csSrc, err = src.QueryClientState(hs[src.ChainID].Height); err != nil {
		return nil, err
	}
	if csDst, err = dst.QueryClientState(hs[dst.ChainID].Height); err != nil {
		return nil, err
	}

	// Store the heights
	srcConsH, dstConsH := int64(csSrc.ClientState.GetLatestHeight()), int64(csDst.ClientState.GetLatestHeight())

	// Query the stored client consensus states at those heights on both src and dst
	var srcCons, dstCons clientTypes.ConsensusStateResponse
	if srcCons, err = src.QueryClientConsensusState(hs[src.ChainID].Height-1, srcConsH); err != nil {
		return nil, err
	}
	if dstCons, err = dst.QueryClientConsensusState(hs[dst.ChainID].Height-1, dstConsH); err != nil {
		return nil, err
	}

	switch {
	// Handshake hasn't been started on src or dst, relay `connOpenInit` to src
	case srcEnd.Connection.State == connState.UNINITIALIZED && dstEnd.Connection.State == connState.UNINITIALIZED:
		out.Src = append(out.Src, src.PathEnd.ConnInit(dst.PathEnd, src.MustGetAddress()))

	// Handshake has started on dst (1 stepdone), relay `connOpenTry` and `updateClient` on src
	case srcEnd.Connection.State == connState.UNINITIALIZED && dstEnd.Connection.State == connState.INIT:
		out.Src = append(out.Src,
			src.PathEnd.UpdateClient(hs[dst.ChainID], src.MustGetAddress()),
			src.PathEnd.ConnTry(dst.PathEnd, dstEnd, dstCons, dstConsH, src.MustGetAddress()),
		)

	// Handshake has started on src (1 step done), relay `connOpenTry` and `updateClient` on dst
	case srcEnd.Connection.State == connState.INIT && dstEnd.Connection.State == connState.UNINITIALIZED:
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(hs[src.ChainID], dst.MustGetAddress()),
			dst.PathEnd.ConnTry(src.PathEnd, srcEnd, srcCons, srcConsH, dst.MustGetAddress()),
		)

	// Handshake has started on src end (2 steps done), relay `connOpenAck` and `updateClient` to dst end
	case srcEnd.Connection.State == connState.TRYOPEN && dstEnd.Connection.State == connState.INIT:
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(hs[src.ChainID], dst.MustGetAddress()),
			dst.PathEnd.ConnAck(srcEnd, srcCons, srcConsH, dst.MustGetAddress()),
		)

	// Handshake has started on dst end (2 steps done), relay `connOpenAck` and `updateClient` to src end
	case srcEnd.Connection.State == connState.INIT && dstEnd.Connection.State == connState.TRYOPEN:
		out.Src = append(out.Src,
			src.PathEnd.UpdateClient(hs[dst.ChainID], src.MustGetAddress()),
			src.PathEnd.ConnAck(dstEnd, dstCons, dstConsH, src.MustGetAddress()),
		)

	// Handshake has confirmed on dst (3 steps done), relay `connOpenConfirm` and `updateClient` to src end
	case srcEnd.Connection.State == connState.TRYOPEN && dstEnd.Connection.State == connState.OPEN:
		out.Src = append(out.Src,
			src.PathEnd.UpdateClient(hs[dst.ChainID], src.MustGetAddress()),
			src.PathEnd.ConnConfirm(dstEnd, src.MustGetAddress()),
		)

	// Handshake has confirmed on src (3 steps done), relay `connOpenConfirm` and `updateClient` to dst end
	case srcEnd.Connection.State == connState.OPEN && dstEnd.Connection.State == connState.TRYOPEN:
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(hs[src.ChainID], dst.MustGetAddress()),
			dst.PathEnd.ConnConfirm(srcEnd, dst.MustGetAddress()),
		)
	}

	return out, nil
}

// CreateChannel runs the channel creation messages on timeout until they pass
// TODO: add max retries or something to this function
func (src *Chain) CreateChannel(dst *Chain, ordered bool, to time.Duration, cmd *cobra.Command) error {
	var order chanState.Order
	if ordered {
		order = chanState.ORDERED
	} else {
		order = chanState.UNORDERED
	}

	ticker := time.NewTicker(to)
	for ; true; <-ticker.C {
		chanSteps, err := src.CreateChannelStep(dst, order)
		if err != nil {
			return err
		}

		if !chanSteps.Ready() {
			break
		}

		if err = chanSteps.Send(src, dst, cmd); err != nil {
			return err
		}
	}

	return nil
}

// CreateChannelStep returns the next set of messages for creating a channel with given
// identifiers between chains src and dst. If the handshake hasn't started, then CreateChannelStep
// will begin the handshake on the src chain
func (src *Chain) CreateChannelStep(dst *Chain, ordering chanState.Order) (*RelayMsgs, error) {
	out := &RelayMsgs{Src: []sdk.Msg{}, Dst: []sdk.Msg{}}

	if err := src.PathEnd.Validate(); err != nil {
		return nil, src.ErrCantSetPath(err)
	}

	if err := dst.PathEnd.Validate(); err != nil {
		return nil, dst.ErrCantSetPath(err)
	}

	hs, err := UpdatesWithHeaders(src, dst)
	if err != nil {
		return nil, err
	}

	var srcEnd, dstEnd chanTypes.ChannelResponse
	if dstEnd, err = dst.QueryChannel(hs[dst.ChainID].Height - 1); err != nil {
		return nil, err
	}

	if srcEnd, err = src.QueryChannel(hs[src.ChainID].Height - 1); err != nil {
		return nil, err
	}

	switch {
	// Handshake hasn't been started on src or dst, relay `chanOpenInit` to src
	case srcEnd.Channel.State == chanState.UNINITIALIZED && dstEnd.Channel.State == chanState.UNINITIALIZED:
		out.Src = append(out.Src,
			src.PathEnd.ChanInit(dst.PathEnd, ordering, src.MustGetAddress()),
		)

	// Handshake has started on dst (1 step done), relay `chanOpenTry` and `updateClient` to src
	case srcEnd.Channel.State == chanState.UNINITIALIZED && dstEnd.Channel.State == chanState.INIT:
		out.Src = append(out.Src,
			src.PathEnd.UpdateClient(hs[dst.ChainID], src.MustGetAddress()),
			src.PathEnd.ChanTry(dst.PathEnd, dstEnd, src.MustGetAddress()),
		)

	// Handshake has started on src (1 step done), relay `chanOpenTry` and `updateClient` to dst
	case srcEnd.Channel.State == chanState.INIT && dstEnd.Channel.State == chanState.UNINITIALIZED:
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(hs[src.ChainID], dst.MustGetAddress()),
			dst.PathEnd.ChanTry(src.PathEnd, srcEnd, dst.MustGetAddress()),
		)

	// Handshake has started on src (2 steps done), relay `chanOpenAck` and `updateClient` to dst
	case srcEnd.Channel.State == chanState.TRYOPEN && dstEnd.Channel.State == chanState.INIT:
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(hs[src.ChainID], dst.MustGetAddress()),
			dst.PathEnd.ChanAck(srcEnd, dst.MustGetAddress()),
		)

	// Handshake has started on dst (2 steps done), relay `chanOpenAck` and `updateClient` to src
	case srcEnd.Channel.State == chanState.INIT && dstEnd.Channel.State == chanState.TRYOPEN:
		out.Src = append(out.Src,
			src.PathEnd.UpdateClient(hs[dst.ChainID], src.MustGetAddress()),
			src.PathEnd.ChanAck(dstEnd, src.MustGetAddress()),
		)

	// Handshake has confirmed on dst (3 steps done), relay `chanOpenConfirm` and `updateClient` to src
	case srcEnd.Channel.State == chanState.TRYOPEN && dstEnd.Channel.State == chanState.OPEN:
		out.Src = append(out.Src,
			src.PathEnd.UpdateClient(hs[dst.ChainID], src.MustGetAddress()),
			src.PathEnd.ChanConfirm(dstEnd, src.MustGetAddress()),
		)

	// Handshake has confirmed on src (3 steps done), relay `chanOpenConfirm` and `updateClient` to dst
	case srcEnd.Channel.State == chanState.OPEN && dstEnd.Channel.State == chanState.TRYOPEN:
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(hs[src.ChainID], dst.MustGetAddress()),
			dst.PathEnd.ChanConfirm(srcEnd, dst.MustGetAddress()),
		)
	}

	return out, nil
}
