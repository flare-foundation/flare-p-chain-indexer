package database

import (
	"time"
)

// Abstact entity, all other entities should be derived from it
type BaseEntity struct {
	ID uint64 `gorm:"primaryKey"`
}

type Migration struct {
	BaseEntity
	Version     string `gorm:"type:varchar(50);unique;not null"`
	Description string `gorm:"type:varchar(256)"`
	ExecutedAt  time.Time
	Duration    int
	Status      MigrationStatus `gorm:"type:varchar(20)"`
}

type State struct {
	BaseEntity
	Name           string `gorm:"type:varchar(50);index"`
	NextDBIndex    uint64 // Next item to index, i.e., "last index" + 1
	LastChainIndex uint64
	Updated        time.Time
}

// Abstact entity, common columns for X-chain and P-chain transaction inputs
type TxInput struct {
	BaseEntity
	InIdx   uint32 // Index of the input
	TxID    string `gorm:"type:varchar(50);not null;index"` // Transaction ID
	Amount  uint64
	Address string    `gorm:"type:varchar(60);index"`
	OutTxID string    `gorm:"type:varchar(50)"` // Transaction ID with output
	OutIdx  uint32    // Index of the output
	Type    InputType `gorm:"type:varchar(20)"` // Transaction input type (default or "input" input)
}

// Abstact entity, common columns for X-chain and P-chain transaction inputs
type TxOutput struct {
	BaseEntity
	TxID    string `gorm:"type:varchar(50);not null;index"` // Transaction ID
	Amount  uint64
	Idx     uint32
	Address string `gorm:"type:varchar(60);index"`
	Type OutputType `gorm:"type:varchar(20)"` // Transaction output type (default or "stake" output)

}
