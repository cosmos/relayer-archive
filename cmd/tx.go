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
	"strconv"

	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
)

// transactionCmd represents the tx command
func transactionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "transactions",
		Aliases: []string{"tx"},
		Short:   "IBC Transaction Commands",
	}

	cmd.AddCommand(
		createClientsCmd(),
		createConnectionCmd(),
		createChannelCmd(),
		fullPathCmd(),
		rawTransactionCmd(),
	)

	return cmd
}

func createClientsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clients [src-chain-id] [dst-chain-id] [index]",
		Short: "create a clients between two configured chains with a configured path",
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.Chains.Gets(src, dst)
			if err != nil {
				return err
			}

			if _, err = setPathsFromArgs(chains[src], chains[dst], args); err != nil {
				return err
			}

			return chains[src].CreateClients(chains[dst], cmd)
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
			chains, err := config.Chains.Gets(src, dst)
			if err != nil {
				return err
			}

			to, err := getTimeout(cmd)
			if err != nil {
				return err
			}

			if _, err = setPathsFromArgs(chains[src], chains[dst], args); err != nil {
				return err
			}

			return chains[src].CreateConnection(chains[dst], to, cmd)
		},
	}

	return timeoutFlag(transactionFlags(cmd))
}

func createChannelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channel [src-chain-id] [dst-chain-id] [index]",
		Short: "create a channel between two configured chains with a configured path",
		Long:  "This command is meant to be used to repair or create a channel between two chains with a configured path in the config file",
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.Chains.Gets(src, dst)
			if err != nil {
				return err
			}

			to, err := getTimeout(cmd)
			if err != nil {
				return err
			}

			if _, err = setPathsFromArgs(chains[src], chains[dst], args); err != nil {
				return err
			}

			return chains[src].CreateChannel(chains[dst], true, to, cmd)
		},
	}

	return timeoutFlag(transactionFlags(cmd))
}

func fullPathCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "full-path [src-chain-id] [dst-chain-id] [index]",
		Short: "create clients, connection, and channel between two configured chains with a configured path",
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			chains, err := config.Chains.Gets(src, dst)
			if err != nil {
				return err
			}

			to, err := getTimeout(cmd)
			if err != nil {
				return err
			}

			if _, err = setPathsFromArgs(chains[src], chains[dst], args); err != nil {
				return err
			}

			// Check if clients have been created, if not create them
			if err = chains[src].CreateClients(chains[dst], cmd); err != nil {
				return err
			}

			// Check if connection has been created, if not create it
			if err = chains[src].CreateConnection(chains[dst], to, cmd); err != nil {
				return err
			}

			// NOTE: this is hardcoded to create ordered channels right now. Add a flag here to toggle
			// Check if channel has been created, if not create it
			return chains[src].CreateChannel(chains[dst], true, to, cmd)
		},
	}

	return timeoutFlag(transactionFlags(cmd))
}

func setPathsFromArgs(src, dst *relayer.Chain, args []string) (*relayer.Path, error) {
	// Find any configured paths between the chains
	paths, err := config.Paths.PathsFromChains(src.ChainID, dst.ChainID)
	if err != nil {
		return nil, err
	}

	// Given the number of args and the number of paths,
	// work on the appropriate path
	var path *relayer.Path
	switch {
	case len(args) == 3 && len(paths) > 1:
		i, err := strconv.ParseInt(args[2], 10, 64)
		if err != nil {
			return nil, err
		}
		path = paths[i]
	case len(args) == 3 && len(paths) == 1:
		fmt.Println(paths.MustYAML())
		return nil, fmt.Errorf("passed in an index where only one path exists between chains %s and %s", src, dst)
	case len(args) == 2 && len(paths) > 1:
		fmt.Println(paths.MustYAML())
		return nil, fmt.Errorf("more than one path between %s and %s exists, please specify index", src, dst)
	case len(args) == 2 && len(paths) == 1:
		path = paths[0]
	}

	if err = src.SetPath(path.End(src.ChainID), relayer.FULLPATH); err != nil {
		return nil, src.ErrCantSetPath(relayer.FULLPATH, err)
	}

	if err = dst.SetPath(path.End(dst.ChainID), relayer.FULLPATH); err != nil {
		return nil, dst.ErrCantSetPath(relayer.FULLPATH, err)
	}

	return path, nil
}
