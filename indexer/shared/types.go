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

type Input interface {
	OutTx() string     // output transaction id of the input
	OutIndex() uint32  // index of output transaction
	Addr() string      // address
	ImportInput() bool // is import input (does not need address update)

	UpdateAddr(string)
}

// Create chain specific database object from generic TxOutput (TxInput) type, e.g.,
// XChainTxOutput, PChainTxInput
type OutputCreator interface {
	CreateOutput(out *database.TxOutput) Output
}

type InputCreator interface {
	CreateInput(out *database.TxInput) Input
}

type InputOutputCreator interface {
	OutputCreator
	InputCreator
}

type IdIndexKey struct {
	ID    string
	Index uint32
}

type OutputMap map[IdIndexKey]Output

type InputList struct {
	inputs *list.List
}
