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
