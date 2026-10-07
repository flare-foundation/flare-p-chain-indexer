package database

import (
	"flare-indexer/utils"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/utils/constants"
	"golang.org/x/exp/slices"
	"gorm.io/gorm"
)

var (
	errInvalidTransactionType = fmt.Errorf("invalid transaction type")

	// Only primary-network ("mainnet") stakes may ever be voted on, mirrored, or
	// uptime-voted. Elastic subnets cannot be created on current chains and none
	// exist, but a subnet stake would otherwise satisfy the tx-type filters and be
	// mirrored as primary-network voting power -- exclude it by construction. The
	// empty string (and, in the queries, NULL) covers rows indexed before the
	// subnet_id column existed; a transaction indexed by current code always has the
	// subnet id set.
	votableSubnetIDs = []string{constants.PrimaryNetworkID.String(), ""}

	// SQL counterpart of PChainTx.StakerTxID for the p_chain_txes table: the transaction whose inputs
	// (input address, signing credential) define the staker of a row. For a renewal row
	// (REWARD_AUTO_RENEWED_VALIDATOR_TX) that is the referenced AddAutoRenewedValidatorTx, since the
	// reward tx has no inputs of its own.
	stakerTxIDSQL = stakerTxIDExpr("p_chain_txes")
)

func stakerTxIDExpr(table string) string {
	return fmt.Sprintf("CASE WHEN %[1]s.type = '%[2]s' THEN %[1]s.reward_tx_id ELSE %[1]s.tx_id END",
		table, PChainRewardAutoRenewedValidatorTx)
}

func FetchPChainTxOutputs(db *gorm.DB, ids []string) ([]PChainTxOutput, error) {
	var txs []PChainTxOutput
	err := db.Where("tx_id IN ?", ids).Find(&txs).Error
	return txs, err
}

func CreatePChainEntities(db *gorm.DB, txs []*PChainTx, ins []*PChainTxInput, outs []*PChainTxOutput) error {
	if len(txs) > 0 { // attempt to create from an empty slice returns error
		err := db.Create(txs).Error
		if err != nil {
			return err
		}
	}
	if len(ins) > 0 {
		err := db.Create(ins).Error
		if err != nil {
			return err
		}
	}
	if len(outs) > 0 {
		return db.Create(outs).Error
	}
	return nil
}

// Returns a list of transaction ids initiating a create validator transaction or a create delegation transaction
// - if address is not empty, only returns transactions where the given address is the sender of the transaction
// - if time is not zero, only returns transactions where the validatot time or delegation time contains the given time
// - if nodeID is not empty, only returns transactions where the given node ID is the validator node ID
func FetchPChainStakingTransactions(
	db *gorm.DB,
	txTypes []PChainTxType,
	nodeID string,
	address string,
	time time.Time,
	offset int,
	limit int,
) ([]string, error) {
	var validatorTxs []PChainTx

	for _, txType := range txTypes {
		if !slices.Contains(PChainStakingTransactions[:], txType) {
			return nil, errInvalidTransactionType
		}
	}
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	// Only rows that describe a stake: the reward auto renewed validator tx of a validator that exited
	// (or whose commit/abort block is not indexed yet) has no validation cycle
	query := db.Where("type IN ?", txTypes).Where("start_time IS NOT NULL")
	if len(nodeID) > 0 {
		query = query.Where("node_id = ?", nodeID)
	}
	if !time.IsZero() {
		query = query.Where("start_time <= ?", time).Where("end_time >= ?", time)
	}
	if len(address) > 0 {
		query = query.Joins("left join p_chain_tx_inputs as inputs on inputs.tx_id = "+stakerTxIDSQL).
			Where("inputs.address = ?", address)
	}
	err := query.Offset(offset).Limit(limit).Order("p_chain_txes.tx_id").
		Distinct().Select("p_chain_txes.tx_id").Find(&validatorTxs).Error
	if err != nil {
		return nil, err
	}

	return utils.Map(validatorTxs, func(t PChainTx) string { return *t.TxID }), nil
}

// Returns a list of staking data for stakers active at specific time which include input addresses.
// Request is paginated (offset, limit).
func FetchPChainStakingData(
	db *gorm.DB,
	time time.Time,
	txTypes []PChainTxType,
	offset int,
	limit int,
) ([]PChainTxData, error) {
	var validatorTxs []PChainTxData

	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	withSubquery := "WITH matched_ids AS (" +
		"SELECT id FROM p_chain_txes " +
		"WHERE start_time <= ? AND end_time >= ? AND type IN ? " +
		"ORDER BY id LIMIT ? OFFSET ?) "
	query := db.Raw(withSubquery+
		"SELECT p.*, GROUP_CONCAT(DISTINCT i.address) AS input_address "+
		"FROM p_chain_txes p "+
		"LEFT JOIN p_chain_tx_inputs i ON i.tx_id = "+stakerTxIDExpr("p")+" "+
		"WHERE p.id IN (SELECT id FROM matched_ids) "+
		"GROUP BY p.id "+
		"ORDER BY p.id",
		time, time, txTypes, limit, offset).
		Scan(&validatorTxs)
	return validatorTxs, query.Error
}

// Returns a list of transaction ids initiating transfers between chains (import/export transactions)
func FetchPChainTransferTransactions(
	db *gorm.DB,
	txType PChainTxType,
	address string,
	offset int,
	limit int,
) ([]string, error) {
	var txs []PChainTx
	if txType != PChainImportTx && txType != PChainExportTx {
		return nil, errInvalidTransactionType
	}
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	query := db.Where(&PChainTx{Type: txType})
	if len(address) > 0 {
		if txType == PChainImportTx {
			query = query.Joins("left join p_chain_tx_outputs as outputs on outputs.tx_id = p_chain_txes.tx_id").
				Where("outputs.address = ?", address)
		} else {
			query = query.Joins("left join p_chain_tx_inputs as inputs on inputs.tx_id = p_chain_txes.tx_id").
				Where("inputs.address = ?", address)
		}
	}
	err := query.Offset(offset).Limit(limit).Order("p_chain_txes.tx_id").
		Distinct().Select("p_chain_txes.tx_id").Find(&txs).Error
	if err != nil {
		return nil, err
	}

	return utils.Map(txs, func(t PChainTx) string { return *t.TxID }), nil
}

func FetchPChainTxFull(db *gorm.DB, txID string) (*PChainTx, []PChainTxInput, []PChainTxOutput, error) {
	var tx PChainTx
	err := db.Where(&PChainTx{TxID: &txID}).First(&tx).Error
	if err != nil {
		return nil, nil, nil, err
	}

	var inputs []PChainTxInput
	err = db.Where(&PChainTxInput{TxInput: TxInput{TxID: txID}}).Find(&inputs).Error
	if err != nil {
		return nil, nil, nil, err
	}

	var outputs []PChainTxOutput
	err = db.Where(&PChainTxOutput{TxOutput: TxOutput{TxID: txID}}).Order("idx").Find(&outputs).Error
	if err != nil {
		return nil, nil, nil, err
	}

	return &tx, inputs, outputs, nil
}

func FetchPChainTx(db *gorm.DB, txID string) (*PChainTx, error) {
	var tx PChainTx
	err := db.Where(&PChainTx{TxID: &txID}).First(&tx).Error
	if err != nil {
		return nil, err
	}
	return &tx, nil
}

func FetchPChainTxData(db *gorm.DB, txID string, address string) (*PChainTxData, error) {
	var tx PChainTxData
	err := db.Table("p_chain_txes").
		Joins("left join p_chain_tx_inputs as inputs on inputs.tx_id = p_chain_txes.tx_id").
		Where("p_chain_txes.tx_id = ?", txID).
		Where("inputs.address = ?", address).
		Group("p_chain_txes.id").
		// any_value is used to avoid only_full_group_by error
		Select("p_chain_txes.*, any_value(inputs.address) as input_address, min(inputs.in_idx) as input_index").
		First(&tx).Error
	if err == nil {
		return &tx, nil
	} else if err == gorm.ErrRecordNotFound {
		return nil, nil
	} else {
		return nil, err
	}
}

type PChainTxData struct {
	PChainTx
	InputAddress string
	InputIndex   uint32
}

func FetchPChainVotingData(db *gorm.DB, from time.Time, to time.Time) ([]PChainTxData, error) {
	var data []PChainTxData

	query := db.
		Table("p_chain_txes").
		Joins("left join p_chain_tx_inputs as inputs on inputs.tx_id = "+stakerTxIDSQL).
		Where("type IN ?", PChainStakingTransactions).
		Where("subnet_id IN ? OR subnet_id IS NULL", votableSubnetIDs).
		Where("start_time >= ?", from).Where("start_time < ?", to).
		Select("p_chain_txes.*, inputs.address as input_address, inputs.in_idx as input_index").
		Scan(&data)
	return data, query.Error
}

type GetPChainTxsForEpochInput struct {
	DB             *gorm.DB
	StartTimestamp time.Time
	EndTimestamp   time.Time
}

func GetPChainTxsForEpoch(in *GetPChainTxsForEpochInput) ([]PChainTxData, error) {
	var txs []PChainTxData
	err := in.DB.
		Table("p_chain_txes").
		Joins("left join p_chain_tx_inputs as inputs on inputs.tx_id = "+stakerTxIDSQL).
		Where("p_chain_txes.start_time >= ?", in.StartTimestamp).
		Where("p_chain_txes.start_time < ?", in.EndTimestamp).
		Where("p_chain_txes.type IN ?", PChainStakingTransactions).
		Where("p_chain_txes.subnet_id IN ? OR p_chain_txes.subnet_id IS NULL", votableSubnetIDs).
		Select("p_chain_txes.*, inputs.address as input_address, inputs.in_idx as input_index").
		Find(&txs).
		Error
	if err != nil {
		return nil, err
	}

	return txs, nil
}

// Fetches all primary-network P-chain staking transactions of type txType
// intersecting the given time interval
func FetchNodeStakingIntervals(db *gorm.DB, txTypes []PChainTxType, startTime time.Time, endTime time.Time) ([]PChainTx, error) {
	for _, txType := range txTypes {
		if !slices.Contains(PChainStakingTransactions[:], txType) {
			return nil, errInvalidTransactionType
		}
	}

	var txs []PChainTx
	err := db.Where("type IN ?", txTypes).
		Where("subnet_id IN ? OR subnet_id IS NULL", votableSubnetIDs).
		Where("start_time <= ?", endTime).
		Where("end_time >= ?", startTime).
		Find(&txs).Error
	return txs, err
}

// A REWARD_AUTO_RENEWED_VALIDATOR_TX row whose commit/abort block is indexed but whose outcome is not
// applied to the row yet (period IS NULL)
type PendingAutoRenewedRewardTx struct {
	PChainTx
	OptionBlockType PChainBlockType // COMMIT_BLOCK (validator renewed or exited gracefully) or ABORT_BLOCK (validator removed)
}

// Returns the pending reward auto renewed validator txs in blocks [fromHeight, toHeight] whose
// commit/abort block (the next height) is already indexed, in block order
func FetchPendingAutoRenewedRewardTxs(db *gorm.DB, fromHeight uint64, toHeight uint64) ([]PendingAutoRenewedRewardTx, error) {
	var txs []PendingAutoRenewedRewardTx
	err := db.Table("p_chain_txes").
		Joins("join p_chain_txes as options on options.block_height = p_chain_txes.block_height + 1 "+
			"and options.tx_id is null and options.block_type in ?",
			[]PChainBlockType{PChainCommitBlock, PChainAbortBlock}).
		Where("p_chain_txes.type = ?", PChainRewardAutoRenewedValidatorTx).
		Where("p_chain_txes.period IS NULL").
		Where("p_chain_txes.block_height BETWEEN ? AND ?", fromHeight, toHeight).
		Order("p_chain_txes.block_height, p_chain_txes.id").
		Select("p_chain_txes.*, options.block_type as option_block_type").
		Scan(&txs).Error
	return txs, err
}

// Returns the latest SET_AUTO_RENEWED_VALIDATOR_CONFIG_TX of the auto-renewed validator created by
// stakerTxID that was accepted before the transaction (beforeHeight, beforeID), or nil if there is none.
// Rows are inserted in processing order, so within a block a smaller id means an earlier transaction;
// a config tx packed into the same proposal block as the reward tx is executed before it and counts.
func FetchLatestAutoRenewedConfigTx(db *gorm.DB, stakerTxID string, beforeHeight uint64, beforeID uint64) (*PChainTx, error) {
	var txs []PChainTx
	err := db.Where("type = ?", PChainSetAutoRenewedValidatorConfigTx).
		Where("reward_tx_id = ?", stakerTxID).
		Where("block_height < ? OR (block_height = ? AND id < ?)", beforeHeight, beforeHeight, beforeID).
		Order("block_height DESC, id DESC").
		Limit(1).
		Find(&txs).Error
	if err != nil || len(txs) == 0 {
		return nil, err
	}
	return &txs[0], nil
}

func UpdatePChainTx(db *gorm.DB, tx *PChainTx) error {
	return db.Save(tx).Error
}
