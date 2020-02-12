/*
Copyright © 2020 Jack Zampolin <jack.zampolin@gmail.com>

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
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	neturl "net/url"
	"strconv"

	tmclient "github.com/cosmos/cosmos-sdk/x/ibc/07-tendermint"
	"github.com/cosmos/relayer/relayer"
	"github.com/spf13/cobra"
	lite "github.com/tendermint/tendermint/lite2"
)

var (
	flagHeight = "height"
	flagHash   = "hash"
	flagURL    = "url"
	flagForce  = "force"
)

// chainCmd represents the keys command
var liteCmd = &cobra.Command{
	Use:   "lite",
	Short: "basic functionality for managing the lite clients",
}

func init() {
	for _, cmd := range []*cobra.Command{initLiteCmd(), updateLiteCmd()} {
		cmd.Flags().Int64P(flagHeight, "", -1, "Trusted header's height")
		cmd.Flags().BytesHexP(flagHash, "x", []byte{}, "Trusted header's hash")
		cmd.Flags().StringP(flagURL, "u", "", "Optional URL to fetch trusted-hash and trusted-height")
		cmd.Flags().BoolP(flagForce, "f", false, "Option to skip confirmation prompt for trusting hash & height from configured url")

		liteCmd.AddCommand(cmd)
	}

	liteCmd.AddCommand(headerCmd())
	liteCmd.AddCommand(latestHeightCmd())
}

func initLiteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [chain-id]",
		Short: "Initiate the light client",
		Long: `Initiate the light client by passing it a root of trust as a --hash
		and --height, either directly or via --url. Use --force to skip
		confirmation prompt.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.c.GetChain(args[0])
			if err != nil {
				return err
			}

			db, df, err := chain.NewLiteDB()
			if err != nil {
				return fmt.Errorf("can't open db connection: %w", err)
			}
			defer df()

			height, _ := cmd.Flags().GetInt64(flagHeight)
			hash, _ := cmd.Flags().GetBytesHex(flagHash)
			url, _ := cmd.Flags().GetString(flagURL)

			switch {
			case height > 0 && len(hash) > 0: // height and hash are given
				_, err = chain.InitLiteClient(db, chain.TrustOptions(height, hash))
				if err != nil {
					return fmt.Errorf("init failed: %w", err)
				}
			case len(url) > 0: // URL is given
				_, err := neturl.Parse(url)
				if err != nil {
					return fmt.Errorf("incorrect url: %w", err)
				}
				// TODO: we should use the given url and force flag here
				//
				// initialize the lite client database by querying the configured node
				_, err = chain.TrustNodeInitClient(db)
				if err != nil {
					return fmt.Errorf("init failed: %w", err)
				}
			default:
				return errors.New("expected either --hash & --height OR --url, none given")
			}

			return nil
		},
	}

	return cmd
}

func updateLiteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update [chain-id]",
		Short: "Update the light client by providing a new root of trust",
		Long: `Update the light client by providing a new root of trust as a --hash
		and --height, either directly or via --url. Use --force to skip
		confirmation prompt.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			chain, err := config.c.GetChain(args[0])
			if err != nil {
				return err
			}

			height, _ := cmd.Flags().GetInt64(flagHeight)
			hash, _ := cmd.Flags().GetBytesHex(flagHash)
			url, _ := cmd.Flags().GetString(flagURL)

			switch {
			case height > 0 && len(hash) > 0: // height and hash are given
				db, df, err := chain.NewLiteDB()
				if err != nil {
					return fmt.Errorf("can't open db connection: %w", err)
				}
				defer df()

				_, err = chain.InitLiteClient(db, chain.TrustOptions(height, hash))
				if err != nil {
					return fmt.Errorf("init failed: %w", err)
				}
			case len(url) > 0: // URL is given
				_, err := neturl.Parse(url)
				if err != nil {
					return fmt.Errorf("incorrect url: %w", err)
				}

				db, df, err := chain.NewLiteDB()
				if err != nil {
					return fmt.Errorf("can't open db connection: %w", err)
				}
				defer df()

				// TODO: we should use the given url and force flag here
				//
				// initialize the lite client database by querying the configured node
				_, err = chain.TrustNodeInitClient(db)
				if err != nil {
					return fmt.Errorf("init failed: %w", err)
				}
			default: // nothing is given => update existing client
				// NOTE: "Update the light client by providing a new root of trust"
				// does not mention this at all. I mean that we can update existing
				// client by calling "update [chain-id]".
				//
				// Since first two conditions essentially repeat initLiteCmd above, I
				// think we should remove first two conditions here and just make
				// updateLiteCmd only about updating the light client to latest header
				// (i.e. not mix responsibilities).
				err = chain.UpdateLiteDBToLatestHeader()
				if err != nil {
					return fmt.Errorf("can't update to latest header: %w", err)
				}
			}

			return nil
		},
	}
	return cmd
}

func headerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "header [chain-id] [height]",
		Short: "Get header from the database. 0 returns last trusted header and " +
			"all others return the header at that height if stored",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			chainID := args[0]
			chain, err := config.c.GetChain(chainID)
			if err != nil {
				return err
			}

			var header *tmclient.Header

			switch len(args) {
			case 1:
				header, err = chain.GetLatestLiteHeader()
				if err != nil {
					return err
				}
			case 2:
				var height int64
				height, err = strconv.ParseInt(args[1], 10, 64) //convert to int64
				if err != nil {
					return err
				}

				if height == 0 {
					height, err = chain.GetLatestLiteHeight()
					if err != nil {
						return err
					}

					if height == -1 {
						return relayer.ErrLiteNotInitialized
					}
				}

				header, err = chain.GetLiteSignedHeaderAtHeight(height)
				if err != nil {
					return err
				}

			}

			out, err := chain.Cdc.MarshalJSON(header)
			if err != nil {
				return err
			}

			fmt.Println(string(out))
			return nil
		},
	}
	return cmd
}

func latestHeightCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "latest-height [chain-id]",
		Short: "Get header from relayer database. 0 returns last trusted header and " +
			"all others return the header at that height if stored",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			chainID := args[0]
			chain, err := config.c.GetChain(chainID)
			if err != nil {
				return err
			}

			// Get stored height
			height, err := chain.GetLatestLiteHeader()
			if err != nil {
				return err
			}

			fmt.Println(height.Height)
			return nil
		},
	}
	return cmd
}

func queryTrustOptions(url string) (out lite.TrustOptions, err error) {
	// fetch from URL
	res, err := http.Get(url)
	if err != nil {
		return
	}

	// read in the res body
	bz, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return
	}

	// close the response body
	err = res.Body.Close()
	if err != nil {
		return
	}

	// unmarshal the data into the trust options hash
	err = json.Unmarshal(bz, &out)
	if err != nil {
		return
	}

	return
}
