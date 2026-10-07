package database

import "time"

func (s *State) Update(nextIndex, lastIndex uint64) {
	s.NextDBIndex = nextIndex
	s.LastChainIndex = lastIndex
	s.Updated = time.Now()
}

func (s *State) UpdateTime() {
	s.Updated = time.Now()
}

func (out TxOutput) Addr() string {
	return out.Address
}

func (out TxOutput) Tx() string {
	return out.TxID
}

func (out TxOutput) Index() uint32 {
	return out.Idx
}

// StakerTxID returns the id of the transaction that created the stake described by this row: the row's
// own transaction, or, for a REWARD_AUTO_RENEWED_VALIDATOR_TX row (which describes the renewed validation
// cycle of an auto-renewed validator), the referenced AddAutoRenewedValidatorTx. The inputs, the input
// address and the signing credential of a stake belong to this transaction; a reward tx has none of its
// own. Keep in sync with stakerTxIDExpr in pchain_queries.go.
func (tx *PChainTx) StakerTxID() string {
	if tx.Type == PChainRewardAutoRenewedValidatorTx {
		return tx.RewardTxID
	}
	if tx.TxID == nil {
		return ""
	}
	return *tx.TxID
}
