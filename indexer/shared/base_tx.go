package shared

import (
	"flare-indexer/database"
	"flare-indexer/utils/chain"
	"fmt"

	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/components/verify"
	"github.com/ava-labs/avalanchego/vms/platformvm/fx"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
)

// Create database outputs from TransferableOutputs, provided their type is *secp256k1fx.TransferOutput
func OutputsFromTxOuts(
	txID string,
	outs []*avax.TransferableOutput,
	startIndex int,
	creator OutputCreator,
) ([]Output, error) {
	txOuts := make([]Output, 0, len(outs))
	for outi, cout := range outs {
		dbOuts, err := CreateTransferableOutputs(txID, uint32(outi+startIndex), cout.Out)
		if err != nil {
			return nil, err
		}
		for _, dbOut := range dbOuts {
			txOuts = append(txOuts, creator.CreateOutput(dbOut))
		}
	}
	return txOuts, nil
}

// Create database outputs from UTXOs, provided their type is *secp256k1fx.TransferOutput and the
// number of addresses for each output is 1. Error is returned if these two conditions
// are not met.
func OutputsFromUTXO(txID string, utxos []*avax.UTXO, creator OutputCreator) ([]Output, error) {
	txOuts := make([]Output, len(utxos))
	for i, utxo := range utxos {
		dbOut := &database.TxOutput{
			TxID: txID,
			Idx:  utxo.OutputIndex,
		}
		err := UpdateTransferableOutput(dbOut, utxo.Out)
		if err != nil {
			return nil, err
		}
		txOuts[i] = creator.CreateOutput(dbOut)
	}
	return txOuts, nil
}

// Update database output from out provided its type is *secp256k1fx.TransferOutput and the
// number of addresses is 1. Error is returned if these two conditions are not met.
func UpdateTransferableOutput(dbOut *database.TxOutput, out verify.State) error {
	to, ok := out.(*secp256k1fx.TransferOutput)
	if !ok {
		return fmt.Errorf("TransferableOutput has unsupported type")
	}
	if len(to.Addrs) != 1 {
		return fmt.Errorf("TransferableOutput has 0 or more than one address")
	}

	addr, err := chain.FormatAddressBytes(to.Addrs[0].Bytes())
	if err != nil {
		return err
	}
	dbOut.Amount = to.Amount()
	dbOut.Address = addr
	return nil
}

// Create database outputs from out provided its type is *secp256k1fx.TransferOutput.
// Note that multiple database outputs are created if there are multiple addresses in out.
func CreateTransferableOutputs(
	txID string,
	idx uint32,
	out verify.State,
) ([]*database.TxOutput, error) {
	to, ok := out.(*secp256k1fx.TransferOutput)
	if !ok {
		return nil, fmt.Errorf("TransferableOutput has unsupported type")
	}
	dbOuts := make([]*database.TxOutput, len(to.Addrs))
	for i, addr := range to.Addrs {
		formattedAddr, err := chain.FormatAddressBytes(addr.Bytes())
		if err != nil {
			return nil, err
		}
		dbOut := &database.TxOutput{
			TxID:    txID,
			Idx:     idx,
			Amount:  to.Amount(),
			Address: formattedAddr,
		}
		dbOuts[i] = dbOut
	}
	return dbOuts, nil
}

// Return address from Owner interface provided its type is *secp256k1fx.OutputOwners and the
// number of addresses is 1. Error is returned if these two conditions are not met.
func RewardsOwnerAddress(owner fx.Owner) (string, error) {
	oo, ok := owner.(*secp256k1fx.OutputOwners)
	if !ok {
		return "", fmt.Errorf("rewards owner has unsupported type")
	}
	if len(oo.Addrs) != 1 {
		return "", fmt.Errorf("rewards owner has 0 or more than one address")
	}
	return chain.FormatAddressBytes(oo.Addrs[0].Bytes())
}

// Create inputs to BaseTx. Note that addresses of inputs are are not set. They should be updated from
// cached outputs, outputs from the database or outputs from chain
func InputsFromTxIns(txID string, ins []*avax.TransferableInput, creator InputCreator) []Input {
	txIns := make([]Input, len(ins))
	for ini, in := range ins {
		txIns[ini] = creator.CreateInput(&database.TxInput{
			InIdx:   uint32(ini),
			TxID:    txID,
			Amount:  in.In.Amount(),
			OutTxID: in.TxID.String(),
			OutIdx:  in.OutputIndex,
		})
	}
	return txIns
}
