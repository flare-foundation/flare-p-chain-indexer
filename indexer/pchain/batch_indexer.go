package pchain

import (
	"flare-indexer/database"
	"flare-indexer/indexer/context"
	"flare-indexer/indexer/shared"
	"flare-indexer/utils"
	"flare-indexer/utils/chain"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/indexer"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/platformvm/blocks"
	"github.com/ava-labs/avalanchego/vms/platformvm/fx"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"gorm.io/gorm"
)

// Transform indexer database entities before persisting them
// They are always no-op except in certain tests
type PChainDataTransformer struct {
	transformPChainTx func(*database.PChainTx) *database.PChainTx
}

func (dt *PChainDataTransformer) TransformPChainTxs(txs []*database.PChainTx) []*database.PChainTx {
	return utils.Map(txs, dt.transformPChainTx)
}

// Indexer for P-chain transactions. Implements ContainerBatchIndexer
type txBatchIndexer struct {
	db        *gorm.DB
	client    chain.IndexerClient
	rpcClient chain.RPCClient

	inOutIndexer    *shared.InputOutputIndexer
	newTxs          []*database.PChainTx
	dataTransformer *PChainDataTransformer
}

func NewPChainDataTransformer(txTransformer func(tx *database.PChainTx) *database.PChainTx) *PChainDataTransformer {
	return &PChainDataTransformer{
		transformPChainTx: txTransformer,
	}
}

func NewPChainBatchIndexer(
	ctx context.IndexerContext,
	client chain.IndexerClient,
	rpcClient chain.RPCClient,
	dataTransformer *PChainDataTransformer,
) *txBatchIndexer {
	updater := newPChainInputUpdater(ctx, rpcClient)
	return &txBatchIndexer{
		db:        ctx.DB(),
		client:    client,
		rpcClient: rpcClient,

		inOutIndexer:    shared.NewInputOutputIndexer(updater),
		newTxs:          make([]*database.PChainTx, 0),
		dataTransformer: dataTransformer,
	}
}

func (xi *txBatchIndexer) Reset(containerLen int) {
	xi.newTxs = make([]*database.PChainTx, 0, containerLen)
	xi.inOutIndexer.Reset(containerLen)
}

func (xi *txBatchIndexer) AddContainer(index uint64, container indexer.Container) error {
	innerBlk, err := chain.ParsePChainBlock(container.Bytes)
	if err != nil {
		return err
	}

	switch innerBlkType := innerBlk.(type) {
	case *blocks.ApricotProposalBlock:
		tx := innerBlkType.Tx
		err = xi.addTx(&container, database.PChainProposalBlock, innerBlk.Height(), 0, tx)
	case *blocks.ApricotCommitBlock:
		xi.addEmptyTx(&container, database.PChainCommitBlock, innerBlk.Height(), 0)
	case *blocks.ApricotAbortBlock:
		xi.addEmptyTx(&container, database.PChainAbortBlock, innerBlk.Height(), 0)
	case *blocks.ApricotStandardBlock:
		for _, tx := range innerBlkType.Txs() {
			err = xi.addTx(&container, database.PChainStandardBlock, innerBlk.Height(), 0, tx)
			if err != nil {
				break
			}
		}
	// Banff blocks were introduced in Avalanche 1.9.0
	case *blocks.BanffProposalBlock:
		tx := innerBlkType.Tx
		err = xi.addTx(&container, database.PChainProposalBlock, innerBlk.Height(), innerBlkType.Time, tx)
	case *blocks.BanffCommitBlock:
		xi.addEmptyTx(&container, database.PChainCommitBlock, innerBlk.Height(), innerBlkType.Time)
	case *blocks.BanffAbortBlock:
		xi.addEmptyTx(&container, database.PChainAbortBlock, innerBlk.Height(), innerBlkType.Time)
	case *blocks.BanffStandardBlock:
		for _, tx := range innerBlkType.Txs() {
			err = xi.addTx(&container, database.PChainStandardBlock, innerBlk.Height(), innerBlkType.Time, tx)
			if err != nil {
				break
			}
		}
	default:
		err = fmt.Errorf("block %d has unexpected type %T", index, innerBlkType)
	}
	return err
}

func (xi *txBatchIndexer) ProcessBatch() error {
	return xi.inOutIndexer.ProcessBatch()
}

func (xi *txBatchIndexer) addTx(container *indexer.Container, blockType database.PChainBlockType, height uint64, blockTime uint64, tx *txs.Tx) error {
	txID := tx.ID().String()
	dbTx := &database.PChainTx{}
	dbTx.TxID = &txID
	dbTx.BlockID = container.ID.String()
	dbTx.BlockType = blockType
	dbTx.BlockHeight = height
	dbTx.Timestamp = chain.TimestampToTime(container.Timestamp)
	dbTx.Bytes = container.Bytes
	if blockTime != 0 {
		time := time.Unix(int64(blockTime), 0)
		dbTx.BlockTime = &time
	}

	var err error = nil
	switch unsignedTx := tx.Unsigned.(type) {
	case *txs.RewardValidatorTx:
		err = xi.updateRewardValidatorTx(dbTx, unsignedTx)
	case *txs.AddValidatorTx:
		err = xi.updateAddValidatorTx(dbTx, unsignedTx)
	case *txs.AddDelegatorTx:
		err = xi.updateAddDelegatorTx(dbTx, unsignedTx)
	// case *txs.AddPermissionlessDelegatorTx:
	// 	err = xi.updateAddDelegatorTx(dbTx, &unsignedTx)
	case *txs.ImportTx:
		err = xi.updateImportTx(dbTx, unsignedTx)
	case *txs.ExportTx:
		err = xi.updateExportTx(dbTx, unsignedTx)
	case *txs.AdvanceTimeTx:
		xi.updateAdvanceTimeTx(dbTx, unsignedTx)
	case *txs.CreateChainTx:
		err = xi.updateGeneralBaseTx(dbTx, database.PChainCreateChainTx, &unsignedTx.BaseTx)
	case *txs.CreateSubnetTx:
		err = xi.updateGeneralBaseTx(dbTx, database.PChainCreateSubnetTx, &unsignedTx.BaseTx)
	case *txs.RemoveSubnetValidatorTx:
		err = xi.updateGeneralBaseTx(dbTx, database.PChainRemoveSubnetValidatorTx, &unsignedTx.BaseTx)
	case *txs.TransformSubnetTx:
		err = xi.updateGeneralBaseTx(dbTx, database.PChainTransformSubnetTx, &unsignedTx.BaseTx)
	// We leave out the following transaction types as they are rejected by Flare nodes
	// - AddSubnetValidatorTx
	// - AddPermissionlessValidatorTx
	// - AddPermissionlessDelegatorTx
	default:
		err = fmt.Errorf("p-chain transaction %v with type %T in block %d is not indexed", dbTx.TxID, unsignedTx, height)
	}
	return err
}

func (xi *txBatchIndexer) addEmptyTx(container *indexer.Container, blockType database.PChainBlockType, height uint64, blockTime uint64) {
	dbTx := &database.PChainTx{}
	dbTx.BlockID = container.ID.String()
	dbTx.BlockType = blockType
	dbTx.BlockHeight = height
	dbTx.Timestamp = chain.TimestampToTime(container.Timestamp)
	dbTx.Bytes = container.Bytes
	dbTx.TxID = nil
	if blockTime != 0 {
		time := time.Unix(int64(blockTime), 0)
		dbTx.BlockTime = &time
	}

	xi.newTxs = append(xi.newTxs, dbTx)
}

func (xi *txBatchIndexer) updateRewardValidatorTx(dbTx *database.PChainTx, tx *txs.RewardValidatorTx) error {
	dbTx.Type = database.PChainRewardValidatorTx
	dbTx.RewardTxID = tx.TxID.String()

	outs, err := getRewardOutputs(xi.rpcClient, dbTx.RewardTxID)
	if err != nil {
		return err
	}
	xi.inOutIndexer.Add(outs, nil)
	xi.newTxs = append(xi.newTxs, dbTx)
	return nil
}

func (xi *txBatchIndexer) updateAddValidatorTx(dbTx *database.PChainTx, tx *txs.AddValidatorTx) error {
	dbTx.Type = database.PChainAddValidatorTx
	dbTx.FeePercentage = tx.DelegationShares
	return xi.updateAddStakerTx(dbTx, tx, tx.Ins, tx.RewardsOwner)
}

func (xi *txBatchIndexer) updateAddDelegatorTx(dbTx *database.PChainTx, tx *txs.AddDelegatorTx) error {
	dbTx.Type = database.PChainAddDelegatorTx
	return xi.updateAddStakerTx(dbTx, tx, tx.Ins, tx.DelegationRewardsOwner)
}

func (xi *txBatchIndexer) updateImportTx(dbTx *database.PChainTx, tx *txs.ImportTx) error {
	dbTx.Type = database.PChainImportTx
	dbTx.ChainID = tx.SourceChain.String()
	xi.newTxs = append(xi.newTxs, dbTx)
	return xi.inOutIndexer.AddNewFromBaseTx(*dbTx.TxID, &tx.BaseTx.BaseTx, PChainDefaultInputOutputCreator)
}

func (xi *txBatchIndexer) updateExportTx(dbTx *database.PChainTx, tx *txs.ExportTx) error {
	dbTx.Type = database.PChainExportTx
	dbTx.ChainID = tx.DestinationChain.String()
	xi.newTxs = append(xi.newTxs, dbTx)
	return xi.inOutIndexer.AddNewFromBaseTx(*dbTx.TxID, &tx.BaseTx.BaseTx, PChainDefaultInputOutputCreator)
}

func (xi *txBatchIndexer) updateAdvanceTimeTx(dbTx *database.PChainTx, tx *txs.AdvanceTimeTx) {
	time := time.Unix(int64(tx.Time), 0)
	dbTx.Type = database.PChainAdvanceTimeTx
	dbTx.Time = &time
	xi.newTxs = append(xi.newTxs, dbTx)
}

func (xi *txBatchIndexer) updateGeneralBaseTx(dbTx *database.PChainTx, txType database.PChainTxType, baseTx *txs.BaseTx) error {
	dbTx.Type = txType
	xi.newTxs = append(xi.newTxs, dbTx)
	return xi.inOutIndexer.AddNewFromBaseTx(*dbTx.TxID, &baseTx.BaseTx, PChainDefaultInputOutputCreator)
}

// Persist all entities
func (xi *txBatchIndexer) PersistEntities(db *gorm.DB) error {
	ins, err := utils.CastArray[*database.PChainTxInput](xi.inOutIndexer.GetIns())
	if err != nil {
		return err
	}
	outs, err := utils.CastArray[*database.PChainTxOutput](xi.inOutIndexer.GetNewOuts())
	if err != nil {
		return err
	}

	var txs []*database.PChainTx
	if xi.dataTransformer != nil {
		txs = xi.dataTransformer.TransformPChainTxs(xi.newTxs)
	} else {
		txs = xi.newTxs
	}
	return database.CreatePChainEntities(db, txs, ins, outs)
}

// Common code for AddDelegatorTx and AddValidatorTx
func (xi *txBatchIndexer) updateAddStakerTx(
	dbTx *database.PChainTx,
	tx txs.PermissionlessStaker,
	txIns []*avax.TransferableInput,
	rewardsOwner fx.Owner,
) error {
	startTime := tx.StartTime()
	endTime := tx.EndTime()
	dbTx.NodeID = tx.NodeID().String()
	dbTx.StartTime = &startTime
	dbTx.EndTime = &endTime
	dbTx.Weight = tx.Weight()

	ownerAddress, err := shared.RewardsOwnerAddress(rewardsOwner)
	if err != nil {
		return err
	}
	dbTx.RewardsOwner = ownerAddress

	outs, err := getAddStakerTxOutputs(*dbTx.TxID, tx)
	if err != nil {
		return err
	}
	ins := shared.InputsFromTxIns(*dbTx.TxID, txIns, PChainDefaultInputOutputCreator)

	xi.newTxs = append(xi.newTxs, dbTx)
	xi.inOutIndexer.Add(outs, ins)
	return nil
}

func getAddStakerTxOutputs(txID string, tx txs.PermissionlessStaker) ([]shared.Output, error) {
	outs, err := shared.OutputsFromTxOuts(txID, tx.Outputs(), 0, PChainDefaultInputOutputCreator)
	if err != nil {
		return nil, err
	}
	stakeOuts, err := shared.OutputsFromTxOuts(txID, tx.Stake(), len(outs), PChainStakerInputOutputCreator)
	if err != nil {
		return nil, err
	}
	outs = append(outs, stakeOuts...)
	return outs, nil
}

func getRewardOutputs(client chain.RPCClient, txID string) ([]shared.Output, error) {
	utxos, err := CallPChainGetRewardUTXOsApi(client, txID)
	if err != nil {
		return nil, err
	}
	return shared.OutputsFromUTXO(txID, utxos, PChainRewardOutputCreator)
}
