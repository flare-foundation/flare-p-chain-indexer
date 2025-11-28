package xchain

import (
	"flare-indexer/database"
	"flare-indexer/indexer/shared"
	"flare-indexer/utils"
)

var XChainInputOutputCreator = inputOutputCreator{}

type inputOutputCreator struct{}

func (ioc inputOutputCreator) CreateInputs(in shared.UpdatableInput) []*database.XChainTxInput {
	return utils.Map(in.ToDbInputs(), func(dbIn *database.TxInput) *database.XChainTxInput {
		return &database.XChainTxInput{
			TxInput: *dbIn,
		}
	})
}

func (ioc inputOutputCreator) CreateOutput(out *database.TxOutput) shared.Output {
	return &database.XChainTxOutput{
		TxOutput: *out,
	}
}
