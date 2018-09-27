/**
 *  @file
 *  @copyright defined in aergo/LICENSE.txt
 */

package cmd

import (
	"context"
	"fmt"
	"log"

	"github.com/mr-tron/base58/base58"

	"github.com/aergoio/aergo/cmd/aergocli/util"
	aergorpc "github.com/aergoio/aergo/types"
	"github.com/spf13/cobra"
)

func init() {
	receiptCmd := &cobra.Command{
		Use:   "receipt [flags] subcommand",
		Short: "receipt command",
	}
	rootCmd.AddCommand(receiptCmd)

	receiptCmd.AddCommand(
		&cobra.Command{
			Use:   "get [flags] tx_hash",
			Short: "get a receipt",
			Args:  cobra.MinimumNArgs(1),
			Run: func(cmd *cobra.Command, args []string) {
				txHash, err := base58.Decode(args[0])
				if err != nil {
					log.Fatal(err)
				}
				msg, err := client.GetReceipt(context.Background(), &aergorpc.SingleBytes{Value: txHash})
				if err != nil {
					log.Fatal(err)
				}
				fmt.Println(util.JSON(msg))
			},
		},
	)
}
