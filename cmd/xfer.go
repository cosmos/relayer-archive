package cmd

import (
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	chanTypes "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/types"
	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
)

// NOTE: These commands are registered over in cmd/raw.go

func xfer() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "xfer [src-chain-id] [dst-chain-id] [src-chan-id] [dst-chan-id] [src-port-id] [dst-port-id] [amount] [dst-addr]",
		Short: "xfer",
		Long:  "This sends tokens from a relayers configured wallet on chain src to a dst addr on dst",
		Args:  cobra.ExactArgs(8),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			if err = chains[src].PathChannel(args[2], args[4]); err != nil {
				return chains[src].ErrCantSetPath(relayer.CLNTCHANPATH, err)
			}

			if err = chains[dst].PathChannel(args[3], args[5]); err != nil {
				return chains[dst].ErrCantSetPath(relayer.CHANPATH, err)
			}

			amount, err := sdk.ParseCoin(args[6])
			if err != nil {
				return err
			}

			// If there is a path seperator in the denom of the coins being sent,
			// then src is not the source, otherwise it is
			// NOTE: this will not work in the case where tokens are sent from A -> B -> C
			// Need a function in the SDK to determine from a denom if the tokens are from this chain
			var source bool
			if strings.Contains(amount.GetDenom(), "/") {
				source = false
			} else {
				source = true
			}

			dstAddr, err := sdk.AccAddressFromBech32(args[7])
			if err != nil {
				return err
			}

			dstHeader, err := chains[dst].UpdateLiteWithHeader()
			if err != nil {
				return err
			}

			// MsgTransfer will call SendPacket on src chain
			txs := []sdk.Msg{
				chains[src].MsgTransfer(chains[dst], dstHeader.GetHeight(), sdk.NewCoins(amount), dstAddr, source),
			}

			err = SendAndPrint(txs, chains[src], cmd)
			if err != nil {
				return err
			}

			// Working on SRC chain :point_up:
			// Working on DST chain :point_down:

			hs, err := relayer.UpdatesWithHeaders(chains[src], chains[dst])
			if err != nil {
				return err
			}

			seqRecv, err := chains[dst].QueryNextSeqRecv(hs[dst].Height - 1)
			if err != nil {
				return err
			}

			seqSend, err := chains[src].QueryNextSeqSend(hs[src].Height - 1)
			if err != nil {
				return err
			}

			srcCommitRes, err := chains[src].QueryPacketCommitment(hs[src].Height, int64(seqSend-1))
			if err != nil {
				return err
			}

			// reconstructing packet data here instead of retrieving from an indexed node
			xferPacket := chains[src].XferPacket(
				sdk.NewCoins(),
				chains[src].MustGetAddress(),
				dstAddr,
				false,
				19291024,
			)

			// Debugging by simply passing in the packet information that we know was sent earlier in the SendPacket
			// part of the command. In a real relayer, this would be a separate command that retrieved the packet
			// information from an indexing node
			txs = []sdk.Msg{
				chains[src].UpdateClient(hs[dst]),
				chains[src].MsgRecvPacket(
					chains[dst],
					seqRecv.NextSequenceRecv,
					xferPacket,
					chanTypes.NewPacketResponse(
						chains[dst].PathEnd.PortID,
						chains[dst].PathEnd.ChannelID,
						seqSend,
						chains[src].NewPacket(
							chains[dst],
							seqSend,
							xferPacket,
						),
						srcCommitRes.Proof.Proof,
						int64(srcCommitRes.ProofHeight),
					),
				),
			}

			return SendAndPrint(txs, chains[src], cmd)
		},
	}
	return transactionFlags(cmd)
}

func xfersend() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "xfer-send [src-chain-id] [dst-chain-id] [src-chan-id] [dst-chan-id] [src-port-id] [dst-port-id] [amount] [dst-addr]",
		Short: "xfer-send",
		Long:  "This sends tokens from a relayers configured wallet on chain src to a dst addr on dst",
		Args:  cobra.ExactArgs(8),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			if err = chains[src].PathChannel(args[2], args[4]); err != nil {
				return chains[src].ErrCantSetPath(relayer.CLNTCHANPATH, err)
			}

			if err = chains[dst].PathChannel(args[3], args[5]); err != nil {
				return chains[dst].ErrCantSetPath(relayer.CHANPATH, err)
			}

			amount, err := sdk.ParseCoin(args[6])
			if err != nil {
				return err
			}

			// If there is a path seperator in the denom of the coins being sent,
			// then src is not the source, otherwise it is
			// NOTE: this will not work in the case where tokens are sent from A -> B -> C
			// Need a function in the SDK to determine from a denom if the tokens are from this chain
			var source bool
			if strings.Contains(amount.GetDenom(), "/") {
				source = false
			} else {
				source = true
			}

			dstAddr, err := sdk.AccAddressFromBech32(args[7])
			if err != nil {
				return err
			}

			dstHeader, err := chains[dst].UpdateLiteWithHeader()
			if err != nil {
				return err
			}

			txs := []sdk.Msg{
				chains[src].MsgTransfer(chains[dst], dstHeader.GetHeight(), sdk.NewCoins(amount), dstAddr, source),
			}

			return SendAndPrint(txs, chains[src], cmd)
		},
	}
	return transactionFlags(cmd)
}

// UNTESTED: Currently filled with incorrect logic to make code compile
func xferrecv() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "xfer-recv [src-chain-id] [dst-chain-id] [src-chan-id] [dst-chan-id] [src-port-id] [dst-port-id] [amount] [dst-addr]",
		Short: "xfer-recv",
		Long:  "recives tokens sent from dst to src",
		Args:  cobra.ExactArgs(8),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			if err = chains[src].PathChannel(args[2], args[4]); err != nil {
				return chains[src].ErrCantSetPath(relayer.CLNTCHANPATH, err)
			}

			if err = chains[dst].PathChannel(args[3], args[5]); err != nil {
				return chains[dst].ErrCantSetPath(relayer.CHANPATH, err)
			}

			hs, err := relayer.UpdatesWithHeaders(chains[src], chains[dst])
			if err != nil {
				return err
			}

			seqRecv, err := chains[src].QueryNextSeqRecv(hs[src].Height - 1)
			if err != nil {
				return err
			}

			// seqSend, err := chains[dst].QueryNextSeqSend(hs[dst].Height - 1)
			// if err != nil {
			// 	return err
			// }

			// dstCommitRes, err := chains[dst].QueryPacketCommitment(hs[dst].Height, int64(seqSend))
			// if err != nil {
			// 	return err
			// }

			txs := []sdk.Msg{
				chains[src].UpdateClient(hs[dst]),
				chains[src].MsgRecvPacket(
					chains[dst],
					seqRecv.NextSequenceRecv,
					chains[src].XferPacket(
						sdk.NewCoins(),
						chains[src].MustGetAddress(),
						chains[src].MustGetAddress(),
						false,
						19291024),
					chanTypes.PacketResponse{},
				),
			}

			return SendAndPrint(txs, chains[src], cmd)
		},
	}
	return transactionFlags(cmd)
}
