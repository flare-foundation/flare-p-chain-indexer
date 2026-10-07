package pchain

import (
	"encoding/hex"
	"flare-indexer/database"
	"flare-indexer/indexer/context"
	"flare-indexer/indexer/shared"
	"flare-indexer/logger"
	"flare-indexer/utils"
	"flare-indexer/utils/chain"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/indexer"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/platformvm/block"
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

	durangoTime time.Time
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
	var durangoTime time.Time
	if time, ok := shared.DurangoTimes[ctx.Config().Chain.ChainAddressHRP]; ok {
		durangoTime = time
	}

	updater := newPChainInputUpdater(ctx, rpcClient)
	return &txBatchIndexer{
		db:        ctx.DB(),
		client:    client,
		rpcClient: rpcClient,

		inOutIndexer:    shared.NewInputOutputIndexer(updater),
		newTxs:          make([]*database.PChainTx, 0),
		dataTransformer: dataTransformer,

		durangoTime: durangoTime,
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
	case *block.ApricotProposalBlock:
		tx := innerBlkType.Tx
		err = xi.addTx(&container, database.PChainProposalBlock, innerBlk.Height(), 0, tx)
	case *block.ApricotCommitBlock:
		xi.addEmptyTx(&container, database.PChainCommitBlock, innerBlk.Height(), 0)
	case *block.ApricotAbortBlock:
		xi.addEmptyTx(&container, database.PChainAbortBlock, innerBlk.Height(), 0)
	case *block.ApricotStandardBlock:
		for _, tx := range innerBlkType.Txs() {
			err = xi.addTx(&container, database.PChainStandardBlock, innerBlk.Height(), 0, tx)
			if err != nil {
				break
			}
		}
	// Banff blocks were introduced in Avalanche 1.9.0
	case *block.BanffProposalBlock:
		for _, tx := range innerBlkType.Txs() {
			err = xi.addTx(&container, database.PChainProposalBlock, innerBlk.Height(), innerBlkType.Time, tx)
			if err != nil {
				break
			}
		}
	case *block.BanffCommitBlock:
		xi.addEmptyTx(&container, database.PChainCommitBlock, innerBlk.Height(), innerBlkType.Time)
	case *block.BanffAbortBlock:
		xi.addEmptyTx(&container, database.PChainAbortBlock, innerBlk.Height(), innerBlkType.Time)
	case *block.BanffStandardBlock:
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
	case *txs.RewardAutoRenewedValidatorTx:
		err = xi.updateRewardAutoRenewedValidatorTx(dbTx, unsignedTx)
	case *txs.AddValidatorTx:
		err = xi.updateAddValidatorTx(dbTx, unsignedTx)
	case *txs.AddPermissionlessValidatorTx:
		err = xi.updateAddPermissionlessValidatorTx(dbTx, unsignedTx)
	case *txs.AddDelegatorTx:
		err = xi.updateAddDelegatorTx(dbTx, unsignedTx)
	case *txs.AddPermissionlessDelegatorTx:
		err = xi.updateAddPermissionlessDelegatorTx(dbTx, unsignedTx)
	case *txs.AddAutoRenewedValidatorTx:
		err = xi.updateAddAutoRenewedValidatorTx(dbTx, unsignedTx)
	case *txs.SetAutoRenewedValidatorConfigTx:
		err = xi.updateSetAutoRenewedValidatorConfigTx(dbTx, unsignedTx)
	case *txs.ImportTx:
		err = xi.updateImportTx(dbTx, unsignedTx)
	case *txs.ExportTx:
		err = xi.updateExportTx(dbTx, unsignedTx)
	case *txs.AdvanceTimeTx:
		xi.updateAdvanceTimeTx(dbTx, unsignedTx)
	case *txs.BaseTx:
		err = xi.updateGeneralBaseTx(dbTx, database.PChainBaseTx, unsignedTx)

	// We do not save specific info for the following transaction types.
	// We may change this in the future if we need to index more data.
	case *txs.CreateChainTx:
		err = xi.updateGeneralBaseTx(dbTx, database.PChainCreateChainTx, &unsignedTx.BaseTx)
	case *txs.CreateSubnetTx:
		err = xi.updateGeneralBaseTx(dbTx, database.PChainCreateSubnetTx, &unsignedTx.BaseTx)
	case *txs.RemoveSubnetValidatorTx:
		err = xi.updateGeneralBaseTx(dbTx, database.PChainRemoveSubnetValidatorTx, &unsignedTx.BaseTx)
	case *txs.TransformSubnetTx:
		err = xi.updateGeneralBaseTx(dbTx, database.PChainTransformSubnetTx, &unsignedTx.BaseTx)
	case *txs.AddSubnetValidatorTx:
		err = xi.updateGeneralBaseTx(dbTx, database.PChainAddSubnetValidatorTx, &unsignedTx.BaseTx)
	case *txs.TransferSubnetOwnershipTx:
		err = xi.updateGeneralBaseTx(dbTx, database.PChainTransferSubnetOwnershipTx, &unsignedTx.BaseTx)
	case *txs.ConvertSubnetToL1Tx:
		err = xi.updateGeneralBaseTx(dbTx, database.PChainConvertSubnetToL1Tx, &unsignedTx.BaseTx)
	case *txs.RegisterL1ValidatorTx:
		err = xi.updateGeneralBaseTx(dbTx, database.PChainRegisterL1ValidatorTx, &unsignedTx.BaseTx)
	case *txs.DisableL1ValidatorTx:
		err = xi.updateGeneralBaseTx(dbTx, database.PChainDisableL1ValidatorTx, &unsignedTx.BaseTx)
	case *txs.SetL1ValidatorWeightTx:
		err = xi.updateGeneralBaseTx(dbTx, database.PChainSetL1ValidatorWeightTx, &unsignedTx.BaseTx)
	case *txs.IncreaseL1ValidatorBalanceTx:
		err = xi.updateGeneralBaseTx(dbTx, database.PChainIncreaseL1ValidatorBalanceTx, &unsignedTx.BaseTx)
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

// rewardUTXOsTxID is the id under which the chain stores this reward's UTXOs:
// the staker's txID for RewardValidatorTx, the reward tx's own id for
// RewardAutoRenewedValidatorTx
func (xi *txBatchIndexer) updateRewardTx(dbTx *database.PChainTx, tx RewardTx, rewardUTXOsTxID string) ([]shared.Output, error) {
	dbTx.RewardTxID = tx.StakerTxID().String()
	outs, err := getRewardOutputs(xi.rpcClient, rewardUTXOsTxID)
	if err != nil {
		return nil, err
	}
	xi.inOutIndexer.Add(outs, nil)
	xi.newTxs = append(xi.newTxs, dbTx)
	return outs, nil
}

func (xi *txBatchIndexer) updateRewardValidatorTx(dbTx *database.PChainTx, tx *txs.RewardValidatorTx) error {
	dbTx.Type = database.PChainRewardValidatorTx

	_, err := xi.updateRewardTx(dbTx, tx, tx.StakerTxID().String())
	return err
}

func (xi *txBatchIndexer) updateRewardAutoRenewedValidatorTx(dbTx *database.PChainTx, tx *txs.RewardAutoRenewedValidatorTx) error {
	timestamp := time.Unix(int64(tx.Timestamp), 0)
	dbTx.Type = database.PChainRewardAutoRenewedValidatorTx
	dbTx.Time = &timestamp
	// Period, StartTime, EndTime, NodeID and Weight of the renewed validation cycle are set once the
	// commit/abort block of the proposal block is indexed, see completeAutoRenewedRewardTxs

	outs, err := xi.updateRewardTx(dbTx, tx, *dbTx.TxID)
	if err != nil {
		return err
	}
	if len(outs) > 0 {
		// Staking rewards exist on this chain: a share of them may be restaked at renewal, which the
		// indexer does not track (the weight of the renewed cycle is copied from the add tx)
		logger.Warn("reward auto renewed validator tx %s minted %d reward UTXOs; the indexed weight of auto renewed validator %s ignores restaked rewards",
			*dbTx.TxID, len(outs), dbTx.RewardTxID)
	}
	return nil
}

func (xi *txBatchIndexer) updateAddValidatorTx(dbTx *database.PChainTx, tx *txs.AddValidatorTx) error {
	dbTx.Type = database.PChainAddValidatorTx
	return xi.updateBoundedValidatorTx(dbTx, tx, tx.Ins)
}

func (xi *txBatchIndexer) updateAddPermissionlessValidatorTx(dbTx *database.PChainTx, tx *txs.AddPermissionlessValidatorTx) error {
	dbTx.Type = database.PChainAddPermissionlessValidatorTx
	return xi.updateBoundedValidatorTx(dbTx, tx, tx.Ins)
}

func (xi *txBatchIndexer) updateAddAutoRenewedValidatorTx(dbTx *database.PChainTx, tx *txs.AddAutoRenewedValidatorTx) error {
	dbTx.Type = database.PChainAddAutoRenewedValidatorTx
	return xi.updateUnboundedValidatorTx(dbTx, tx, tx.Ins)
}

func (xi *txBatchIndexer) updateAddDelegatorTx(dbTx *database.PChainTx, tx *txs.AddDelegatorTx) error {
	dbTx.Type = database.PChainAddDelegatorTx
	return xi.updateDelegatorTx(dbTx, tx, tx.Ins)
}

func (xi *txBatchIndexer) updateAddPermissionlessDelegatorTx(dbTx *database.PChainTx, tx *txs.AddPermissionlessDelegatorTx) error {
	dbTx.Type = database.PChainAddPermissionlessDelegatorTx
	return xi.updateDelegatorTx(dbTx, tx, tx.Ins)
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
	updatableIns := xi.inOutIndexer.GetIns()
	ins := make([]*database.PChainTxInput, 0, 2*len(updatableIns))
	for _, in := range updatableIns {
		for _, dbIn := range in.ToDbInputs() {
			ins = append(ins, &database.PChainTxInput{TxInput: *dbIn})
		}
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
	err = database.CreatePChainEntities(db, txs, ins, outs)
	if err != nil {
		return err
	}
	return completeAutoRenewedRewardTxsInBatch(db, txs)
}

// Apply the outcomes of the validation cycles of auto-renewed validators whose commit/abort block is in
// this batch (see auto_renewed.go). The block before the batch is included: a proposal block may be the
// last block of the previous batch and its commit/abort block the first one of this batch.
func completeAutoRenewedRewardTxsInBatch(db *gorm.DB, txs []*database.PChainTx) error {
	if len(txs) == 0 {
		return nil
	}
	minHeight, maxHeight := txs[0].BlockHeight, txs[0].BlockHeight
	for _, tx := range txs[1:] {
		minHeight = min(minHeight, tx.BlockHeight)
		maxHeight = max(maxHeight, tx.BlockHeight)
	}
	if minHeight > 0 {
		minHeight--
	}
	return completeAutoRenewedRewardTxs(db, minHeight, maxHeight)
}

// Common code for addValidatorTx, addPermissionlessValidatorTx, addAutoRenewedValidatorTx (ValidatorTx interface)
func (xi *txBatchIndexer) updateValidatorTx(
	dbTx *database.PChainTx,
	tx ValidatorTx,
	txIns []*avax.TransferableInput,
) error {
	ownerAddress, err := shared.RewardsOwnerAddress(tx.ValidationRewardsOwner())
	if err != nil {
		return err
	}
	dbTx.RewardsOwner = ownerAddress

	ownerAddress, err = shared.RewardsOwnerAddress(tx.DelegationRewardsOwner())
	if err != nil {
		return err
	}
	dbTx.DelegationRewardsOwner = ownerAddress

	dbTx.FeePercentage = tx.Shares()
	return nil
}

// Common code for addValidatorTx and addPermissionlessValidatorTx (ValidatorTx interface)
func (xi *txBatchIndexer) updateBoundedValidatorTx(
	dbTx *database.PChainTx,
	tx BoundedValidatorTx,
	txIns []*avax.TransferableInput,
) error {
	err := xi.updateValidatorTx(dbTx, tx, txIns)
	if err != nil {
		return err
	}
	return xi.updateAddBoundedStakerTx(dbTx, tx, txIns)
}

// Code for addAutoRenewedValidatorTx: the first validation cycle starts at the block (chain) time and
// lasts Period seconds; renewed cycles are stored on the rows of the reward txs, see auto_renewed.go
func (xi *txBatchIndexer) updateUnboundedValidatorTx(
	dbTx *database.PChainTx,
	tx *txs.AddAutoRenewedValidatorTx,
	txIns []*avax.TransferableInput,
) error {
	if dbTx.BlockTime == nil {
		return fmt.Errorf("add auto renewed validator tx %s in block %d has no block time", *dbTx.TxID, dbTx.BlockHeight)
	}
	period := tx.Period
	startTime := *dbTx.BlockTime
	endTime := time.Unix(startTime.Unix()+int64(period), 0)
	dbTx.Period = &period
	dbTx.StartTime = &startTime
	dbTx.EndTime = &endTime
	err := xi.updateValidatorTx(dbTx, tx, txIns)
	if err != nil {
		return err
	}
	return xi.updateAddStakerTx(dbTx, tx, txIns)
}

// Common code for AddDelegatorTx and AddPermissionlessDelegatorTx (DelegatorTx interface)
func (xi *txBatchIndexer) updateDelegatorTx(
	dbTx *database.PChainTx,
	tx DelegatorTx,
	txIns []*avax.TransferableInput,
) error {
	ownerAddress, err := shared.RewardsOwnerAddress(tx.RewardsOwner())
	if err != nil {
		return err
	}
	dbTx.RewardsOwner = ownerAddress
	return xi.updateAddBoundedStakerTx(dbTx, tx, txIns)
}

func (xi *txBatchIndexer) updateAddStakerTx(
	dbTx *database.PChainTx,
	tx StakerTx,
	txIns []*avax.TransferableInput,
) error {
	dbTx.NodeID = tx.NodeID().String()
	dbTx.Weight = tx.Weight()
	dbTx.SubnetID = tx.SubnetID().String()

	publicKey, _, err := tx.PublicKey()
	if err != nil {
		return err
	}
	if publicKey != nil {
		pkString := hex.EncodeToString(publicKey.Compress())
		dbTx.SignerPublicKey = &pkString
	}

	outs, err := getAddStakerTxOutputs(*dbTx.TxID, tx)
	if err != nil {
		return err
	}
	ins := shared.InputsFromTxIns(*dbTx.TxID, txIns)

	xi.newTxs = append(xi.newTxs, dbTx)
	xi.inOutIndexer.Add(outs, ins)
	return nil
}

// Common code for Add[Permissionless]DelegatorTx and Add[Permissionless]ValidatorTx
// We assume that dbTx.blockTime is set to the block time before calling this function
func (xi *txBatchIndexer) updateAddBoundedStakerTx(
	dbTx *database.PChainTx,
	tx BoundedStakerTx,
	txIns []*avax.TransferableInput,
) error {
	var startTime time.Time
	if xi.isDurango(dbTx.BlockTime) {
		startTime = *dbTx.BlockTime
	} else {
		startTime = tx.StartTime()
	}
	dbTx.StartTime = &startTime
	endTime := tx.EndTime()
	dbTx.EndTime = &endTime
	return xi.updateAddStakerTx(dbTx, tx, txIns)
}

func (xi *txBatchIndexer) isDurango(blockTime *time.Time) bool {
	return blockTime != nil && !blockTime.Before(xi.durangoTime)
}

func getAddStakerTxOutputs(txID string, tx StakerTx) ([]shared.Output, error) {
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

func (xi *txBatchIndexer) updateSetAutoRenewedValidatorConfigTx(
	dbTx *database.PChainTx,
	tx *txs.SetAutoRenewedValidatorConfigTx,
) error {
	// The referenced add auto renewed validator tx is stored like the staker tx of a reward tx
	period := tx.Period
	dbTx.Period = &period
	dbTx.RewardTxID = tx.TxID.String()
	return xi.updateGeneralBaseTx(dbTx, database.PChainSetAutoRenewedValidatorConfigTx, &tx.BaseTx)
}
