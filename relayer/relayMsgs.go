package relayer

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// RelayMsgs contains the msgs that need to be sent to both a src and dst chain
// after a given relay round
type RelayMsgs struct {
	Src []sdk.Msg
	Dst []sdk.Msg
}

// Ready returns true if there are messages to relay
func (r *RelayMsgs) Ready() bool {
	if len(r.Src) == 0 && len(r.Dst) == 0 {
		return false
	}
	return true
}

// Send sends the messages with appropriate output
func (r *RelayMsgs) Send(src, dst *Chain) error {
	// TODO: maybe figure out a better way to indicate error here?
	var out error

	// TODO: Parallelize? Maybe?
	if len(r.Src) > 0 {
		// Submit the transactions to src chain
		res, err := src.SendMsgs(r.Src)
		if err != nil || res.Code != 0 {
			out = err
			src.LogFailedTx(res, r.Src)
		} else {
			// NOTE: Add more data to this such as identifiers
			src.LogSuccessTx(res, r.Src)
		}
	}

	if len(r.Dst) > 0 {
		// Submit the transactions to dst chain
		res, err := dst.SendMsgs(r.Dst)
		if err != nil || res.Code != 0 {
			out = err
			dst.LogFailedTx(res, r.Dst)
		} else {
			// NOTE: Add more data to this such as identifiers
			dst.LogSuccessTx(res, r.Dst)
		}
	}

	return out
}

// LogFailedTx takes the transaction and the messages to create it and logs the appropriate data
func (c *Chain) LogFailedTx(res sdk.TxResponse, msgs []sdk.Msg) {
	if c.debug {
		c.Log(fmt.Sprintf("- [%s] -> sending transaction:", c.ChainID))
		c.Print(msgs, false, false)
	}

	msg, err := GetCodespace(res.Codespace, int(res.Code))
	if err != nil {
		c.logger.Info(err.Error())
	}

	c.logger.Info(fmt.Sprintf("✘ [%s]@{%d} - msg(%s) err(%s: %s)", c.ChainID, res.Height, getMsgAction(msgs), res.Codespace, msg))
	if c.debug {
		c.Print(res, false, false)
	}
}

// LogSuccessTx take the transaction and the messages to create it and logs the appropriate data
func (c *Chain) LogSuccessTx(res sdk.TxResponse, msgs []sdk.Msg) {
	c.logger.Info(fmt.Sprintf("✔ [%s]@{%d} - msg(%s)", c.ChainID, res.Height, getMsgAction(msgs)))
}

func getMsgAction(msgs []sdk.Msg) string {
	switch len(msgs) {
	case 1:
		return msgs[0].Type()
	case 2:
		return msgs[1].Type()
	default:
		return ""
	}
}
