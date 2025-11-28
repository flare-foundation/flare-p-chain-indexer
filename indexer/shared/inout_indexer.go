package shared

import (
	"fmt"

	"github.com/ava-labs/avalanchego/vms/components/avax"
)

// Indexer for inputs and outputs of transactions for batch processing
type InputOutputIndexer struct {
	inUpdater InputUpdater

	// Outputs of new transactions in a batch or additional outputs (reward transactions),
	// outputs should be chain-specific database objects
	outs []Output

	// Inputs of new transactions, not database objects!
	ins []UpdatableInput
}

// Return new input output indexer
func NewInputOutputIndexer(inUpdater InputUpdater) *InputOutputIndexer {
	indexer := InputOutputIndexer{
		inUpdater: inUpdater,
	}
	indexer.Reset(0)
	return &indexer
}

// Should be called before new batch is started
func (iox *InputOutputIndexer) Reset(containersLen int) {
	iox.outs = make([]Output, 0, 2*containersLen)
	iox.ins = make([]UpdatableInput, 0, 2*containersLen)
	iox.inUpdater.PurgeCache()
}

func (iox *InputOutputIndexer) AddNewFromBaseTx(
	txID string,
	tx *avax.BaseTx,
	creator OutputCreator,
) error {
	outs, err := OutputsFromTxOuts(txID, tx.Outs, 0, creator)
	if err != nil {
		return err
	}
	iox.outs = append(iox.outs, outs...)
	iox.ins = append(iox.ins, InputsFromTxIns(txID, tx.Ins)...)
	return nil
}

func (iox *InputOutputIndexer) Add(outs []Output, ins []UpdatableInput) {
	iox.outs = append(iox.outs, outs...)
	iox.ins = append(iox.ins, ins...)
}

func (iox *InputOutputIndexer) UpdateInputs(inputs []UpdatableInput) error {
	list := NewInputList(inputs)
	notUpdated, err := iox.inUpdater.UpdateInputs(list)
	if err != nil {
		return err
	}
	if notUpdated.Cardinality() > 0 {
		return fmt.Errorf("unable to fetch transactions with ids %v", notUpdated)
	}
	return nil
}

func (iox *InputOutputIndexer) ProcessBatch() error {
	iox.inUpdater.CacheOutputs(iox.outs)
	return iox.UpdateInputs(iox.ins)
}

func (iox *InputOutputIndexer) GetIns() []UpdatableInput {
	return iox.ins
}

func (iox *InputOutputIndexer) GetNewOuts() []Output {
	return iox.outs
}
