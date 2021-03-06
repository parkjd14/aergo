/**
 *  @file
 *  @copyright defined in aergo/LICENSE.txt
 */

package cmd

import (
	"context"
	"errors"
	"math/big"

	"github.com/aergoio/aergo/types"
	"github.com/mr-tron/base58/base58"
	"github.com/spf13/cobra"
)

var stakingCmd = &cobra.Command{
	Use:   "staking",
	Short: "Staking balance to aergo system",
	RunE:  execStaking,
}

func execStaking(cmd *cobra.Command, args []string) error {
	return sendStaking(cmd, true)
}

var unstakingCmd = &cobra.Command{
	Use:   "unstaking",
	Short: "Unstaking balance from aergo system",
	RunE:  execUnstaking,
}

func execUnstaking(cmd *cobra.Command, args []string) error {
	return sendStaking(cmd, false)
}

func sendStaking(cmd *cobra.Command, s bool) error {
	account, err := types.DecodeAddress(address)
	if err != nil {
		return errors.New("failed to parse --address flag (" + address + ")\n" + err.Error())
	}
	payload := make([]byte, 1)
	if s {
		payload[0] = 's'
	} else {
		payload[0] = 'u'
	}
	amountBigInt, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return errors.New("failed to parse --amount flag\n" + err.Error())
	}
	if amountBigInt.Cmp(types.StakingMinimum) < 0 {
		return errors.New("Failed: minimum staking value is " + types.StakingMinimum.String())
	}

	tx := &types.Tx{
		Body: &types.TxBody{
			Account:   account,
			Recipient: []byte(types.AergoSystem),
			Amount:    amountBigInt.Bytes(),
			Payload:   payload,
			Limit:     0,
			Type:      types.TxType_GOVERNANCE,
		},
	}
	msg, err := client.SendTX(context.Background(), tx)
	if err != nil {
		return err
	}
	cmd.Println(base58.Encode(msg.Hash), msg.Error)
	return nil
}
