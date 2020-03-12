package cmd

import (
	"fmt"
	"strconv"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/spf13/cobra"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	"gopkg.in/yaml.v2"
)

func chainsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chains",
		Short: "commands to configure chains",
	}

	cmd.AddCommand(
		chainsListCmd(),
		chainsDeleteCmd(),
		chainsAddCmd(),
	)

	return cmd
}

func chainsDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [chain-id]",
		Short: "Returns chain configuration data",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return overWriteConfig(cmd, config.DeleteChain(args[0]))
		},
	}
	return cmd
}

func chainsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Returns chain configuration data",
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := yaml.Marshal(config.Chains)
			if err != nil {
				return err
			}
			fmt.Println(string(out))
			return nil
		},
	}
	return cmd
}

func chainsAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Reads in a series of user input and generates a new chain in the config",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("ChainID (i.e. cosmoshub2):")
			cid, err := readStdin()
			if err != nil {
				return err
			}

			fmt.Println("Default Key (i.e. testkey):")
			key, err := readStdin()
			if err != nil {
				return err
			}

			fmt.Println("RPC Address (i.e. http://localhost:26657):")
			rpc, err := readStdin()
			if err != nil {
				return err
			}

			if _, err = rpcclient.NewHTTP(rpc, "/websocket"); err != nil {
				return err
			}

			fmt.Println("Account Prefix (i.e. cosmos):")
			accPrefix, err := readStdin()
			if err != nil {
				return err
			}

			fmt.Println("Gas (i.e. 200000):")
			g, err := readStdin()
			if err != nil {
				return err
			}

			gas, err := strconv.ParseInt(g, 10, 64)
			if err != nil {
				return err
			}

			fmt.Println("gas-prices (i.e. 0.025stake):")
			gasPrices, err := readStdin()
			if err != nil {
				return err
			}

			if _, err = sdk.ParseDecCoins(gasPrices); err != nil {
				return err
			}

			fmt.Println("Default Denom (i.e. stake):")
			denom, err := readStdin()
			if err != nil {
				return err
			}

			fmt.Println("Trusting Period (i.e. 336h)")
			trustingPeriod, err := readStdin()
			if err != nil {
				return err
			}

			if _, err = time.ParseDuration(trustingPeriod); err != nil {
				return err
			}

			return overWriteConfig(cmd, config.AddChain(ChainConfig{
				Key:            key,
				ChainID:        cid,
				RPCAddr:        rpc,
				AccountPrefix:  accPrefix,
				Gas:            uint64(gas),
				GasPrices:      gasPrices,
				DefaultDenom:   denom,
				TrustingPeriod: trustingPeriod,
			}))
		},
	}
	return cmd
}
