package api

import (
	"flare-indexer/database"
	"time"
)

type ApiPChainTx struct {
	Type        database.PChainTxType `json:"type"`
	TxID        *string               `json:"txID"`
	BlockHeight uint64                `json:"blockHeight"`
	ChainID     string                `json:"chainID"`
	NodeID      string                `json:"nodeID"`
	StartTime   *time.Time            `json:"startTime"`
	EndTime     *time.Time            `json:"endTime"`
	Weight      uint64                `json:"weight"`
	BlockTime   *time.Time            `json:"blockTime"`
	Period      *uint64               `json:"period"`
	RefTxID     *string               `json:"refTxID"` // Referenced transaction: the staker tx of a reward tx, the add tx of a set auto renewed validator config tx

	Inputs  []ApiPChainTxInput  `json:"inputs"`
	Outputs []ApiPChainTxOutput `json:"outputs"`
}

type ApiPChainTxInput struct {
	Amount  uint64 `json:"amount"`
	Address string `json:"address"`
}

type ApiPChainTxOutput struct {
	Amount  uint64 `json:"amount"`
	Address string `json:"address"`
	Idx     uint32 `json:"index"`
}

func NewApiPChainTx(tx *database.PChainTx, inputs []database.PChainTxInput, outputs []database.PChainTxOutput) *ApiPChainTx {
	return &ApiPChainTx{
		Type:        tx.Type,
		TxID:        tx.TxID,
		BlockHeight: tx.BlockHeight,
		ChainID:     tx.ChainID,
		NodeID:      tx.NodeID,
		StartTime:   tx.StartTime,
		EndTime:     tx.EndTime,
		Weight:      tx.Weight,
		BlockTime:   tx.BlockTime,
		Period:      tx.Period,
		RefTxID:     refTxID(tx),
		Inputs:      newApiPChainInputs(inputs),
		Outputs:     newApiPChainOutputs(outputs),
	}
}

func newApiPChainInputs(inputs []database.PChainTxInput) []ApiPChainTxInput {
	result := make([]ApiPChainTxInput, len(inputs))
	for i, in := range inputs {
		result[i] = ApiPChainTxInput{
			Amount:  in.Amount,
			Address: in.Address,
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
		}
	}
	return result
}

type ApiPChainTxListItem struct {
	Type         database.PChainTxType `json:"type"`
	TxID         *string               `json:"txID"`
	BlockHeight  uint64                `json:"blockHeight"`
	ChainID      string                `json:"chainID"`
	NodeID       string                `json:"nodeID"`
	StartTime    *time.Time            `json:"startTime"`
	EndTime      *time.Time            `json:"endTime"`
	Weight       uint64                `json:"weight"`
	Period       *uint64               `json:"period"`
	RefTxID      *string               `json:"refTxID"`
	InputAddress string                `json:"inputAddress"`
	InputIndex   uint32                `json:"inputIndex"`
}

func newApiPChainTxListItem(tx *database.PChainTxData) ApiPChainTxListItem {
	return ApiPChainTxListItem{
		Type:         tx.Type,
		TxID:         tx.TxID,
		BlockHeight:  tx.BlockHeight,
		ChainID:      tx.ChainID,
		NodeID:       tx.NodeID,
		StartTime:    tx.StartTime,
		EndTime:      tx.EndTime,
		Weight:       tx.Weight,
		Period:       tx.Period,
		RefTxID:      refTxID(&tx.PChainTx),
		InputAddress: tx.InputAddress,
		InputIndex:   tx.InputIndex,
	}
}

func NewApiPChainTxList(txs []database.PChainTxData) []ApiPChainTxListItem {
	result := make([]ApiPChainTxListItem, len(txs))
	for i := range txs {
		result[i] = newApiPChainTxListItem(&txs[i])
	}
	return result
}

// Referenced transaction of a reward (auto renewed) validator tx (the staker tx) or of a set auto renewed
// validator config tx (the add auto renewed validator tx); nil for other transactions
func refTxID(tx *database.PChainTx) *string {
	if tx.RewardTxID == "" {
		return nil
	}
	return &tx.RewardTxID
}
