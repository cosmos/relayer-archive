package relayer

import (
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clientExported "github.com/cosmos/cosmos-sdk/x/ibc/02-client/exported"
	clientTypes "github.com/cosmos/cosmos-sdk/x/ibc/02-client/types"
	connState "github.com/cosmos/cosmos-sdk/x/ibc/03-connection/exported"
	connTypes "github.com/cosmos/cosmos-sdk/x/ibc/03-connection/types"
	chanState "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/exported"
	chanTypes "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/types"
	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint"
	commitment "github.com/cosmos/cosmos-sdk/x/ibc/23-commitment"
)

var (
	defaultChainPrefix = commitment.NewPrefix([]byte("ibc"))
	defaultIBCVersion  = "1.0.0"
	defaultIBCVersions = []string{defaultIBCVersion}
)

// CreateConnection creates a connection between two chains given src and dst client IDs
func (src *Chain) CreateConnection(dst *Chain, srcClientID, dstClientID, srcConnectionID, dstConnectionID string, timeout time.Duration) error {
	ticker := time.NewTicker(timeout)
	for ; true; <-ticker.C {
		msgs, err := src.CreateConnectionStep(dst, srcClientID, dstClientID, srcConnectionID, dstConnectionID)
		if err != nil {
			return err
		}

		if len(msgs.Dst) == 0 && len(msgs.Src) == 0 {
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

// CreateConnectionStep returns the next set of messags for relaying between a src and dst chain
func (src *Chain) CreateConnectionStep(dst *Chain,
	srcClientID, dstClientID,
	srcConnectionID, dstConnectionID string) (*RelayMsgs, error) {
	out := &RelayMsgs{}

	errs := UpdateLiteDBsToLatestHeaders(src, dst)
	if err != nil {
		return err
	}
	hs, errs := GetLatestHeaders(src, dst)
	if err != nil {
		return err
	}
	srcEnd, err := src.QueryConnection(srcConnectionID, hs[src.ChainID].Height)
	if err != nil {
		return nil, err
	}

	dstEnd, err := dst.QueryConnection(dstConnectionID, hs[src.ChainID].Height)
	if err != nil {
		return nil, err
	}

	switch {
	// Handshake hasn't been started locally, relay `connOpenInit` locally
	case srcEnd.Connection.State == connState.UNINITIALIZED && dstEnd.Connection.State == connState.UNINITIALIZED:
		// TODO: need to add a msgUpdateClient here?
		out.Src = append(out.Src, src.ConnInit(srcConnectionID, srcClientID, dstConnectionID, dstClientID))

	// Handshake has started locally (1 step done), relay `connOpenTry` to the remote end
	case srcEnd.Connection.State == connState.INIT && dstEnd.Connection.State == connState.UNINITIALIZED:
		out.Dst = append(out.Dst, dst.UpdateClient(dstClientID, hs[src.ChainID]),
			dst.ConnTry(dstClientID, srcClientID, dstConnectionID, srcConnectionID, srcEnd, hs[src.ChainID].Height))

	// Handshake has started on the other end (2 steps done), relay `connOpenAck` to the local end
	case srcEnd.Connection.State == connState.INIT && dstEnd.Connection.State == connState.TRYOPEN:
		out.Src = append(out.Src, src.UpdateClient(srcClientID, hs[dst.ChainID]),
			src.ConnAck(srcConnectionID, dstEnd, hs[src.ChainID].Height))

	// Handshake has confirmed locally (3 steps done), relay `connOpenConfirm` to the remote end
	case srcEnd.Connection.State == connState.OPEN && dstEnd.Connection.State == connState.TRYOPEN:
		out.Dst = append(out.Dst, dst.UpdateClient(dstClientID, hs[src.ChainID]),
			dst.ConnConfirm(dstConnectionID, srcEnd, hs[dst.ChainID].Height))
	default:
		fmt.Printf("srcEnd.Connection %#v\n", srcEnd.Connection)
		fmt.Printf("dstEnd.Connection %#v\n", dstEnd.Connection)
	}
	return &RelayMsgs{}, nil
}

// CreateChannel creates a connection between two chains given src and dst client IDs
func (src *Chain) CreateChannel(dst *Chain,
	srcConnectionID, dstConnectionID,
	srcChannelID, dstChannelID,
	srcPortID, dstPortID string,
	timeout time.Duration) error {
	srcAddr, dstAddr, srcHeader, dstHeader, err := addrsHeaders(src, dst)
	if err != nil {
		return err
	}

	ticker := time.NewTicker(timeout)
	for ; true; <-ticker.C {
		msgs, err := src.CreateChannelStep(dst, srcHeader, dstHeader, srcAddr, dstAddr, srcConnectionID, dstConnectionID, srcChannelID, dstChannelID, srcPortID, dstPortID)
		if err != nil {
			return err
		}

		if len(msgs.Dst) == 0 && len(msgs.Src) == 0 {
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

// CreateChannelStep returns the next set of messages for relaying between a src and dst chain
func (src *Chain) CreateChannelStep(dst *Chain,
	srcHeader, dstHeader *tmclient.Header,
	srcAddr, dstAddr sdk.Address,
	srcConnectionID, dstConnectionID,
	srcChannelID, dstChannelID,
	srcPortID, dstPortID string) (*RelayMsgs, error) {
	return &RelayMsgs{}, nil
}

// UpdateClient creates an sdk.Msg to update the client on c with data pulled from cp
func (c *Chain) UpdateClient(srcClientID string, dstHeader *tmclient.Header) clientTypes.MsgUpdateClient {
	return clientTypes.NewMsgUpdateClient(srcClientID, dstHeader, c.MustGetAddress())
}

// CreateClient creates an sdk.Msg to update the client on src with consensus state from dst
func (c *Chain) CreateClient(srcClientID string, dstHeader *tmclient.Header) clientTypes.MsgCreateClient {
	return clientTypes.NewMsgCreateClient(srcClientID, clientExported.ClientTypeTendermint, dstHeader.ConsensusState(), c.MustGetAddress())
}

func (c *Chain) ConnInit(srcConnID, srcClientID, dstConnId, dstClientID string) sdk.Msg {
	return connTypes.NewMsgConnectionOpenInit(srcConnID, srcClientID, dstConnId, dstClientID, defaultChainPrefix, c.MustGetAddress())
}

func (c *Chain) ConnTry(srcClientID, dstClientID, srcConnID, dstConnID string, dstConnState connTypes.ConnectionResponse, srcHeight int64) sdk.Msg {
	return connTypes.NewMsgConnectionOpenTry(srcConnID, srcClientID, dstConnID, dstClientID, defaultChainPrefix, defaultIBCVersions, dstConnState.Proof, dstConnState.Proof, dstConnState.ProofHeight, uint64(srcHeight), c.MustGetAddress())
}

func (c *Chain) ConnAck(srcConnID string, dstConnState connTypes.ConnectionResponse, srcHeight int64) sdk.Msg {
	return connTypes.NewMsgConnectionOpenAck(srcConnID, dstConnState.Proof, dstConnState.Proof, dstConnState.ProofHeight, uint64(srcHeight), defaultIBCVersion, c.MustGetAddress())
}

func (c *Chain) ConnConfirm(srcConnID string, dstConnState connTypes.ConnectionResponse, srcHeight int64) sdk.Msg {
	return connTypes.NewMsgConnectionOpenAck(srcConnID, dstConnState.Proof, dstConnState.Proof, dstConnState.ProofHeight, uint64(srcHeight), defaultIBCVersion, c.MustGetAddress())
}

func (c *Chain) ChanInit(srcConnID, srcChanID, dstChanID, srcPortID, dstPortID string, ordering chanState.Order) sdk.Msg {
	return chanTypes.NewMsgChannelOpenInit(srcPortID, srcChanID, defaultIBCVersion, ordering, []string{srcConnID}, dstPortID, dstChanID, c.MustGetAddress())
}

func (c *Chain) ChanTry(srcChanID, dstChanID, srcPortID, dstPortID string, dstChanState chanTypes.ChannelResponse) sdk.Msg {
	return chanTypes.NewMsgChannelOpenTry(srcPortID, srcChanID, defaultIBCVersion, dstChanState.Channel.Ordering, dstChanState.Channel.ConnectionHops,
		dstPortID, dstChanID, defaultIBCVersion, dstChanState.Proof, dstChanState.ProofHeight, c.MustGetAddress())
}

func (c *Chain) ChanAck(srcChanID, srcPortID string, dstChanState chanTypes.ChannelResponse) sdk.Msg {
	return chanTypes.NewMsgChannelOpenAck(srcPortID, srcChanID, dstChanState.Channel.GetVersion(), dstChanState.Proof, dstChanState.ProofHeight, c.MustGetAddress())
}

func (c *Chain) ChanConfirm(srcChanID, srcPortID string, dstChanState chanTypes.ChannelResponse) sdk.Msg {
	return chanTypes.NewMsgChannelOpenConfirm(srcPortID, srcChanID, dstChanState.Proof, dstChanState.ProofHeight, c.MustGetAddress())
}

func (c *Chain) ChanCloseInit(srcChanID, srcPortID string) sdk.Msg {
	return chanTypes.NewMsgChannelCloseInit(srcPortID, srcChanID, c.MustGetAddress())
}

func (c *Chain) ChanCloseConfirm(srcChanID, srcPortID string, dstChanState chanTypes.ChannelResponse) sdk.Msg {
	return chanTypes.NewMsgChannelCloseConfirm(srcPortID, srcChanID, dstChanState.Proof, dstChanState.ProofHeight, c.MustGetAddress())
}

// SendMsg wraps the msg in a stdtx, signs and sends it
func (c *Chain) SendMsg(datagram sdk.Msg) (sdk.TxResponse, error) {
	return c.SendMsgs([]sdk.Msg{datagram})
}

// SendMsgs wraps the msgs in a stdtx, signs and sends it
func (c *Chain) SendMsgs(datagrams []sdk.Msg) (sdk.TxResponse, error) {

	txBytes, err := c.BuildAndSignTx(datagrams)
	if err != nil {
		return sdk.TxResponse{}, err
	}

	return c.BroadcastTxCommit(txBytes)
}
