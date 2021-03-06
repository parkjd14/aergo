/**
 *  @file
 *  @copyright defined in aergo/LICENSE.txt
 */
package system

import (
	"bytes"
	"encoding/gob"
	"errors"
	"math/big"

	"github.com/aergoio/aergo/state"
	"github.com/aergoio/aergo/types"
)

var stakingkey = []byte("staking")

const StakingDelay = 10

func staking(txBody *types.TxBody, senderState *types.State,
	scs *state.ContractState, blockNo types.BlockNo) error {

	err := validateForStaking(txBody, scs, blockNo)
	if err != nil {
		return err
	}

	staked, err := getStaking(scs, txBody.Account)
	if err != nil {
		return err
	}
	beforeStaked := staked.GetAmountBigInt()
	amount := txBody.GetAmountBigInt()
	staked.Amount = new(big.Int).Add(beforeStaked, amount).Bytes()
	staked.When = blockNo
	err = setStaking(scs, txBody.Account, staked)
	if err != nil {
		return err
	}

	senderState.Balance = new(big.Int).Sub(senderState.GetBalanceBigInt(), amount).Bytes()
	return nil
}

func unstaking(txBody *types.TxBody, senderState *types.State, scs *state.ContractState, blockNo types.BlockNo) error {
	staked, err := validateForUnstaking(txBody, scs, blockNo)
	if err != nil {
		return err
	}
	amount := txBody.GetAmountBigInt()
	var backToBalance *big.Int
	if staked.GetAmountBigInt().Cmp(amount) < 0 {
		amount = new(big.Int).SetUint64(0)
		backToBalance = staked.GetAmountBigInt()
	} else {
		amount = new(big.Int).Sub(staked.GetAmountBigInt(), txBody.GetAmountBigInt())
		backToBalance = txBody.GetAmountBigInt()
	}
	staked.Amount = amount.Bytes()
	//blockNo will be updated in voting
	staked.When = 0 /*blockNo*/

	err = setStaking(scs, txBody.Account, staked)
	if err != nil {
		return err
	}
	err = voting(txBody, scs, blockNo)
	if err != nil {
		return err
	}

	senderState.Balance = new(big.Int).Add(senderState.GetBalanceBigInt(), backToBalance).Bytes()
	return nil
}

func setStaking(scs *state.ContractState, who []byte, staking *types.Staking) error {
	key := append(stakingkey, who...)
	var data bytes.Buffer
	enc := gob.NewEncoder(&data)
	err := enc.Encode(staking)
	if err != nil {
		return err
	}
	return scs.SetData(key, data.Bytes())
}

func getStaking(scs *state.ContractState, who []byte) (*types.Staking, error) {
	key := append(stakingkey, who...)
	data, err := scs.GetData(key)
	if err != nil {
		return nil, err
	}
	var staking types.Staking
	if len(data) != 0 {
		dec := gob.NewDecoder(bytes.NewBuffer(data))
		err = dec.Decode(&staking)
		if err != nil {
			return nil, err
		}
	}
	return &staking, nil
}

func GetStaking(scs *state.ContractState, address []byte) (*types.Staking, error) {
	if address != nil {
		return getStaking(scs, address)
	}
	return nil, errors.New("invalid argument : address should not nil")
}
