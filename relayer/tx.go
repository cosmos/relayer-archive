package relayer

import (
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clientTypes "github.com/cosmos/cosmos-sdk/x/ibc/02-client/types"
	connState "github.com/cosmos/cosmos-sdk/x/ibc/03-connection/exported"
	chanState "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/exported"
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

	// Create client for dst on src if it doesn't exist
	var srcCs, dstCs clientTypes.StateResponse
	if srcCs, err = src.QueryClientState(); err != nil {
		return err
	} else if srcCs.ClientState == nil {
		dstH, err := dst.UpdateLiteWithHeader()
		if err != nil {
			return err
		}
		clients.Src = append(clients.Src, src.PathEnd.CreateClient(dstH, src.GetTrustingPeriod(), src.MustGetAddress()))
	}
	// TODO: maybe log something here that the client has been created?

	// Create client for src on dst if it doesn't exist
	if dstCs, err = dst.QueryClientState(); err != nil {
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

	scid, dcid := src.ChainID, dst.ChainID

	// Query Connection data from src and dst
	// NOTE: We query connection at height - 1 because of the way tendermint returns
	// proofs the commit for height n is contained in the header of height n + 1
	conn, err := QueryConnectionPair(src, dst, hs[scid].Height-1, hs[dcid].Height-1)
	if err != nil {
		return nil, err
	}

	// NOTE: We query connection at height - 1 because of the way tendermint returns
	// proofs the commit for height n is contained in the header of height n + 1
	cs, err := QueryClientStatePair(src, dst)
	if err != nil {
		return nil, err
	}

	// Store the heights
	srcConsH, dstConsH := int64(cs[scid].ClientState.GetLatestHeight()), int64(cs[dcid].ClientState.GetLatestHeight())

	// NOTE: We query connection at height - 1 because of the way tendermint returns
	// proofs the commit for height n is contained in the header of height n + 1
	cons, err := QueryClientConsensusStatePair(src, dst, hs[scid].Height-1, hs[dcid].Height-1, srcConsH, dstConsH)
	if err != nil {
		return nil, err
	}

	switch {
	// Handshake hasn't been started on src or dst, relay `connOpenInit` to src
	case conn[scid].Connection.State == connState.UNINITIALIZED && conn[dcid].Connection.State == connState.UNINITIALIZED:
		out.Src = append(out.Src, src.PathEnd.ConnInit(dst.PathEnd, src.MustGetAddress()))

	// Handshake has started on dst (1 stepdone), relay `connOpenTry` and `updateClient` on src
	case conn[scid].Connection.State == connState.UNINITIALIZED && conn[dcid].Connection.State == connState.INIT:
		out.Src = append(out.Src,
			src.PathEnd.UpdateClient(hs[dcid], src.MustGetAddress()),
			src.PathEnd.ConnTry(dst.PathEnd, conn[dcid], cons[dcid], dstConsH, src.MustGetAddress()),
		)

	// Handshake has started on src (1 step done), relay `connOpenTry` and `updateClient` on dst
	case conn[scid].Connection.State == connState.INIT && conn[dcid].Connection.State == connState.UNINITIALIZED:
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(hs[scid], dst.MustGetAddress()),
			dst.PathEnd.ConnTry(src.PathEnd, conn[scid], cons[scid], srcConsH, dst.MustGetAddress()),
		)

	// Handshake has started on src end (2 steps done), relay `connOpenAck` and `updateClient` to dst end
	case conn[scid].Connection.State == connState.TRYOPEN && conn[dcid].Connection.State == connState.INIT:
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(hs[scid], dst.MustGetAddress()),
			dst.PathEnd.ConnAck(conn[scid], cons[scid], srcConsH, dst.MustGetAddress()),
		)

	// Handshake has started on dst end (2 steps done), relay `connOpenAck` and `updateClient` to src end
	case conn[scid].Connection.State == connState.INIT && conn[dcid].Connection.State == connState.TRYOPEN:
		out.Src = append(out.Src,
			src.PathEnd.UpdateClient(hs[dcid], src.MustGetAddress()),
			src.PathEnd.ConnAck(conn[dcid], cons[dcid], dstConsH, src.MustGetAddress()),
		)

	// Handshake has confirmed on dst (3 steps done), relay `connOpenConfirm` and `updateClient` to src end
	case conn[scid].Connection.State == connState.TRYOPEN && conn[dcid].Connection.State == connState.OPEN:
		out.Src = append(out.Src,
			src.PathEnd.UpdateClient(hs[dcid], src.MustGetAddress()),
			src.PathEnd.ConnConfirm(conn[dcid], src.MustGetAddress()),
		)

	// Handshake has confirmed on src (3 steps done), relay `connOpenConfirm` and `updateClient` to dst end
	case conn[scid].Connection.State == connState.OPEN && conn[dcid].Connection.State == connState.TRYOPEN:
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(hs[scid], dst.MustGetAddress()),
			dst.PathEnd.ConnConfirm(conn[scid], dst.MustGetAddress()),
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

	scid, dcid := src.ChainID, dst.ChainID

	hs, err := UpdatesWithHeaders(src, dst)
	if err != nil {
		return nil, err
	}

	chans, err := QueryChannelPair(src, dst, hs[scid].Height-1, hs[dcid].Height-1)
	if err != nil {
		return nil, err
	}

	switch {
	// Handshake hasn't been started on src or dst, relay `chanOpenInit` to src
	case chans[scid].Channel.State == chanState.UNINITIALIZED && chans[dcid].Channel.State == chanState.UNINITIALIZED:
		out.Src = append(out.Src,
			src.PathEnd.ChanInit(dst.PathEnd, ordering, src.MustGetAddress()),
		)

	// Handshake has started on dst (1 step done), relay `chanOpenTry` and `updateClient` to src
	case chans[scid].Channel.State == chanState.UNINITIALIZED && chans[dcid].Channel.State == chanState.INIT:
		out.Src = append(out.Src,
			src.PathEnd.UpdateClient(hs[dcid], src.MustGetAddress()),
			src.PathEnd.ChanTry(dst.PathEnd, chans[dcid], src.MustGetAddress()),
		)

	// Handshake has started on src (1 step done), relay `chanOpenTry` and `updateClient` to dst
	case chans[scid].Channel.State == chanState.INIT && chans[dcid].Channel.State == chanState.UNINITIALIZED:
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(hs[scid], dst.MustGetAddress()),
			dst.PathEnd.ChanTry(src.PathEnd, chans[scid], dst.MustGetAddress()),
		)

	// Handshake has started on src (2 steps done), relay `chanOpenAck` and `updateClient` to dst
	case chans[scid].Channel.State == chanState.TRYOPEN && chans[dcid].Channel.State == chanState.INIT:
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(hs[scid], dst.MustGetAddress()),
			dst.PathEnd.ChanAck(chans[scid], dst.MustGetAddress()),
		)

	// Handshake has started on dst (2 steps done), relay `chanOpenAck` and `updateClient` to src
	case chans[scid].Channel.State == chanState.INIT && chans[dcid].Channel.State == chanState.TRYOPEN:
		out.Src = append(out.Src,
			src.PathEnd.UpdateClient(hs[dcid], src.MustGetAddress()),
			src.PathEnd.ChanAck(chans[dcid], src.MustGetAddress()),
		)

	// Handshake has confirmed on dst (3 steps done), relay `chanOpenConfirm` and `updateClient` to src
	case chans[scid].Channel.State == chanState.TRYOPEN && chans[dcid].Channel.State == chanState.OPEN:
		out.Src = append(out.Src,
			src.PathEnd.UpdateClient(hs[dcid], src.MustGetAddress()),
			src.PathEnd.ChanConfirm(chans[dcid], src.MustGetAddress()),
		)

	// Handshake has confirmed on src (3 steps done), relay `chanOpenConfirm` and `updateClient` to dst
	case chans[scid].Channel.State == chanState.OPEN && chans[dcid].Channel.State == chanState.TRYOPEN:
		out.Dst = append(out.Dst,
			dst.PathEnd.UpdateClient(hs[scid], dst.MustGetAddress()),
			dst.PathEnd.ChanConfirm(chans[scid], dst.MustGetAddress()),
		)
	}

	return out, nil
}
