package pchain

import (
	"flare-indexer/database"
	"flare-indexer/indexer/migrations"
	"time"

	"gorm.io/gorm"
)

func init() {
	migrations.Container.Add("2023-02-10-00-00", "Create initial state for P-Chain transactions", createPChainTxState)
	migrations.Container.Add("2024-11-07-00-00", "Alter type column size in p_chain_txes table", alterPChainTxType)
	migrations.Container.Add("2025-09-30-00-00", "Delete all P-chain transactions", deleteTransactions)
}

func createPChainTxState(db *gorm.DB) error {
	return database.CreateState(db, &database.State{
		Name:           StateName,
		NextDBIndex:    0,
		LastChainIndex: 0,
		Updated:        time.Now(),
	})
}

func alterPChainTxType(db *gorm.DB) error {
	return db.Exec("ALTER TABLE p_chain_txes CHANGE COLUMN type type VARCHAR(40)").Error
}

// Delete all P-chain transactions and reset the state to start indexing from the beginning
func deleteTransactions(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM p_chain_tx_inputs").Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM p_chain_tx_outputs").Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM p_chain_txes").Error; err != nil {
			return err
		}
		if err := tx.Exec("UPDATE states SET next_db_index = 0, last_chain_index = 0 WHERE name = ?", StateName).Error; err != nil {
			return err
		}
		return nil
	})
}
