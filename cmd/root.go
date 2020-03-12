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
	"path"
	"strings"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/codec"
	codecstd "github.com/cosmos/cosmos-sdk/codec/std"
	"github.com/cosmos/cosmos-sdk/simapp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
			home, err := cmd.Flags().GetString(flags.FlagHome)
			if err != nil {
				return err
			}

			cfgPath := path.Join(home, "config", "config.yaml")
			if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
				if _, err := os.Stat(home); os.IsNotExist(err) {
					return fmt.Errorf("Home path does not exist: %s", home)
				}
				return fmt.Errorf("Config does not exist: %s", cfgPath)
			}

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

func initCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Creates a default home directory at path defined by --home",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
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
