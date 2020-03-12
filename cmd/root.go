/*
Copyright © 2020 Jack Zampolin jack.zampolin@gmail.com

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
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/codec"
	codecstd "github.com/cosmos/cosmos-sdk/codec/std"
	"github.com/cosmos/cosmos-sdk/simapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	"gopkg.in/yaml.v2"
)

var (
	cfgPath     string
	homePath    string
	config      *Config
	defaultHome = os.ExpandEnv("$HOME/.relayer")
	cdc         *codec.Codec
	appCodec    *codecstd.Codec
)

func init() {
	// Register top level flags --home and --config
	rootCmd.PersistentFlags().StringVar(&homePath, flags.FlagHome, defaultHome, "set home directory")
	rootCmd.PersistentFlags().StringVar(&cfgPath, flagConfig, "config.yaml", "set config file")
	viper.BindPFlag(flags.FlagHome, rootCmd.Flags().Lookup(flags.FlagHome))
	viper.BindPFlag(flagConfig, rootCmd.Flags().Lookup(flagConfig))

	// Register subcommands
	rootCmd.AddCommand(
		liteCmd,
		keysCmd,
		queryCmd,
		startCmd,
		transactionCmd(),
		chainsCmd(),
		pathsCommand(),
		configCmd(),
	)

	// This is a bit of a cheat :shushing_face:
	// TODO: Remove cdc in favor of appCodec once all modules are migrated.
	cdc = codecstd.MakeCodec(simapp.ModuleBasics)

	appCodec = codecstd.NewAppCodec(cdc)
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "relayer",
	Short: "This application relays data between configured IBC enabled chains",
}

func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Returns configuration data",
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := yaml.Marshal(config)
			if err != nil {
				return err
			}

			fmt.Println(string(out))
			return nil
		},
	}

	return cmd
}

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

func pathsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "paths",
		Short: "print out configured paths with direction",
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, p := range config.Paths {
				fmt.Println(p.String())
			}
			return nil
		},
	}
	return cmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	rootCmd.PersistentPreRunE = func(_ *cobra.Command, _ []string) error {
		// reads `homeDir/config/config.yaml` into `var config *Config` before each command
		return initConfig(rootCmd)
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// readLineFromBuf reads one line from stdin.
func readStdin() (string, error) {
	str, err := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimSpace(str), err
}
