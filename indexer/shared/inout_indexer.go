package shared

import (
	"flare-indexer/database"
	"fmt"

	"github.com/ava-labs/avalanchego/vms/components/avax"
)

// Indexer for inputs and outputs of transactions for batch processing
type InputOutputIndexer struct {
	inUpdater InputUpdater

	// Outputs of new transactions in a batch or additional outputs (reward transactions),
	// outputs should be chain-specific database objects
	outs []Output

	// Inputs of new transactions, should be chain-specific database objects
	ins []Input
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
	iox.ins = make([]Input, 0, 2*containersLen)
	iox.inUpdater.PurgeCache()
}

func (iox *InputOutputIndexer) AddNewFromBaseTx(
	txID string,
	tx *avax.BaseTx,
	creator InputOutputCreator,
) error {
	outs, err := OutputsFromTxOuts(txID, tx.Outs, 0, database.DefaultOutput, creator)
	if err != nil {
		return err
	}
	iox.outs = append(iox.outs, outs...)
	iox.ins = append(iox.ins, InputsFromTxIns(txID, tx.Ins, database.DefaultInput, creator)...)
	return nil
}

func (iox *InputOutputIndexer) Add(outs []Output, ins []Input) {
	iox.outs = append(iox.outs, outs...)
	iox.ins = append(iox.ins, ins...)
}

func (iox *InputOutputIndexer) AddImportInputs(
	txID string,
	importedInputs []*avax.TransferableInput,
	creator InputOutputCreator,
) {
	iox.ins = append(iox.ins, InputsFromTxIns(txID, importedInputs, database.ImportInput, creator)...)
}

func (iox *InputOutputIndexer) AddExportOutputs(
	txID string,
	exportedOutputs []*avax.TransferableOutput,
	offset int,
	creator InputOutputCreator,
) error {
	outs, err := OutputsFromTxOuts(txID, exportedOutputs, offset, database.ExportOutput, creator)
	if err != nil {
		return err
	}
	iox.outs = append(iox.outs, outs...)
	return nil
}

func (iox *InputOutputIndexer) UpdateInputs(inputs []Input) error {
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

func (iox *InputOutputIndexer) GetIns() []Input {
	return iox.ins
}

func (iox *InputOutputIndexer) GetNewOuts() []Output {
	return iox.outs
}
