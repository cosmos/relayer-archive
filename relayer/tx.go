package relayer

import (
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clientExported "github.com/cosmos/cosmos-sdk/x/ibc/02-client/exported"
	clientTypes "github.com/cosmos/cosmos-sdk/x/ibc/02-client/types"
	connState "github.com/cosmos/cosmos-sdk/x/ibc/03-connection/exported"
	connTypes "github.com/cosmos/cosmos-sdk/x/ibc/03-connection/types"
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
	if len(errs) != 0 {
		for _, err := range errs {
			return nil, err
		}
	}

	hs, errs := GetLatestHeaders(src, dst)
	if len(errs) != 0 {
		for _, err := range errs {
			return nil, err
		}
	}

	srcAddr, err := src.GetAddress()
	if err != nil {
		return nil, err
	}

	dstAddr, err := dst.GetAddress()
	if err != nil {
		return nil, err
	}

	srcEnd, err := src.QueryConnection(srcConnectionID, hs.Map[src.ChainID].Height)
	if err != nil {
		return nil, err
	}

	dstEnd, err := dst.QueryConnection(dstConnectionID, hs.Map[src.ChainID].Height)
	if err != nil {
		return nil, err
	}

	switch {
	// Handshake hasn't been started locally, relay `connOpenInit` locally
	case srcEnd.Connection.State == connState.UNINITIALIZED && dstEnd.Connection.State == connState.UNINITIALIZED:
		// TODO: need to add a msgUpdateClient here?
		out.Src = append(out.Src, connTypes.NewMsgConnectionOpenInit(
			srcConnectionID,
			srcClientID,
			dstConnectionID,
			dstClientID,
			defaultChainPrefix,
			srcAddr,
		))

	// Handshake has started locally (1 step done), relay `connOpenTry` to the remote end
	case srcEnd.Connection.State == connState.INIT && dstEnd.Connection.State == connState.UNINITIALIZED:
		out.Dst = append(out.Dst, dst.UpdateClient(dstClientID, hs.Map[src.ChainID]),
			connTypes.NewMsgConnectionOpenTry(
				dstConnectionID,
				dstClientID,
				srcConnectionID,
				srcClientID,
				defaultChainPrefix,
				defaultIBCVersions,
				srcEnd.Proof,
				srcEnd.Proof,
				srcEnd.ProofHeight,
				uint64(hs.Map[dst.ChainID].Height),
				dstAddr,
			))

	// Handshake has started on the other end (2 steps done), relay `connOpenAck` to the local end
	case srcEnd.Connection.State == connState.INIT && dstEnd.Connection.State == connState.TRYOPEN:
		out.Src = append(out.Src, src.UpdateClient(srcClientID, hs.Map[dst.ChainID]),
			connTypes.NewMsgConnectionOpenAck(
				srcConnectionID,
				dstEnd.Proof,
				dstEnd.Proof,
				dstEnd.ProofHeight,
				uint64(hs.Map[src.ChainID].Height),
				ibcversion,
				srcAddr,
			))

	// Handshake has confirmed locally (3 steps done), relay `connOpenConfirm` to the remote end
	case srcEnd.Connection.State == connState.OPEN && dstEnd.Connection.State == connState.TRYOPEN:
		out.Dst = append(out.Dst, dst.UpdateClient(dstClientID, hs.Map[src.ChainID]),
			connTypes.NewMsgConnectionOpenConfirm(
				dstConnectionID,
				srcEnd.Proof,
				srcEnd.ProofHeight,
				dstAddr,
			))
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

func (src *Chain) ChanInit(dst *Chain) sdk.Msg {
	return nil
}

func (src *Chain) ChanTry(dst *Chain) sdk.Msg {
	return nil
}

func (src *Chain) ChanAck(dst *Chain) sdk.Msg {
	return nil
}

func (src *Chain) ChanConfirm(dst *Chain) sdk.Msg {
	return nil
}

func (src *Chain) ChanCloseInit(dst *Chain) sdk.Msg {
	return nil
}

func (src *Chain) ChanCloseConfirm(dst *Chain) sdk.Msg {
	return nil
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
