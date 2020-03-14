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
	Short: "TODO: This cmd is wip right now",
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

		events := "tm.event = 'Tx'"

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

		done := trapSignal()
		defer close(done)

		for {
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
			default:
				// NOTE: This causes the for loop to run continuously
			}

			// If there done channel msg, quit
			if len(done) > 0 {
				<-done
				fmt.Println("shutdown activated")
				break
			}
		}
		return nil
	},
}

func trapSignal() chan bool {
	sigCh := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		fmt.Println("Signal Recieved:", sig.String())
		close(sigCh)
		done <- true
	}()

	return done
}
