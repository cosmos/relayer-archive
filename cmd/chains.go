package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
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
		chainsEditCmd(),
		chainsShowCmd(),
	)

	return cmd
}

func chainsShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show [chain-id]",
		Short: "Returns a chain's configuration data",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := yaml.Marshal(config.Chains.Get(args[0]))
			if err != nil {
				return err
			}
			fmt.Println(string(out))
			return nil
		},
	}
	return cmd

}

func chainsEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit [chain-id] [key] [value]",
		Short: "Returns chain configuration data",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := config.Chains.Get(args[0]).Update(args[1], args[2])
			if err != nil {
				return err
			}
			config.DeleteChain(args[0])
			config.AddChain(c)
			return overWriteConfig(cmd, config)
		},
	}
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
			c := ChainConfig{}
			var err error

			// TODO: figure out how to parse gaia style configs to pull out necessary data
			// for chain configuration
			//
			// url, err := getURL(cmd)
			// if err != nil {
			// 	return err
			// }

			// if len(url) > 0 {
			// 	cl, err := rpcclient.NewHTTP(url, "/websocket")
			// 	if err != nil {
			// 		return err
			// 	}
			// 	gen, err := cl.Genesis()
			// 	if err != nil {
			// 		return err
			// 	}
			// }

			var value string
			fmt.Println("ChainID (i.e. cosmoshub2):")
			if value, err = readStdin(); err != nil {
				return err
			}

			if c, err = c.Update("chain-id", value); err != nil {
				return err
			}

			fmt.Println("Default Key (i.e. testkey):")
			if value, err = readStdin(); err != nil {
				return err
			}

			if c, err = c.Update("key", value); err != nil {
				return err
			}

			fmt.Println("RPC Address (i.e. http://localhost:26657):")
			if value, err = readStdin(); err != nil {
				return err
			}

			if c, err = c.Update("rpc-addr", value); err != nil {
				return err
			}

			fmt.Println("Account Prefix (i.e. cosmos):")
			if value, err = readStdin(); err != nil {
				return err
			}

			if c, err = c.Update("account-prefix", value); err != nil {
				return err
			}

			fmt.Println("Gas (i.e. 200000):")
			if value, err = readStdin(); err != nil {
				return err
			}

			if c, err = c.Update("gas", value); err != nil {
				return err
			}

			fmt.Println("Gas Prices (i.e. 0.025stake):")
			if value, err = readStdin(); err != nil {
				return err
			}

			if c, err = c.Update("gas-prices", value); err != nil {
				return err
			}

			fmt.Println("Default Denom (i.e. stake):")
			if value, err = readStdin(); err != nil {
				return err
			}

			if c, err = c.Update("default-denom", value); err != nil {
				return err
			}

			fmt.Println("Trusting Period (i.e. 336h)")
			if value, err = readStdin(); err != nil {
				return err
			}

			if c, err = c.Update("trusting-period", value); err != nil {
				return err
			}

			// TODO: ensure that there are no other configured chains with the same ID
			return overWriteConfig(cmd, config.AddChain(c))
		},
	}
	return cmd
}
