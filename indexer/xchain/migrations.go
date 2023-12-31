package xchain

import (
	"flare-indexer/database"
	"flare-indexer/indexer/migrations"
	"time"

	"gorm.io/gorm"
)

func init() {
	migrations.Container.Add("2023-01-27-00-00", "Create initial state for X-Chain transactions", createXChainTxState)
}

func createXChainTxState(db *gorm.DB) error {
	return database.CreateState(db, &database.State{
		Name:           StateName,
		NextDBIndex:    0,
		LastChainIndex: 0,
		Updated:        time.Now(),
	})
}
