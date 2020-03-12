/*
Copyright © 2020 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"os"
	"strconv"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	chanState "github.com/cosmos/cosmos-sdk/x/ibc/04-channel/exported"
	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
	"github.com/tendermint/tendermint/libs/log"
)

// transactionCmd represents the tx command
func transactionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "transactions",
		Aliases: []string{"tx"},
		Short:   "IBC Transaction Commands, UNDER CONSTRUCTION",
	}

	cmd.AddCommand(
		createClientCmd(),
		createClientsCmd(),
		createConnectionCmd(),
		createConnectionStepCmd(),
		createChannelCmd(),
		createChannelStepCmd(),
		updateClientCmd(),
		rawTransactionCmd(),
		fullPathCmd(),
	)

	return cmd
}

func updateClientCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update-client [src-chain-id] [dst-chain-id] [client-id]",
		Short: "update client for dst-chain on src-chain",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]

			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			if err = chains[src].PathClient(args[2]); err != nil {
				return chains[src].ErrCantSetPath(relayer.CLNTPATH, err)
			}
			if err != nil {
				return err
			}

			dstHeader, err := chains[dst].UpdateLiteWithHeader()
			if err != nil {
				return err
			}

			return SendAndPrint([]sdk.Msg{chains[src].UpdateClient(dstHeader)}, chains[src], cmd)
		},
	}
	return transactionFlags(cmd)
}

func createClientCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "client [src-chain-id] [dst-chain-id] [client-id]",
		Short: "create a client for dst-chain on src-chain",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			dstHeader, err := chains[dst].UpdateLiteWithHeader()
			if err != nil {
				return err
			}

			err = chains[src].PathClient(args[2])
			if err != nil {
				return err
			}

			return SendAndPrint([]sdk.Msg{chains[src].CreateClient(dstHeader)}, chains[src], cmd)
		},
	}

	return transactionFlags(cmd)
}

func createClientsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clients [src-chain-id] [dst-chain-id] [index]",
		Short: "create a clients for dst-chain on src-chain and src-chain on dst-chain",
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			hs, err := relayer.UpdatesWithHeaders(chains[src], chains[dst])
			if err != nil {
				return err
			}

			// Find any configured paths between the chains
			paths, err := config.Paths.PathsFromChains(src, dst)
			if err != nil {
				return err
			}

			// Given the number of args and the number of paths,
			// work on the appropriate path
			var path relayer.Path
			switch {
			case len(args) == 3 && len(paths) > 1:
				i, err := strconv.ParseInt(args[2], 10, 64)
				if err != nil {
					return err
				}
				path = paths[i]
			case len(args) == 3 && len(paths) == 1:
				fmt.Println(paths.MustYAML())
				return fmt.Errorf("passed in index where only one path exists between chains %s and %s", src, dst)
			case len(args) == 2 && len(paths) > 1:
				fmt.Println(paths.MustYAML())
				return fmt.Errorf("more than one path between %s and %s exists, please try again with index", src, dst)
			case len(args) == 2 && len(paths) == 1:
				path = paths[0]
			}

			if err = chains[src].SetPath(path.Src, relayer.CLNTPATH); err != nil {
				return chains[src].ErrCantSetPath(relayer.CLNTPATH, err)
			}

			if err = chains[dst].SetPath(path.Dst, relayer.CLNTPATH); err != nil {
				return chains[dst].ErrCantSetPath(relayer.CLNTPATH, err)
			}

			if err = SendAndPrint([]sdk.Msg{chains[src].CreateClient(hs[dst])}, chains[src], cmd); err != nil {
				return err
			}

			return SendAndPrint([]sdk.Msg{chains[dst].CreateClient(hs[src])}, chains[dst], cmd)
		},
	}
	return transactionFlags(cmd)
}

func createConnectionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connection [src-chain-id] [dst-chain-id] [index]",
		Short: "create a connection between two configured chains with a configured path",
		Long:  "This command is meant to be used to repair or create a connection between two chains with a configured path in the config file",
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			// Find any configured paths between the chains
			paths, err := config.Paths.PathsFromChains(src, dst)
			if err != nil {
				return err
			}

			// Given the number of args and the number of paths,
			// work on the appropriate path
			var path relayer.Path
			switch {
			case len(args) == 3 && len(paths) > 1:
				i, err := strconv.ParseInt(args[2], 10, 64)
				if err != nil {
					return err
				}
				path = paths[i]
			case len(args) == 3 && len(paths) == 1:
				fmt.Println(paths.MustYAML())
				return fmt.Errorf("passed in index where only one path exists between chains %s and %s", src, dst)
			case len(args) == 2 && len(paths) > 1:
				fmt.Println(paths.MustYAML())
				return fmt.Errorf("more than one path between %s and %s exists, please try again with index", src, dst)
			case len(args) == 2 && len(paths) == 1:
				path = paths[0]
			}

			to, err := getTimeout(cmd)
			if err != nil {
				return err
			}

			if err = chains[src].SetPath(path.End(src), relayer.CONNPATH); err != nil {
				return chains[src].ErrCantSetPath(relayer.CONNPATH, err)
			}

			if err = chains[dst].SetPath(path.End(dst), relayer.CONNPATH); err != nil {
				return chains[dst].ErrCantSetPath(relayer.CONNPATH, err)
			}

			ticker := time.NewTicker(to)
			for ; true; <-ticker.C {
				msgs, err := chains[src].CreateConnectionStep(chains[dst])
				if err != nil {
					return err
				}

				if !msgs.Ready() {
					break
				}

				if len(msgs.Src) > 0 {
					// Submit the transactions to src chain
					if err = SendAndPrint(msgs.Src, chains[src], cmd); err != nil {
						return err
					}
				}

				if len(msgs.Dst) > 0 {
					// Submit the transactions to dst chain
					if err = SendAndPrint(msgs.Dst, chains[dst], cmd); err != nil {
						return err
					}
				}
			}

			return nil
		},
	}

	return timeoutFlag(transactionFlags(cmd))
}

func createConnectionStepCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connection-step [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] [src-connection-id] [dst-connection-id]",
		Short: "create a connection between chains, passing in identifiers",
		Long:  "This command creates the next handshake message given a specifc set of identifiers. If the command fails, you can safely run it again to repair an unfinished connection",
		Args:  cobra.ExactArgs(6),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			if err = chains[src].PathConnection(args[2], args[4]); err != nil {
				return chains[src].ErrCantSetPath(relayer.CONNPATH, err)
			}

			if err = chains[dst].PathConnection(args[3], args[5]); err != nil {
				return chains[dst].ErrCantSetPath(relayer.CONNPATH, err)
			}

			msgs, err := chains[src].CreateConnectionStep(chains[dst])
			if err != nil {
				return err
			}

			if len(msgs.Src) > 0 {
				if err = SendAndPrint(msgs.Src, chains[src], cmd); err != nil {
					return err
				}
			} else if len(msgs.Dst) > 0 {
				if err = SendAndPrint(msgs.Dst, chains[dst], cmd); err != nil {
					return err
				}
			}

			return nil
		},
	}

	return transactionFlags(cmd)
}

func createChannelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channel [src-chain-id] [dst-chain-id] [index]",
		Short: "create a channel between two configured chains with a configured path",
		Long:  "FYI: DRAGONS HERE, not tested",
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			to, err := getTimeout(cmd)
			if err != nil {
				return err
			}

			// Find any configured paths between the chains
			paths, err := config.Paths.PathsFromChains(src, dst)
			if err != nil {
				return err
			}

			// Given the number of args and the number of paths,
			// work on the appropriate path
			var path relayer.Path
			switch {
			case len(args) == 3 && len(paths) > 1:
				i, err := strconv.ParseInt(args[2], 10, 64)
				if err != nil {
					return err
				}
				path = paths[i]
			case len(args) == 3 && len(paths) == 1:
				fmt.Println(paths.MustYAML())
				return fmt.Errorf("passed in an index where only one path exists between chains %s and %s", src, dst)
			case len(args) == 2 && len(paths) > 1:
				fmt.Println(paths.MustYAML())
				return fmt.Errorf("more than one path between %s and %s exists, please specify index", src, dst)
			case len(args) == 2 && len(paths) == 1:
				path = paths[0]
			}

			if err = chains[src].SetPath(path.End(src), relayer.FULLPATH); err != nil {
				return chains[src].ErrCantSetPath(relayer.FULLPATH, err)
			}

			if err = chains[dst].SetPath(path.End(dst), relayer.FULLPATH); err != nil {
				return chains[dst].ErrCantSetPath(relayer.FULLPATH, err)
			}

			var order chanState.Order
			if order = chanState.OrderFromString("ORDERED"); order == chanState.NONE {
				return fmt.Errorf("invalid order passed in %s, expected 'UNORDERED' or 'ORDERED'", args[10])
			}

			ticker := time.NewTicker(to)
			for ; true; <-ticker.C {
				msgs, err := chains[src].CreateChannelStep(chains[dst], order)
				if err != nil {
					return err
				}

				if !msgs.Ready() {
					break
				}

				if len(msgs.Src) > 0 {
					// Submit the transactions to src chain
					if err = SendAndPrint(msgs.Src, chains[src], cmd); err != nil {
						return err
					}
				}

				if len(msgs.Dst) > 0 {
					// Submit the transactions to dst chain
					if err = SendAndPrint(msgs.Dst, chains[dst], cmd); err != nil {
						return err
					}
				}
			}
			return nil
		},
	}

	return timeoutFlag(transactionFlags(cmd))
}

func createChannelStepCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channel-step [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] [src-connection-id] [dst-connection-id] [src-channel-id] [dst-channel-id] [src-port-id] [dst-port-id] [ordering]",
		Short: "create the next step in creating a channel between chains with the passed identifiers",
		Args:  cobra.ExactArgs(11),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			ordering := chanState.OrderFromString(args[10])
			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			if err = chains[src].FullPath(args[2], args[4], args[6], args[8]); err != nil {
				return chains[src].ErrCantSetPath(relayer.FULLPATH, err)
			}

			if err = chains[dst].FullPath(args[3], args[5], args[7], args[9]); err != nil {
				return chains[dst].ErrCantSetPath(relayer.FULLPATH, err)
			}

			msgs, err := chains[src].CreateChannelStep(chains[dst], ordering)
			if err != nil {
				return err
			}

			if len(msgs.Src) > 0 {
				if err = SendAndPrint(msgs.Src, chains[src], cmd); err != nil {
					return err
				}
			} else if len(msgs.Dst) > 0 {
				if err = SendAndPrint(msgs.Dst, chains[dst], cmd); err != nil {
					return err
				}
			}

			return nil
		},
	}

	return transactionFlags(cmd)
}

func fullPathCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "full-path [src-chain-id] [dst-chain-id] [index]",
		Short: "create all primitaves necessary to send packets between two configured chains with a configured path",
		Long:  "FYI: DRAGONS HERE, not tested",
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			to, err := getTimeout(cmd)
			if err != nil {
				return err
			}

			// Find any configured paths between the chains
			paths, err := config.Paths.PathsFromChains(src, dst)
			if err != nil {
				return err
			}

			// Given the number of args and the number of paths,
			// work on the appropriate path
			var path relayer.Path
			switch {
			case len(args) == 3 && len(paths) > 1:
				i, err := strconv.ParseInt(args[2], 10, 64)
				if err != nil {
					return err
				}
				path = paths[i]
			case len(args) == 3 && len(paths) == 1:
				fmt.Println(paths.MustYAML())
				return fmt.Errorf("passed in an index where only one path exists between chains %s and %s", src, dst)
			case len(args) == 2 && len(paths) > 1:
				fmt.Println(paths.MustYAML())
				return fmt.Errorf("more than one path between %s and %s exists, please specify index", src, dst)
			case len(args) == 2 && len(paths) == 1:
				path = paths[0]
			}

			if err = chains[src].SetPath(path.End(src), relayer.FULLPATH); err != nil {
				return chains[src].ErrCantSetPath(relayer.FULLPATH, err)
			}

			if err = chains[dst].SetPath(path.End(dst), relayer.FULLPATH); err != nil {
				return chains[dst].ErrCantSetPath(relayer.FULLPATH, err)
			}

			hs, err := relayer.UpdatesWithHeaders(chains[src], chains[dst])
			if err != nil {
				return err
			}

			srcCs, err := chains[src].QueryClientState()
			if err != nil {
				return err
			}

			logger := log.NewTMLogger(log.NewSyncWriter(os.Stdout))

			if srcCs.ClientState == nil {
				res, err := chains[src].SendMsg(chains[src].CreateClient(hs[dst]))
				if err != nil || res.Code != 0 {
					if err = PrintOutput(res, cmd); err != nil {
						return err
					}
				}
				logger.Info(fmt.Sprintf("tx.hash=%s", res.TxHash), "chain", src, "action", "create-client")
			}

			dstCs, err := chains[dst].QueryClientState()
			if err != nil {
				return err
			}

			if dstCs.ClientState == nil {
				res, err := chains[dst].SendMsg(chains[dst].CreateClient(hs[src]))
				if err != nil || res.Code != 0 {
					if err = PrintOutput(res, cmd); err != nil {
						return err
					}
				}
				logger.Info(fmt.Sprintf("tx.hash=%s", res.TxHash), "chain", dst, "action", "create-client")
			}

			ticker := time.NewTicker(to)
			for ; true; <-ticker.C {
				msgs, err := chains[src].CreateConnectionStep(chains[dst])
				if err != nil {
					return err
				}

				if !msgs.Ready() {
					break
				}

				if len(msgs.Src) > 0 {
					// Submit the transactions to src chain
					res, err := chains[src].SendMsgs(msgs.Src)
					if err != nil || res.Code != 0 {
						if err = PrintOutput(res, cmd); err != nil {
							return err
						}
					}
					logger.Info(fmt.Sprintf("tx.hash=%s", res.TxHash), "chain", src, "action", getMsgAction(msgs.Src))
				}

				if len(msgs.Dst) > 0 {
					// Submit the transactions to dst chain
					res, err := chains[dst].SendMsgs(msgs.Dst)
					if err != nil || res.Code != 0 {
						if err = PrintOutput(res, cmd); err != nil {
							return err
						}
					}
					logger.Info(fmt.Sprintf("tx.hash=%s", res.TxHash), "chain", dst, "action", getMsgAction(msgs.Dst))
				}
			}

			var order chanState.Order
			if order = chanState.OrderFromString("ORDERED"); order == chanState.NONE {
				return fmt.Errorf("invalid order passed in, expected 'UNORDERED' or 'ORDERED'")
			}

			ticker = time.NewTicker(to)
			for ; true; <-ticker.C {
				msgs, err := chains[src].CreateChannelStep(chains[dst], order)
				if err != nil {
					return err
				}

				if !msgs.Ready() {
					break
				}

				if len(msgs.Src) > 0 {
					// Submit the transactions to src chain
					res, err := chains[src].SendMsgs(msgs.Src)
					if err != nil || res.Code != 0 {
						if err = PrintOutput(res, cmd); err != nil {
							return err
						}
					}
					logger.Info(fmt.Sprintf("tx.hash=%s", res.TxHash), "chain", src, "action", getMsgAction(msgs.Src))
				}

				if len(msgs.Dst) > 0 {
					// Submit the transactions to dst chain
					res, err := chains[dst].SendMsgs(msgs.Dst)
					if err != nil || res.Code != 0 {
						if err = PrintOutput(res, cmd); err != nil {
							return err
						}
					}
					logger.Info(fmt.Sprintf("tx.hash=%s", res.TxHash), "chain", dst, "action", getMsgAction(msgs.Dst))
				}
			}
			return nil
		},
	}

	return timeoutFlag(transactionFlags(cmd))
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
