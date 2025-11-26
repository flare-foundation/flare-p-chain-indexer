package pchain

import (
	"flare-indexer/database"
	"flare-indexer/indexer/shared"
	"flare-indexer/utils"
)

var (
	PChainDefaultInputOutputCreator = inputOutputCreator{outputType: database.PChainDefaultOutput}
	PChainStakerInputOutputCreator  = inputOutputCreator{outputType: database.PChainStakeOutput}
	PChainRewardOutputCreator       = inputOutputCreator{outputType: database.PChainRewardOutput}
)

type inputOutputCreator struct {
	outputType database.PChainOutputType
}

func (ioc inputOutputCreator) CreateInputs(in shared.UpdatableInput) []*database.PChainTxInput {
	return utils.Map(in.ToDbInputs(), func(dbIn *database.TxInput) *database.PChainTxInput {
		return &database.PChainTxInput{
			TxInput: *dbIn,
		}
	})
}

func (ioc inputOutputCreator) CreateOutput(out *database.TxOutput) shared.Output {
	return &database.PChainTxOutput{
		Type:     ioc.outputType,
		TxOutput: *out,
	}
}
