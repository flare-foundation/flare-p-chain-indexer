package database

import (
	"time"
)

// Table with indexed data for a P-chain transaction
type PChainTx struct {
	BaseEntity
	Type          PChainTxType    `gorm:"type:varchar(20);index"`    // Transaction type
	TxID          *string         `gorm:"type:varchar(50);unique"`   // Transaction ID
	BlockID       string          `gorm:"type:varchar(50);not null"` // Block ID
	BlockType     PChainBlockType `gorm:"type:varchar(20)"`          // Block type (proposal, accepted, rejected, etc.)
	RewardTxID    string          `gorm:"type:varchar(50)"`          // Referred transaction id in case of reward validator tx
	BlockHeight   uint64          `gorm:"index;unique"`              // Block height
	Timestamp     time.Time       // Time when indexed
	ChainID       string          `gorm:"type:varchar(50)"` // Filled in case of export or import transaction
	NodeID        string          `gorm:"type:varchar(50)"` // Filled in case of add delegator or validator transaction
	StartTime     *time.Time      `gorm:"index"`            // Start time of validator or delegator (when NodeID is not null)
	EndTime       *time.Time      `gorm:"index"`            // End time of validator or delegator (when NodeID is not null)
	Time          *time.Time      // Chain time (in case of advance time transaction)
	Weight        uint64          // Weight (stake amount) (when NodeID is not null)
	RewardsOwner  string          `gorm:"type:varchar(60)"` // Rewards owner address (in case of add delegator or validator transaction)
	Memo          string          `gorm:"type:varchar(256)"`
	Bytes         []byte          `gorm:"type:mediumblob"`
	FeePercentage uint32          // Fee percentage (in case of add validator transaction)
}

type PChainTxInput struct {
	TxInput
}

type PChainTxOutput struct {
	TxOutput
	Type PChainOutputType `gorm:"type:varchar(20)"` // Transaction output type (default or "stake" output)
}
