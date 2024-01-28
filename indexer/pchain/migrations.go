package pchain

import (
	"flare-indexer/database"
	"flare-indexer/indexer/migrations"
	"time"

	"gorm.io/gorm"
)

func init() {
	migrations.Container.Add("2023-02-10-00-00", "Create initial state for P-Chain transactions", createPChainTxState)
	migrations.Container.Add("2024-01-24-00-00", "Update transaction input type", updateTxInputType)
}

func createPChainTxState(db *gorm.DB) error {
	return database.CreateState(db, &database.State{
		Name:           StateName,
		NextDBIndex:    0,
		LastChainIndex: 0,
		Updated:        time.Now(),
	})
}

func updateTxInputType(db *gorm.DB) error {
	err := db.Model(&database.XChainTxInput{}).Where("type IS NULL").Update("type", database.DefaultInput).Error
	if err != nil {
		return err
	}
	return db.Model(&database.PChainTxInput{}).Where("type IS NULL").Update("type", database.DefaultInput).Error
}
