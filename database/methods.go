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
