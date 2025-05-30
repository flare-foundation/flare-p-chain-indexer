package shared

import (
	"flare-indexer/database"
	"flare-indexer/utils/chain"
	"fmt"

	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/components/verify"
	"github.com/ava-labs/avalanchego/vms/platformvm/fx"
	"github.com/ava-labs/avalanchego/vms/platformvm/stakeable"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
)

// Create database outputs from TransferableOutputs, provided their type is *secp256k1fx.TransferOutput and the
// number of addresses for each output is 1. Error is returned if these two conditions
// are not met.
func OutputsFromTxOuts(txID string, outs []*avax.TransferableOutput, startIndex int, creator OutputCreator) ([]Output, error) {
	txOuts := make([]Output, len(outs))
	for outi, cout := range outs {
		dbOut := &database.TxOutput{
			TxID: txID,
			Idx:  uint32(outi + startIndex),
		}
		err := UpdateTransferableOutput(dbOut, cout.Out)
		if err != nil {
			return nil, err
		}
		txOuts[outi] = creator.CreateOutput(dbOut)
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

	switch out := out.(type) {
	case *secp256k1fx.TransferOutput:
		// if len(out.Addrs) != 1 {
		// 	return fmt.Errorf("TransferableOutput has 0 or more than one address")
		// }
		if len(out.Addrs) == 0 {
			return fmt.Errorf("TransferableOutput has no addresses")
		}
		addr, err := chain.FormatAddressBytes(out.Addrs[0].Bytes())
		if err != nil {
			return err
		}
		dbOut.Amount = out.Amount()
		dbOut.Address = addr
	case *stakeable.LockOut:
		addresses := out.Addresses()
		if len(addresses) != 1 {
			return fmt.Errorf("LockOut has 0 or more than one address")
		}
		addr, err := chain.FormatAddressBytes(addresses[0])
		if err != nil {
			return fmt.Errorf("failed to format address: %w", err)
		}
		dbOut.Amount = out.Amount()
		dbOut.Address = addr
	default:
		return fmt.Errorf("TransferableOutput has unsupported type %T", out)
	}
	return nil
}

// Return address from Owner interface provided its type is *secp256k1fx.OutputOwners and the
// number of addresses is 1. Error is returned if these two conditions are not met.
func OwnerAddresses(owner fx.Owner) ([]string, error) {
	oo, ok := owner.(*secp256k1fx.OutputOwners)
	if !ok {
		return nil, fmt.Errorf("rewards owner has unsupported type")
	}

	addresses := make([]string, len(oo.Addrs))
	for _, addr := range oo.Addrs {
		formattedAddr, err := chain.FormatAddressBytes(addr.Bytes())
		if err != nil {
			return nil, fmt.Errorf("failed to format address: %w", err)
		}
		addresses = append(addresses, formattedAddr)
	}
	return addresses, nil
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
