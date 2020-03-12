package cmd

import (
	"fmt"
	"strconv"

	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

func pathsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "paths",
		Short: "commands to manage path configurations",
	}

	cmd.AddCommand(
		pathsListCmd(),
		pathsShowCmd(),
		pathsAddCmd(),
	)

	return cmd
}

func pathsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "print out configured paths with direction",
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, p := range config.Paths.SetIndices() {
				fmt.Println(p.Index)
				out, err := yaml.Marshal(p)
				if err != nil {
					return err
				}
				fmt.Println(string(out))
			}
			return nil
		},
	}
	return cmd
}

func pathsShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show [index]",
		Short: "show a path at a given index",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			index, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return err
			}
			if len(config.Paths) > int(index+1) {
				return fmt.Errorf("index %d out of range, %d paths configured", index, len(config.Paths))
			}

			fmt.Println(config.Paths[index].MustYAML())
			return nil
		},
	}
	return cmd
}

func pathsAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add [src-chain-id] [dst-chain-id]",
		Short: "add a path to the list of paths",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			_, err := config.c.GetChains(src, dst)
			if err != nil {
				return fmt.Errorf("chains need to be configured before paths to them can be added: %w", err)
			}

			path := relayer.Path{
				Src: &relayer.PathEnd{
					ChainID: src,
				},
				Dst: &relayer.PathEnd{
					ChainID: dst,
				},
			}

			var value string
			fmt.Printf("enter src(%s) client-id...\n", src)
			if value, err = readStdin(); err != nil {
				return err
			}

			path.Src.ClientID = value

			if err = path.Src.Vclient(); err != nil {
				return err
			}

			fmt.Printf("enter src(%s) connection-id...\n", src)
			if value, err = readStdin(); err != nil {
				return err
			}

			path.Src.ConnectionID = value

			if err = path.Src.Vconn(); err != nil {
				return err
			}

			fmt.Printf("enter src(%s) channel-id...\n", src)
			if value, err = readStdin(); err != nil {
				return err
			}

			path.Src.ChannelID = value

			if err = path.Src.Vchan(); err != nil {
				return err
			}

			fmt.Printf("enter src(%s) port-id...\n", src)
			if value, err = readStdin(); err != nil {
				return err
			}

			path.Src.PortID = value

			if err = path.Src.Vport(); err != nil {
				return err
			}

			fmt.Printf("enter dst(%s) client-id...\n", dst)
			if value, err = readStdin(); err != nil {
				return err
			}

			path.Dst.ClientID = value

			if err = path.Dst.Vclient(); err != nil {
				return err
			}

			fmt.Printf("enter dst(%s) connection-id...\n", dst)
			if value, err = readStdin(); err != nil {
				return err
			}

			path.Dst.ConnectionID = value

			if err = path.Dst.Vconn(); err != nil {
				return err
			}

			fmt.Printf("enter dst(%s) channel-id...\n", dst)
			if value, err = readStdin(); err != nil {
				return err
			}

			path.Dst.ChannelID = value

			if err = path.Dst.Vchan(); err != nil {
				return err
			}

			fmt.Printf("enter dst(%s) port-id...\n", dst)
			if value, err = readStdin(); err != nil {
				return err
			}

			path.Dst.PortID = value

			if err = path.Dst.Vport(); err != nil {
				return err
			}

			return overWriteConfig(cmd, config.AddPath(path))
		},
	}
	return cmd
}
