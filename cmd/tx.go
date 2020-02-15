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
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
)

func init() {
	transactionCmd.AddCommand(createClientCmd())
	transactionCmd.AddCommand(createClientsCmd())
	transactionCmd.AddCommand(createConnectionCmd())
	transactionCmd.AddCommand(createChannelCmd())
	transactionCmd.AddCommand(updateClientCmd())
	transactionCmd.AddCommand(rawTransactionCmd)
	rawTransactionCmd.AddCommand(connTry())
	rawTransactionCmd.AddCommand(connAck())
	rawTransactionCmd.AddCommand(connConfirm())
	rawTransactionCmd.AddCommand(chanInit())
	rawTransactionCmd.AddCommand(chanTry())
	rawTransactionCmd.AddCommand(chanAck())
	rawTransactionCmd.AddCommand(chanConfirm())
	rawTransactionCmd.AddCommand(chanCloseInit())
	rawTransactionCmd.AddCommand(chanCloseConfirm())
}

// transactionCmd represents the tx command
var transactionCmd = &cobra.Command{
	Use:     "transactions",
	Aliases: []string{"tx"},
	Short:   "IBC Transaction Commands, UNDER CONSTRUCTION",
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

			errs := relayer.UpdateLiteDBsToLatestHeaders(chains[src], chains[dst])
			if len(errs) != 0 {
				for _, err := range errs {
					fmt.Println(err)
				}
				return nil
			}

			dstHeader, err := chains[dst].GetLatestLiteHeader()
			if err != nil {
				return err
			}

			res, err := chains[src].SendMsg(chains[src].UpdateClient(args[2], dstHeader))
			if err != nil {
				return err
			}

			return PrintOutput(res, cmd)
		},
	}
	return outputFlags(cmd)
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

			errs := relayer.UpdateLiteDBsToLatestHeaders(chains[src], chains[dst])
			if len(errs) != 0 {
				for _, err := range errs {
					fmt.Println(err)
				}
				return nil
			}

			dstHeader, err := chains[dst].GetLatestLiteHeader()
			if err != nil {
				return err
			}

			res, err := chains[src].SendMsg(chains[src].CreateClient(args[2], dstHeader))
			if err != nil {
				return err
			}

			return PrintOutput(res, cmd)
		},
	}

	return outputFlags(cmd)
}

func createClientsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clients [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id]",
		Short: "create a clients for dst-chain on src-chain and src-chain on dst-chain",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			errs := relayer.UpdateLiteDBsToLatestHeaders(chains[src], chains[dst])
			if len(errs) != 0 {
				for _, err := range errs {
					fmt.Println(err)
					return nil
				}
			}

			headers, errs := relayer.GetLatestHeaders(chains[src], chains[dst])
			if len(errs) != 0 {
				for _, err := range errs {
					fmt.Println(err)
					return nil
				}
			}

			res, err := chains[src].SendMsg(chains[src].CreateClient(args[2], headers.Map[dst]))
			if err != nil {
				return err
			}

			err = PrintOutput(res, cmd)
			if err != nil {
				return err
			}

			res, err = chains[dst].SendMsg(chains[dst].CreateClient(args[3], headers.Map[src]))
			if err != nil {
				return err
			}

			err = PrintOutput(res, cmd)
			if err != nil {
				return err
			}

			return nil
		},
	}
	return outputFlags(cmd)
}

func createConnectionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connection [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] [src-connection-id], [dst-connection-id]",
		Short: "create a connection between chains, passing in identifiers",
		Args:  cobra.ExactArgs(6),
		RunE: func(cmd *cobra.Command, args []string) error {
			timeout := 5 * time.Second
			src, dst := args[0], args[1]
			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			// TODO: validate identifiers ICS24

			err = chains[src].CreateConnection(chains[dst], args[2], args[3], args[4], args[5], timeout)
			if err != nil {
				return err
			}

			return nil
		},
	}

	return cmd
}

func createChannelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channel [src-chain-id] [dst-chain-id] [src-connection-id] [dst-connection-id] [src-channel-id] [dst-channel-id] [src-port-id] [dst-port-id]",
		Short: "",
		Args:  cobra.ExactArgs(8),
		RunE: func(cmd *cobra.Command, args []string) error {
			timeout := 5 * time.Second
			src, dst := args[0], args[1]
			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			// TODO: validate identifiers ICS24

			err = chains[src].CreateChannel(chains[dst], args[2], args[3], args[4], args[5], args[6], args[7], timeout)
			if err != nil {
				return err
			}

			return nil
		},
	}

	return cmd
}

////////////////////////////////////////
////  RAW IBC TRANSACTION COMMANDS  ////
////////////////////////////////////////

var rawTransactionCmd = &cobra.Command{
	Use:   "raw",
	Short: "raw connection and channel steps",
}

func connInit() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conn-init [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] [src-conn-id] [dst-conn-id]",
		Short: "conn-init",
		Args:  cobra.ExactArgs(6),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, _ := args[0], args[1]
			srcClient, dstClient := args[2], args[3]
			srcConn, dstConn := args[4], args[5]
			srcChain, err := config.c.GetChain(src)
			if err != nil {
				return err
			}

			res, err := srcChain.SendMsg(
				srcChain.ConnInit(srcConn, srcClient, dstConn, dstClient))

			if err != nil {
				return nil
			}

			return PrintOutput(res, cmd)
		},
	}
	return outputFlags(cmd)
}

func connTry() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conn-try [src-chain-id] [dst-chain-id] [src-client-id] [dst-client-id] [src-conn-id] [dst-conn-id]",
		Short: "conn-try",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			srcClient, dstClient := args[2], args[3]
			srcConn, dstConn := args[4], args[5]
			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			errs := relayer.UpdateLiteDBsToLatestHeaders(chains[src], chains[dst])
			if len(errs) != 0 {
				for _, err := range errs {
					fmt.Println(err)
					return nil
				}
			}

			headers, errs := relayer.GetLatestHeaders(chains[src], chains[dst])
			if len(errs) != 0 {
				for _, err := range errs {
					fmt.Println(err)
					return nil
				}
			}

			dstState, err := chains[dst].QueryConnection(dstConn, headers.Map[dst].Height)
			if err != nil {
				return err
			}

			res, err := chains[src].SendMsgs([]sdk.Msg{
				chains[src].ConnTry(srcClient, dstClient, srcConn, dstConn, dstState, headers.Map[src].Height),
				chains[src].UpdateClient(srcClient, headers.Map[dst])})

			if err != nil {
				return err
			}

			return PrintOutput(res, cmd)
		},
	}
	return outputFlags(cmd)
}

func connAck() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conn-ack [src-chain-id] [dst-chain-id] [src-conn-id] [dst-conn-id] [src-client-id]",
		Short: "conn-ack",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			srcConn, dstConn := args[2], args[3]
			srcClient := args[4]
			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			errs := relayer.UpdateLiteDBsToLatestHeaders(chains[src], chains[dst])
			if len(errs) != 0 {
				for _, err := range errs {
					fmt.Println(err)
					return nil
				}
			}

			headers, errs := relayer.GetLatestHeaders(chains[src], chains[dst])
			if len(errs) != 0 {
				for _, err := range errs {
					fmt.Println(err)
					return nil
				}
			}

			dstState, err := chains[dst].QueryConnection(dstConn, headers.Map[dst].Height)
			if err != nil {
				return err
			}

			res, err := chains[src].SendMsgs([]sdk.Msg{
				chains[src].ConnAck(srcConn, dstState, headers.Map[src].Height),
				chains[src].UpdateClient(srcClient, headers.Map[dst])})

			if err != nil {
				return nil
			}

			return PrintOutput(res, cmd)
		},
	}
	return outputFlags(cmd)
}

func connConfirm() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conn-confirm [src-chain-id] [dst-chain-id] [src-conn-id] [dst-conn-id] [src-client-id]",
		Short: "conn-confirm",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			srcConn, dstConn := args[2], args[3]
			srcClient := args[4]
			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			errs := relayer.UpdateLiteDBsToLatestHeaders(chains[src], chains[dst])
			if len(errs) != 0 {
				for _, err := range errs {
					fmt.Println(err)
					return nil
				}
			}

			headers, errs := relayer.GetLatestHeaders(chains[src], chains[dst])
			if len(errs) != 0 {
				for _, err := range errs {
					fmt.Println(err)
					return nil
				}
			}

			dstState, err := chains[dst].QueryConnection(dstConn, headers.Map[dst].Height)
			if err != nil {
				return err
			}

			res, err := chains[src].SendMsgs([]sdk.Msg{
				chains[src].ConnConfirm(srcConn, dstState, headers.Map[src].Height),
				chains[src].UpdateClient(srcClient, headers.Map[dst])})

			if err != nil {
				return nil
			}

			return PrintOutput(res, cmd)
		},
	}
	return outputFlags(cmd)
}

func chanInit() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chan-init [src-chain-id] [dst-chain-id]",
		Short: "chan-init",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			errs := relayer.UpdateLiteDBsToLatestHeaders(chains[src], chains[dst])
			if len(errs) != 0 {
				for _, err := range errs {
					fmt.Println(err)
					return nil
				}
			}

			return PrintOutput(errs, cmd)
		},
	}
	return outputFlags(cmd)
}

func chanTry() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chan-try [src-chain-id] [dst-chain-id]",
		Short: "chan-try",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			errs := relayer.UpdateLiteDBsToLatestHeaders(chains[src], chains[dst])
			if len(errs) != 0 {
				for _, err := range errs {
					fmt.Println(err)
					return nil
				}
			}

			return PrintOutput(errs, cmd)
		},
	}
	return outputFlags(cmd)
}

func chanAck() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chan-ack [src-chain-id] [dst-chain-id]",
		Short: "chan-ack",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			errs := relayer.UpdateLiteDBsToLatestHeaders(chains[src], chains[dst])
			if len(errs) != 0 {
				for _, err := range errs {
					fmt.Println(err)
					return nil
				}
			}

			return PrintOutput(errs, cmd)
		},
	}
	return outputFlags(cmd)
}

func chanConfirm() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chan-confirm [src-chain-id] [dst-chain-id]",
		Short: "chan-confirm",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			errs := relayer.UpdateLiteDBsToLatestHeaders(chains[src], chains[dst])
			if len(errs) != 0 {
				for _, err := range errs {
					fmt.Println(err)
					return nil
				}
			}

			return PrintOutput(errs, cmd)
		},
	}
	return outputFlags(cmd)
}

func chanCloseInit() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chan-close-init [src-chain-id] [dst-chain-id]",
		Short: "chan-close-init",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			errs := relayer.UpdateLiteDBsToLatestHeaders(chains[src], chains[dst])
			if len(errs) != 0 {
				for _, err := range errs {
					fmt.Println(err)
					return nil
				}
			}

			return PrintOutput(errs, cmd)
		},
	}
	return outputFlags(cmd)
}

func chanCloseConfirm() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chan-close-confirm [src-chain-id] [dst-chain-id]",
		Short: "chan-close-confirm",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.c.GetChains(src, dst)
			if err != nil {
				return err
			}

			errs := relayer.UpdateLiteDBsToLatestHeaders(chains[src], chains[dst])
			if len(errs) != 0 {
				for _, err := range errs {
					fmt.Println(err)
					return nil
				}
			}

			return PrintOutput(errs, cmd)
		},
	}
	return outputFlags(cmd)
}
