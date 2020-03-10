package relayer

import (
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clientTypes "github.com/cosmos/cosmos-sdk/x/ibc/02-client/types"
	connState "github.com/cosmos/cosmos-sdk/x/ibc/03-connection/exported"
	connTypes "github.com/cosmos/cosmos-sdk/x/ibc/03-connection/types"
	chanState "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/exported"
	chanTypes "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/types"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint/types"
	commitmentypes "github.com/cosmos/cosmos-sdk/x/ibc/23-commitment/types"
)

var (
	defaultChainPrefix   = commitmentypes.NewMerklePrefix([]byte("ibc"))
	defaultIBCVersion    = "1.0.0"
	defaultIBCVersions   = []string{defaultIBCVersion}
	defaultUnbondingTime = time.Hour * 504 // 3 weeks in hours
)

// CreateConnection creates a connection between two chains given src and dst client IDs
func (src *Chain) CreateConnection(dst *Chain, timeout time.Duration) error {
	ticker := time.NewTicker(timeout)
	for ; true; <-ticker.C {
		msgs, err := src.CreateConnectionStep(dst)
		if err != nil {
			return err
		}

		if !msgs.Ready() {
			break
		}

		// Submit the transactions to src chain
		srcRes, err := src.SendMsgs(msgs.Src)
		if err != nil {
			return err
		}
		src.logger.Info(srcRes.String())

		// Submit the transactions to dst chain
		dstRes, err := dst.SendMsgs(msgs.Dst)
		if err != nil {
			return err
		}
		src.logger.Info(dstRes.String())
	}

	return nil
}

// CreateConnectionStep returns the next set of messags for creating a channel
// with the given identifier between chains src and dst. If handshake hasn't started,
// CreateConnetionStep will start the handshake on src
func (src *Chain) CreateConnectionStep(dst *Chain) (*RelayMsgs, error) {
	out := &RelayMsgs{Src: []sdk.Msg{}, Dst: []sdk.Msg{}}

	if err := src.PathEnd.Validate(CONNPATH); err != nil {
		return nil, src.ErrPathNotSet(CONNPATH, err)
	}

	if err := dst.PathEnd.Validate(CONNPATH); err != nil {
		return nil, dst.ErrPathNotSet(CONNPATH, err)
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
	if csSrc, err = src.QueryClientState(); err != nil {
		return nil, err
	}
	if csDst, err = dst.QueryClientState(); err != nil {
		return nil, err
	}

	// Store the heights
	srcConsH, dstConsH := int64(csSrc.ClientState.GetLatestHeight()), int64(csDst.ClientState.GetLatestHeight())

	// Query the stored client consensus states at those heights on both src and dst
	var srcCons, dstCons clientTypes.ConsensusStateResponse
	if srcCons, err = src.QueryClientConsensusState(hs[dst.ChainID].Height, srcConsH); err != nil {
		return nil, err
	}
	if dstCons, err = dst.QueryClientConsensusState(hs[src.ChainID].Height, dstConsH); err != nil {
		return nil, err
	}

	switch {
	// Handshake hasn't been started on src or dst, relay `connOpenInit` to src
	case srcEnd.Connection.State == connState.UNINITIALIZED && dstEnd.Connection.State == connState.UNINITIALIZED:
		out.Src = append(out.Src, src.ConnInit(dst))

	// Handshake has started on dst (1 stepdone), relay `connOpenTry` and `updateClient` on src
	case srcEnd.Connection.State == connState.UNINITIALIZED && dstEnd.Connection.State == connState.INIT:
		out.Src = append(out.Src, src.UpdateClient(hs[dst.ChainID]),
			src.ConnTry(dst, dstEnd, dstCons, dstConsH))

	// Handshake has started on src (1 step done), relay `connOpenTry` and `updateClient` on dst
	case srcEnd.Connection.State == connState.INIT && dstEnd.Connection.State == connState.UNINITIALIZED:
		out.Dst = append(out.Dst, dst.UpdateClient(hs[src.ChainID]),
			dst.ConnTry(src, srcEnd, srcCons, srcConsH))

	// Handshake has started on src end (2 steps done), relay `connOpenAck` and `updateClient` to dst end
	case srcEnd.Connection.State == connState.TRYOPEN && dstEnd.Connection.State == connState.INIT:
		out.Dst = append(out.Dst, dst.UpdateClient(hs[src.ChainID]),
			dst.ConnAck(srcEnd, srcCons, srcConsH))

	// Handshake has started on dst end (2 steps done), relay `connOpenAck` and `updateClient` to src end
	case srcEnd.Connection.State == connState.INIT && dstEnd.Connection.State == connState.TRYOPEN:
		out.Src = append(out.Src, src.UpdateClient(hs[dst.ChainID]),
			src.ConnAck(dstEnd, dstCons, dstConsH))

	// Handshake has confirmed on dst (3 steps done), relay `connOpenConfirm` and `updateClient` to src end
	case srcEnd.Connection.State == connState.TRYOPEN && dstEnd.Connection.State == connState.OPEN:
		out.Src = append(out.Src, src.UpdateClient(hs[dst.ChainID]),
			src.ConnConfirm(dstEnd, dstCons, dstConsH))

	// Handshake has confirmed on src (3 steps done), relay `connOpenConfirm` and `updateClient` to dst end
	case srcEnd.Connection.State == connState.OPEN && dstEnd.Connection.State == connState.TRYOPEN:
		out.Dst = append(out.Dst, dst.UpdateClient(hs[src.ChainID]),
			dst.ConnConfirm(srcEnd, srcCons, srcConsH))
	}

	return out, nil
}

// CreateChannel creates a connection between two chains given src and dst client IDs
func (src *Chain) CreateChannel(dst *Chain, timeout time.Duration, ordering chanState.Order) error {
	ticker := time.NewTicker(timeout)
	for ; true; <-ticker.C {
		msgs, err := src.CreateChannelStep(dst, ordering)
		if err != nil {
			return err
		}

		if !msgs.Ready() {
			break
		}

		// Submit the transactions to src chain
		srcRes, err := src.SendMsgs(msgs.Src)
		if err != nil {
			return err
		}
		src.logger.Info(srcRes.String())

		// Submit the transactions to dst chain
		dstRes, err := dst.SendMsgs(msgs.Dst)
		if err != nil {
			return err
		}
		src.logger.Info(dstRes.String())
	}

	return nil
}

// CreateChannelStep returns the next set of messages for creating a channel with given
// identifiers between chains src and dst. If the handshake hasn't started, then CreateChannelStep
// will begin the handshake on the src chain
func (src *Chain) CreateChannelStep(dst *Chain, ordering chanState.Order) (*RelayMsgs, error) {
	out := &RelayMsgs{Src: []sdk.Msg{}, Dst: []sdk.Msg{}}

	if err := src.PathEnd.Validate(FULLPATH); err != nil {
		return nil, src.ErrPathNotSet(FULLPATH, err)
	}

	if err := dst.PathEnd.Validate(FULLPATH); err != nil {
		return nil, dst.ErrPathNotSet(FULLPATH, err)
	}

	hs, err := UpdatesWithHeaders(src, dst)
	if err != nil {
		return nil, err
	}

	var srcEnd, dstEnd chanTypes.ChannelResponse
	if dstEnd, err = dst.QueryChannel(hs[dst.ChainID].Height); err != nil {
		return nil, err
	}

	if srcEnd, err = src.QueryChannel(hs[src.ChainID].Height); err != nil {
		return nil, err
	}

	switch {
	// Handshake hasn't been started on src or dst, relay `chanOpenInit` to src
	case srcEnd.Channel.State == chanState.UNINITIALIZED && dstEnd.Channel.State == chanState.UNINITIALIZED:
		out.Src = append(out.Src, src.ChanInit(dst, ordering))

	// Handshake has started on dst (1 step done), relay `chanOpenTry` and `updateClient` to src
	case srcEnd.Channel.State == chanState.UNINITIALIZED && dstEnd.Channel.State == chanState.INIT:
		out.Src = append(out.Src, src.UpdateClient(hs[dst.ChainID]),
			src.ChanTry(dst, dstEnd))

	// Handshake has started on src (1 step done), relay `chanOpenTry` and `updateClient` to dst
	case srcEnd.Channel.State == chanState.INIT && dstEnd.Channel.State == chanState.UNINITIALIZED:
		out.Dst = append(out.Dst, dst.UpdateClient(hs[src.ChainID]),
			dst.ChanTry(dst, dstEnd))

	// Handshake has started on src (2 steps done), relay `chanOpenAck` and `updateClient` to dst
	case srcEnd.Channel.State == chanState.TRYOPEN && dstEnd.Channel.State == chanState.INIT:
		out.Dst = append(out.Dst, dst.UpdateClient(hs[src.ChainID]),
			dst.ChanAck(srcEnd))

	// Handshake has started on dst (2 steps done), relay `chanOpenAck` and `updateClient` to src
	case srcEnd.Channel.State == chanState.INIT && dstEnd.Channel.State == chanState.TRYOPEN:
		out.Src = append(out.Src, src.UpdateClient(hs[dst.ChainID]),
			src.ChanAck(dstEnd))

	// Handshake has confirmed on dst (3 steps done), relay `chanOpenConfirm` and `updateClient` to src
	case srcEnd.Channel.State == chanState.TRYOPEN && dstEnd.Channel.State == chanState.OPEN:
		out.Src = append(out.Src, src.UpdateClient(hs[dst.ChainID]),
			src.ChanConfirm(dstEnd))

	// Handshake has confirmed on src (3 steps done), relay `chanOpenConfirm` and `updateClient` to dst
	case srcEnd.Channel.State == chanState.OPEN && dstEnd.Channel.State == chanState.TRYOPEN:
		out.Dst = append(out.Dst, dst.UpdateClient(hs[src.ChainID]),
			dst.ChanConfirm(srcEnd))
	}

	return out, nil
}

// UpdateClient creates an sdk.Msg to update the client on c with data pulled from cp
func (src *Chain) UpdateClient(dstHeader *tmclient.Header) sdk.Msg {
	return tmclient.NewMsgUpdateClient(
		src.PathEnd.ClientID,
		*dstHeader,
		src.MustGetAddress(),
	)
}

// CreateClient creates an sdk.Msg to update the client on src with consensus state from dst
func (src *Chain) CreateClient(dstHeader *tmclient.Header) sdk.Msg {
	if err := dstHeader.ValidateBasic(dstHeader.ChainID); err != nil {
		panic(err)
	}
	// TODO: figure out how to dynmaically set unbonding time
	return tmclient.NewMsgCreateClient(
		src.PathEnd.ClientID,
		*dstHeader,
		src.TrustingPeriod,
		defaultUnbondingTime,
		src.MustGetAddress(),
	)
}

// ConnInit creates a MsgConnectionOpenInit
func (src *Chain) ConnInit(dst *Chain) sdk.Msg {
	return connTypes.NewMsgConnectionOpenInit(
		src.PathEnd.ConnectionID,
		src.PathEnd.ClientID,
		dst.PathEnd.ConnectionID,
		dst.PathEnd.ClientID,
		defaultChainPrefix,
		src.MustGetAddress(),
	)
}

// ConnTry creates a MsgConnectionOpenTry
// NOTE: ADD NOTE ABOUT PROOF HEIGHT CHANGE HERE
func (src *Chain) ConnTry(dst *Chain, dstConnState connTypes.ConnectionResponse, dstConsState clientTypes.ConsensusStateResponse, dstCsHeight int64) sdk.Msg {
	fmt.Printf("dstConnState height: %v\n", dstConnState.ProofHeight)
	fmt.Printf("dstConsState height: %v\n", dstConsState.ProofHeight)
	return connTypes.NewMsgConnectionOpenTry(
		src.PathEnd.ConnectionID,
		src.PathEnd.ClientID,
		dst.PathEnd.ConnectionID,
		dst.PathEnd.ClientID,
		defaultChainPrefix,
		defaultIBCVersions,
		dstConnState.Proof,
		dstConsState.Proof,
		dstConnState.ProofHeight+1,
		uint64(dstCsHeight),
		src.MustGetAddress(),
	)
}

// ConnAck creates a MsgConnectionOpenAck
// NOTE: ADD NOTE ABOUT PROOF HEIGHT CHANGE HERE
func (src *Chain) ConnAck(dstConnState connTypes.ConnectionResponse, dstConsState clientTypes.ConsensusStateResponse, dstCsHeight int64) sdk.Msg {
	return connTypes.NewMsgConnectionOpenAck(
		src.PathEnd.ConnectionID,
		dstConnState.Proof,
		dstConsState.Proof,
		dstConnState.ProofHeight+1,
		uint64(dstCsHeight),
		defaultIBCVersion,
		src.MustGetAddress(),
	)
}

// ConnConfirm creates a MsgConnectionOpenAck
// NOTE: ADD NOTE ABOUT PROOF HEIGHT CHANGE HERE
func (src *Chain) ConnConfirm(dstConnState connTypes.ConnectionResponse, dstConsState clientTypes.ConsensusStateResponse, dstCsHeight int64) sdk.Msg {
	return connTypes.NewMsgConnectionOpenAck(
		src.PathEnd.ConnectionID,
		dstConnState.Proof,
		dstConsState.Proof,
		dstConnState.ProofHeight+1,
		uint64(dstCsHeight),
		defaultIBCVersion,
		src.MustGetAddress(),
	)
}

// ChanInit creates a MsgChannelOpenInit
func (src *Chain) ChanInit(dst *Chain, ordering chanState.Order) sdk.Msg {
	return chanTypes.NewMsgChannelOpenInit(
		src.PathEnd.PortID,
		src.PathEnd.ChannelID,
		defaultIBCVersion,
		ordering,
		[]string{src.PathEnd.ConnectionID, dst.PathEnd.ConnectionID},
		dst.PathEnd.PortID,
		dst.PathEnd.ChannelID,
		src.MustGetAddress(),
	)
}

// ChanTry creates a MsgChannelOpenTry
func (src *Chain) ChanTry(dst *Chain, dstChanState chanTypes.ChannelResponse) sdk.Msg {
	return chanTypes.NewMsgChannelOpenTry(
		src.PathEnd.PortID,
		src.PathEnd.ChannelID,
		defaultIBCVersion,
		dstChanState.Channel.Ordering,
		dstChanState.Channel.ConnectionHops,
		dst.PathEnd.PortID,
		dst.PathEnd.ChannelID,
		defaultIBCVersion,
		dstChanState.Proof,
		dstChanState.ProofHeight,
		src.MustGetAddress(),
	)
}

// ChanAck creates a MsgChannelOpenAck
func (src *Chain) ChanAck(dstChanState chanTypes.ChannelResponse) sdk.Msg {
	return chanTypes.NewMsgChannelOpenAck(
		src.PathEnd.PortID,
		src.PathEnd.ChannelID,
		dstChanState.Channel.GetVersion(),
		dstChanState.Proof,
		dstChanState.ProofHeight,
		src.MustGetAddress(),
	)
}

// ChanConfirm creates a MsgChannelOpenConfirm
func (src *Chain) ChanConfirm(dstChanState chanTypes.ChannelResponse) sdk.Msg {
	return chanTypes.NewMsgChannelOpenConfirm(
		src.PathEnd.PortID,
		src.PathEnd.ChannelID,
		dstChanState.Proof,
		dstChanState.ProofHeight,
		src.MustGetAddress(),
	)
}

// ChanCloseInit creates a MsgChannelCloseInit
func (src *Chain) ChanCloseInit() sdk.Msg {
	return chanTypes.NewMsgChannelCloseInit(
		src.PathEnd.PortID,
		src.PathEnd.ChannelID,
		src.MustGetAddress(),
	)
}

// ChanCloseConfirm creates a MsgChannelCloseConfirm
func (src *Chain) ChanCloseConfirm(dstChanState chanTypes.ChannelResponse) sdk.Msg {
	return chanTypes.NewMsgChannelCloseConfirm(
		src.PathEnd.PortID,
		src.PathEnd.ChannelID,
		dstChanState.Proof,
		dstChanState.ProofHeight,
		src.MustGetAddress(),
	)
}

// SendMsg wraps the msg in a stdtx, signs and sends it
func (src *Chain) SendMsg(datagram sdk.Msg) (sdk.TxResponse, error) {
	return src.SendMsgs([]sdk.Msg{datagram})
}

// SendMsgs wraps the msgs in a stdtx, signs and sends it
func (src *Chain) SendMsgs(datagrams []sdk.Msg) (res sdk.TxResponse, err error) {
	var out []byte
	if out, err = src.BuildAndSignTx(datagrams); err != nil {
		return res, err
	}
	return src.BroadcastTxCommit(out)
}
