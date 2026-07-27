package shared

import (
	"container/list"
	"flare-indexer/database"
)

type Output interface {
	Tx() string    // transaction id of this output
	Index() uint32 // output index
	Addr() string  // address
}

type UpdatableInput interface {
	OutTx() string    // output transaction id of the input
	OutIndex() uint32 // index of output transaction

	// Set the addresses of the input from the owners of the consumed output.
	// Implementations must keep only the owners that authorized the spend -- a listed
	// co-owner that did not sign must never be recorded as the spender.
	UpdateAddresses([]string)
	ToDbInputs() []*database.TxInput
}

type updatableInput struct {
	InIdx  uint32
	TxID   string
	Amount uint64
	// Owner slots of the consumed output that authorized this spend. Nil if they could
	// not be determined, which only happens for input types we cannot parse.
	SigIndices []uint32
	Addresses  []string
	OutTxID    string
	OutIdx     uint32
}

func (ui *updatableInput) OutTx() string {
	return ui.OutTxID
}

func (ui *updatableInput) OutIndex() uint32 {
	return ui.OutIdx
}

func (ui *updatableInput) UpdateAddresses(addrs []string) {
	ui.Addresses = selectSigners(addrs, ui.SigIndices, ui.OutTxID, ui.OutIdx)
}

func (ui *updatableInput) ToDbInputs() []*database.TxInput {
	dbInputs := make([]*database.TxInput, len(ui.Addresses))
	for i, addr := range ui.Addresses {
		dbInputs[i] = &database.TxInput{
			InIdx:   ui.InIdx,
			TxID:    ui.TxID,
			Amount:  ui.Amount,
			Address: addr,
			OutTxID: ui.OutTxID,
			OutIdx:  ui.OutIdx,
		}
	}
	return dbInputs
}

// Create chain specific database object from generic TxOutput (TxInput) type, e.g.,
// XChainTxOutput, PChainTxInput
type OutputCreator interface {
	CreateOutput(out *database.TxOutput) Output
}

type IdIndexKey struct {
	ID    string
	Index uint32
}

type OutputMap map[IdIndexKey][]Output

type InputList struct {
	inputs *list.List
}
