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

func (in TxInput) Addr() string {
	return in.Address
}

func (in TxInput) OutTx() string {
	return in.OutTxID
}

func (in TxInput) OutIndex() uint32 {
	return in.OutIdx
}

func (in *TxInput) UpdateAddr(addr string) {
	in.Address = addr
}

func (in TxInput) ImportInput() bool {
	return in.Type == ImportInput
}
