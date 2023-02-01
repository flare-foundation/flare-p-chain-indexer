package xchain

import (
	"flare-indexer/database"
	"flare-indexer/logger"
	"flare-indexer/utils/chain"
	"fmt"

	avaIndexer "github.com/ava-labs/avalanchego/indexer"
	"github.com/ava-labs/avalanchego/vms/avm/txs"
	"github.com/ava-labs/avalanchego/wallet/chain/x"
	mapset "github.com/deckarep/golang-set/v2"
	"gorm.io/gorm"
)

// Indexer for X-chain transactions of "type" baseTx

type keyType struct {
	string
	uint32
}

type baseTxIndexer struct {
	NewTxs  []*database.XChainTx
	NewOuts []*database.XChainTxOutput
	NewIns  []*database.XChainTxInput

	newInsBase []*XChainTxInputBase
}

// Return new indexer; batch size is approximate and is used for
// the initialization of arrays
func NewBaseTxIndexer(batchSize int) baseTxIndexer {
	return baseTxIndexer{
		NewTxs:     make([]*database.XChainTx, 0, batchSize),
		NewOuts:    make([]*database.XChainTxOutput, 0, 4*batchSize),
		NewIns:     make([]*database.XChainTxInput, 0, 4*batchSize),
		newInsBase: make([]*XChainTxInputBase, 0, 4*batchSize),
	}
}

func (i *baseTxIndexer) AddTx(data *XChainTxData) {
	// New transaction goes db
	i.NewTxs = append(i.NewTxs, data.Tx)

	// New outs get saved to db
	i.NewOuts = append(i.NewOuts, data.TxOuts...)

	// New ins (not db objects)
	i.newInsBase = append(i.newInsBase, data.TxIns...)
}

// Persist new entities
func (i *baseTxIndexer) UpdateIns(db *gorm.DB, client avaIndexer.Client) error {
	// Map of outs needed for ins; key is (txId, output index)
	outsMap := make(map[keyType]*database.XChainTxOutput)

	// First find all needed transactions for inputs
	missingTxIds := mapset.NewSet[string]()
	for _, in := range i.newInsBase {
		missingTxIds.Add(in.TxID)
	}

	updateOutsMapFromOuts(outsMap, i.NewOuts, missingTxIds)

	err := updateOutsMapFromDB(db, outsMap, missingTxIds)
	if err != nil {
		return err
	}

	err = updateOutsMapFromChain(client, outsMap, missingTxIds)
	if err != nil {
		return err
	}

	if missingTxIds.Cardinality() > 0 {
		return fmt.Errorf("unable to fetch transactions %v", missingTxIds)
	}

	for _, in := range i.newInsBase {
		out, ok := outsMap[keyType{in.TxID, in.OutputIndex}]
		if !ok {
			logger.Warn("Unable to find output (%s, %d)", in.TxID, in.OutputIndex)
		} else {
			i.NewIns = append(i.NewIns, &database.XChainTxInput{
				TxID:    in.TxID,
				Address: out.Address,
			})
		}
	}

	return nil
}

// Persist all entities
func (i *baseTxIndexer) PersistEntities(db *gorm.DB) error {
	return database.CreateXChainEntities(db, i.NewTxs, i.NewIns, i.NewOuts)
}

// Update outsMap for missing transaction idxs from transactions fetched in this batch.
// Also updates missingTxIds set.
func updateOutsMapFromOuts(
	outsMap map[keyType]*database.XChainTxOutput,
	newOuts []*database.XChainTxOutput,
	missingTxIds mapset.Set[string],
) {
	for _, out := range newOuts {
		outsMap[keyType{out.TxID, out.Idx}] = out
		// if missingTxIds.Contains(out.TxID) {
		missingTxIds.Remove(out.TxID)
		// }
	}
}

// Update outsMap for missing transaction idxs. Also updates missingTxIds set.
func updateOutsMapFromDB(
	db *gorm.DB,
	outsMap map[keyType]*database.XChainTxOutput,
	missingTxIds mapset.Set[string],
) error {
	outs, err := database.FetchXChainTxOutputs(db, missingTxIds.ToSlice())
	if err != nil {
		return err
	}
	for _, out := range outs {
		outsMap[keyType{out.TxID, out.Idx}] = &out
		missingTxIds.Remove(out.TxID)
	}
	return nil
}

// Update outsMap for missing transaction idxs by fetching transactions from the chain.
// Also updates missingTxIds set.
func updateOutsMapFromChain(
	client avaIndexer.Client,
	outsMap map[keyType]*database.XChainTxOutput,
	missingTxIds mapset.Set[string],
) error {
	for _, txId := range missingTxIds.ToSlice() {
		container, err := chain.FetchContainerFromIndexer(client, txId)
		if err != nil {
			return err
		}
		if container == nil {
			missingTxIds.Remove(txId)
			continue
		}

		tx, err := x.Parser.ParseGenesisTx(container.Bytes)
		if err != nil {
			return err
		}

		var outs []*database.XChainTxOutput
		switch unsignedTx := tx.Unsigned.(type) {
		case *txs.BaseTx:
			outs, err = XChainTxOutputsFromBaseTx(txId, unsignedTx)
		case *txs.ImportTx:
			outs, err = XChainTxOutputsFromBaseTx(txId, &unsignedTx.BaseTx)
		default:
			return fmt.Errorf("transaction with id %s has unsupported type %T", container.ID.String(), unsignedTx)
		}

		if err != nil {
			return err
		}
		updateOutsMapFromOuts(outsMap, outs, missingTxIds)

	}
	return nil
}