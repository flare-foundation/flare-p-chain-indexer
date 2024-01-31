package api

import (
	"flare-indexer/database"
	"time"
)

type ApiPChainTxBase struct {
	Type          database.PChainTxType `json:"type"`
	TxID          *string               `json:"txID"`
	BlockHeight   uint64                `json:"blockHeight"`
	ChainID       string                `json:"chainID"`
	NodeID        string                `json:"nodeID"`
	StartTime     *time.Time            `json:"startTime"`
	EndTime       *time.Time            `json:"endTime"`
	Weight        uint64                `json:"weight"`
	FeePercentage uint32                `json:"feePercentage"`
	ChainTime     *time.Time            `json:"chainTime"`
}

type ApiPChainTx struct {
	ApiPChainTxBase

	Inputs  []ApiPChainTxInput  `json:"inputs"`
	Outputs []ApiPChainTxOutput `json:"outputs"`
}

type ApiPChainTxInput struct {
	Amount  uint64             `json:"amount"`
	Address string             `json:"address"`
	Idx     uint32             `json:"index"`
	Type    database.InputType `json:"type"`
}

type ApiPChainTxOutput struct {
	Amount  uint64              `json:"amount"`
	Address string              `json:"address"`
	Idx     uint32              `json:"index"`
	Type    database.OutputType `json:"type"`
}

func newApiPChainTxBase(tx *database.PChainTx) ApiPChainTxBase {
	return ApiPChainTxBase{
		Type:          tx.Type,
		TxID:          tx.TxID,
		BlockHeight:   tx.BlockHeight,
		ChainID:       tx.ChainID,
		NodeID:        tx.NodeID,
		StartTime:     tx.StartTime,
		EndTime:       tx.EndTime,
		Weight:        tx.Weight,
		ChainTime:     tx.ChainTime,
		FeePercentage: tx.FeePercentage,
	}
}

func NewApiPChainTx(tx *database.PChainTx, inputs []database.PChainTxInput, outputs []database.PChainTxOutput) *ApiPChainTx {
	return &ApiPChainTx{
		newApiPChainTxBase(tx),
		newApiPChainInputs(inputs),
		newApiPChainOutputs(outputs),
	}
}

func newApiPChainInputs(inputs []database.PChainTxInput) []ApiPChainTxInput {
	result := make([]ApiPChainTxInput, len(inputs))
	for i, in := range inputs {
		result[i] = ApiPChainTxInput{
			Amount:  in.Amount,
			Address: in.Address,
			Idx:     in.InIdx,
			Type:    in.Type,
		}
	}
	return result
}

func newApiPChainInputsFromTxData(inputs []database.PChainTxInputData) []ApiPChainTxInput {
	result := make([]ApiPChainTxInput, len(inputs))
	for i, in := range inputs {
		result[i] = ApiPChainTxInput{
			Amount:  in.Amount,
			Address: in.Address,
			Idx:     in.Idx,
			Type:    in.Type,
		}
	}
	return result
}

func newApiPChainOutputs(inputs []database.PChainTxOutput) []ApiPChainTxOutput {
	result := make([]ApiPChainTxOutput, len(inputs))
	for i, out := range inputs {
		result[i] = ApiPChainTxOutput{
			Amount:  out.Amount,
			Address: out.Address,
			Idx:     out.Idx,
			Type:    out.Type,
		}
	}
	return result
}

func newApiPChainOutputsFromTxData(inputs []database.PChainTxOutputData) []ApiPChainTxOutput {
	result := make([]ApiPChainTxOutput, len(inputs))
	for i, out := range inputs {
		result[i] = ApiPChainTxOutput{
			Amount:  out.Amount,
			Address: out.Address,
			Idx:     out.Idx,
			Type:    out.Type,
		}
	}
	return result
}

type ApiPChainTxListItem struct {
	ApiPChainTxBase

	InputAddress string `json:"inputAddress"`
	InputIndex   uint32 `json:"inputIndex"`
}

func newApiPChainTxListItem(tx *database.PChainTxData) ApiPChainTxListItem {
	return ApiPChainTxListItem{
		ApiPChainTxBase: newApiPChainTxBase(&tx.PChainTx),
		InputAddress:    tx.InputAddress,
		InputIndex:      tx.InputIndex,
	}
}

func NewApiPChainTxList(txs []database.PChainTxData) []ApiPChainTxListItem {
	result := make([]ApiPChainTxListItem, len(txs))
	for i, tx := range txs {
		result[i] = newApiPChainTxListItem(&tx)
	}

	return result
}

func NewApiPChainTxInOutList(txs []database.PChainTxInOutData) []ApiPChainTx {

	result := make([]ApiPChainTx, len(txs))
	for i, tx := range txs {
		result[i] = ApiPChainTx{
			ApiPChainTxBase: newApiPChainTxBase(&tx.PChainTx),
			Inputs:          newApiPChainInputsFromTxData(tx.Inputs),
			Outputs:         newApiPChainOutputsFromTxData(tx.Outputs),
		}
	}

	return result
}
