package pchain

import (
	"flare-indexer/database"
	"flare-indexer/indexer/shared"
)

var PChainInputOutputCreator = inputOutputCreator{}

type inputOutputCreator struct{}

func (ioc inputOutputCreator) CreateInput(in *database.TxInput) shared.Input {
	return &database.PChainTxInput{
		TxInput: *in,
	}
}

func (ioc inputOutputCreator) CreateOutput(out *database.TxOutput) shared.Output {
	return &database.PChainTxOutput{
		TxOutput: *out,
	}
}
