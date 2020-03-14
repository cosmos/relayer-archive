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
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

// startCmd represents the start command
// NOTE: This is basically psuedocode
var startCmd = &cobra.Command{
	Use:   "start [src-chain-id] [dst-chain-id] [index]",
	Short: "starts the relayer using the configured chains and strategy",
	Args:  cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		src, dst := args[0], args[1]
		chains, err := config.c.GetChains(src, dst)
		if err != nil {
			return err
		}

		if err = setPathsFromArgs(chains[src], chains[dst], args); err != nil {
			return err
		}

		events := "tm.event = 'NewBlock'"

		srcEvents, srcCancel, err := chains[src].Subscribe(events)
		if err != nil {
			return err
		}
		defer srcCancel()

		dstEvents, dstCancel, err := chains[dst].Subscribe(events)
		if err != nil {
			return err
		}
		defer dstCancel()

		var sigCh = make(chan os.Signal)
		defer close(sigCh)

		signal.Notify(sigCh, syscall.SIGTERM)
		signal.Notify(sigCh, syscall.SIGINT)

		for {
			fmt.Println("FOR LOOP ITER")
			select {
			case srcMsg := <-srcEvents:
				byt, err := json.Marshal(srcMsg.Events)
				if err != nil {
					chains[src].Error(err)
				}
				chains[src].Log(string(byt))
			case dstMsg := <-dstEvents:
				byt, err := json.Marshal(dstMsg.Events)
				if err != nil {
					chains[dst].Error(err)
				}
				chains[dst].Log(string(byt))
			case sig := <-sigCh:
				fmt.Println("Shutdown actived:", sig.String())
				break
			}
			break
		}

		return nil
	},
}
