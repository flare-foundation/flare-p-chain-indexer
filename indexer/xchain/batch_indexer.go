package xchain

import (
	"flare-indexer/database"
	"flare-indexer/indexer/context"
	"flare-indexer/indexer/shared"
	"flare-indexer/logger"
	"flare-indexer/utils"
	"flare-indexer/utils/chain"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/indexer"
	"github.com/ava-labs/avalanchego/snow/engine/avalanche/vertex"
	"github.com/ava-labs/avalanchego/vms/avm/txs"
	"github.com/ava-labs/avalanchego/wallet/chain/x"
	"gorm.io/gorm"
)

// Indexer for X-chain vertices (blocks). Implements ContainerBatchIndexer
type txBatchIndexer struct {
	db     *gorm.DB
	client chain.IndexerClient

	inOutIndexer *shared.InputOutputIndexer
	newTxs       []*database.XChainTx
	newVertices  []*database.XChainVtx
}

func NewXChainBatchIndexer(
	ctx context.IndexerContext,
	client chain.IndexerClient,
	txClient chain.IndexerClient,
) *txBatchIndexer {
	updater := newXChainInputUpdater(ctx, txClient)
	return &txBatchIndexer{
		db:     ctx.DB(),
		client: client,

		inOutIndexer: shared.NewInputOutputIndexer(updater),
		newTxs:       make([]*database.XChainTx, 0),
	}
}

func (xi *txBatchIndexer) Reset(containerLen int) error {
	xi.newVertices = make([]*database.XChainVtx, 0, containerLen)
	xi.newTxs = make([]*database.XChainTx, 0, 5*containerLen) // approximate
	xi.inOutIndexer.Reset(containerLen)
	return nil
}

func (xi *txBatchIndexer) AddContainer(index uint64, container indexer.Container) error {
	vtx, err := vertex.Parse(container.Bytes)
	if err != nil {
		return err
	}
	if len(vtx.ParentIDs()) > 1 {
		return fmt.Errorf("only one vertex parent is expected, got %d for id %s at height %d",
			len(vtx.ParentIDs()), vtx.ID().String(), vtx.Height())
	}
	for _, txBytes := range vtx.Txs() {
		err = xi.addTransaction(vtx.Height(), txBytes)
		if err != nil {
			return err
		}
	}

	xi.newVertices = append(xi.newVertices, &database.XChainVtx{
		VtxID:     vtx.ID().String(),
		ParentID:  vtx.ParentIDs()[0].String(),
		VtxIndex:  index,
		Height:    vtx.Height(),
		Timestamp: time.Unix(0, container.Timestamp),
	})
	return nil
}

func (xi *txBatchIndexer) addTransaction(vtxHeight uint64, txBytes []byte) error {
	tx, err := x.Parser.ParseGenesisTx(txBytes)
	if err != nil {
		return err
	}

	switch unsignedTx := tx.Unsigned.(type) {
	case *txs.BaseTx:
		err := xi.addBaseTx(tx.ID().String(), vtxHeight, unsignedTx, database.XChainBaseTx, txBytes)
		if err != nil {
			return err
		}
	case *txs.ImportTx:
		err := xi.addBaseTx(tx.ID().String(), vtxHeight, &unsignedTx.BaseTx, database.XChainImportTx, txBytes)
		if err != nil {
			return err
		}
	default:
		logger.Warn("Transaction with id '%s' is NOT indexed, type is %T", tx.ID().String(), unsignedTx)
	}
	return nil
}

func (xi *txBatchIndexer) ProcessBatch() error {
	return xi.inOutIndexer.ProcessBatch()
}

func (xi *txBatchIndexer) addBaseTx(
	txID string,
	VtxHeight uint64,
	baseTx *txs.BaseTx,
	txType database.XChainTxType,
	bytes []byte,
) error {
	tx := &database.XChainTx{}
	tx.TxID = txID
	tx.VtxHeight = VtxHeight
	tx.Type = txType
	tx.Memo = string(baseTx.Memo)
	tx.Bytes = bytes

	xi.newTxs = append(xi.newTxs, tx)
	return xi.inOutIndexer.AddNewFromBaseTx(tx.TxID, &baseTx.BaseTx, XChainInputOutputCreator)
}

// Persist all entities
func (i *txBatchIndexer) PersistEntities(db *gorm.DB) error {
	ins, err := utils.CastArray[*database.XChainTxInput](i.inOutIndexer.GetIns())
	if err != nil {
		return err
	}
	outs, err := utils.CastArray[*database.XChainTxOutput](i.inOutIndexer.GetNewOuts())
	if err != nil {
		return err
	}
	return database.CreateXChainEntities(db, i.newVertices, i.newTxs, ins, outs)
}
