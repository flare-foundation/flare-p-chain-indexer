package database

import (
	"flare-indexer/utils"
	"fmt"
	"time"

	"gorm.io/gorm"
)

var (
	errInvalidTransactionType = fmt.Errorf("invalid transaction type")
)

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
	txType PChainTxType,
	nodeID string,
	address string,
	time time.Time,
	offset int,
	limit int,
) ([]string, error) {
	var validatorTxs []PChainTx

	if txType != PChainAddValidatorTx && txType != PChainAddDelegatorTx {
		return nil, errInvalidTransactionType
	}
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	query := db.Where(&PChainTx{Type: txType})
	if len(nodeID) > 0 {
		query = query.Where("node_id = ?", nodeID)
	}
	if !time.IsZero() {
		query = query.Where("start_time <= ?", time).Where("end_time >= ?", time)
	}
	if len(address) > 0 {
		query = query.Joins("left join p_chain_tx_inputs as inputs on inputs.tx_id = p_chain_txes.tx_id").
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
	txType PChainTxType,
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

	query := db.
		Table("p_chain_txes").
		Joins("left join p_chain_tx_inputs as inputs on inputs.tx_id = p_chain_txes.tx_id").
		Where("start_time <= ?", time).Where("? <= end_time", time).
		Where("type = ?", txType).
		Group("p_chain_txes.id").
		Order("p_chain_txes.id").Offset(offset).Limit(limit).
		Select("p_chain_txes.*, group_concat(distinct(inputs.address)) as input_address").
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

// Find P-chain transaction in given block height
// Returns transaction and true if found, nil and true if block was found,
// nil and false if block height does not exist.
func FindPChainTxInBlockHeight(db *gorm.DB,
	txID string,
	height uint32,
) (*PChainTxData, bool, error) {
	var txs []PChainTxData
	// err := db.Where(&PChainTx{BlockHeight: height}).Find(&txs).Error
	err := db.Table("p_chain_txes").
		Joins("left join p_chain_tx_inputs as inputs on inputs.tx_id = p_chain_txes.tx_id").
		Where("p_chain_txes.block_height = ?", height).
		Select("p_chain_txes.*, inputs.address as input_address, inputs.in_idx as input_index").
		Scan(&txs).Error
	if err != nil {
		return nil, false, err
	}
	if len(txs) == 0 {
		return nil, false, nil
	}
	tx := &txs[0]
	if *tx.TxID != txID {
		return nil, true, nil
	}
	return &txs[0], true, nil
}

func FetchPChainVotingData(db *gorm.DB, from time.Time, to time.Time) ([]PChainTxData, error) {
	var data []PChainTxData

	query := db.
		Table("p_chain_txes").
		Joins("left join p_chain_tx_inputs as inputs on inputs.tx_id = p_chain_txes.tx_id").
		Where("type = ? OR type = ?", PChainAddValidatorTx, PChainAddDelegatorTx).
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
		Joins("left join p_chain_tx_inputs as inputs on inputs.tx_id = p_chain_txes.tx_id").
		Where("p_chain_txes.start_time >= ?", in.StartTimestamp).
		Where("p_chain_txes.start_time < ?", in.EndTimestamp).
		Where(
			in.DB.Where("p_chain_txes.type = ?", PChainAddDelegatorTx).
				Or("p_chain_txes.type = ?", PChainAddValidatorTx),
		).
		Select("p_chain_txes.*, inputs.address as input_address, inputs.in_idx as input_index").
		Find(&txs).
		Error
	if err != nil {
		return nil, err
	}

	return txs, nil
}

// Fetches all P-chain staking transactions of type txType intersecting the given time interval
func FetchNodeStakingIntervals(db *gorm.DB, txType PChainTxType, startTime time.Time, endTime time.Time) ([]PChainTx, error) {
	if txType != PChainAddValidatorTx && txType != PChainAddDelegatorTx {
		return nil, errInvalidTransactionType
	}

	var txs []PChainTx
	err := db.Where(&PChainTx{Type: txType}).
		Where("start_time <= ?", endTime).
		Where("end_time >= ?", startTime).
		Find(&txs).Error
	return txs, err
}
