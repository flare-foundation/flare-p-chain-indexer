package database

import (
	"time"
)

// Table with indexed data for a P-chain transaction
type PChainTx struct {
	BaseEntity
	Type                   PChainTxType    `gorm:"type:varchar(40);index"`    // Transaction type
	TxID                   *string         `gorm:"type:varchar(50);unique"`   // Transaction ID
	BlockID                string          `gorm:"type:varchar(50);not null"` // Block ID
	BlockType              PChainBlockType `gorm:"type:varchar(20)"`          // Block type (proposal, accepted, rejected, etc.)
	RewardTxID             string          `gorm:"type:varchar(50);index"`    // Referenced transaction id: the staker tx of a reward (auto renewed) validator tx, the add auto renewed validator tx of a set auto renewed validator config tx
	BlockHeight            uint64          `gorm:"index"`                     // Block height
	Timestamp              time.Time       // Time when indexed
	ChainID                string          `gorm:"type:varchar(50)"` // Filled in case of export or import transaction
	NodeID                 string          `gorm:"type:varchar(50)"` // Filled in case of add delegator or validator transaction
	StartTime              *time.Time      `gorm:"index"`            // Start time of validator or delegator (when NodeID is not null); for a reward auto renewed validator tx the start of the renewed validation cycle
	EndTime                *time.Time      `gorm:"index"`            // End time of validator or delegator (when NodeID is not null); for a reward auto renewed validator tx the end of the renewed validation cycle
	Time                   *time.Time      // Chain time (in case of advance time transaction)
	Weight                 uint64          // Weight (stake amount) (when NodeID is not null)
	RewardsOwner           string          `gorm:"type:varchar(60)"`  // Rewards owner address (in case of add delegator or validator transaction)
	DelegationRewardsOwner string          `gorm:"type:varchar(60)"`  // Delegation rewards owner address (in case of add validator transaction)
	SubnetID               string          `gorm:"type:varchar(50)"`  // Subnet ID (from Cortina update on, will be empty for pre-Cortina)
	SignerPublicKey        *string         `gorm:"type:varchar(256)"` // Signer public key (for PermissionlessStaker transactions)
	Memo                   string          `gorm:"type:varchar(256)"`
	Bytes                  []byte          `gorm:"type:mediumblob"`
	FeePercentage          uint32          // Fee percentage (in case of add validator transaction, also on renewed cycles of auto renewed validators)
	BlockTime              *time.Time      `gorm:"index"` // Block time, non-null from Banff block activation on (Avalanche 1.9.0)
	Period                 *uint64         // Validation cycle length in seconds (add auto renewed validator tx: first cycle; set config tx: next cycle; reward auto renewed validator tx: the cycle it started, 0 = validator exited, NULL = commit/abort block not indexed yet)
}

type PChainTxInput struct {
	TxInput
}

type PChainTxOutput struct {
	TxOutput
	Type PChainOutputType `gorm:"type:varchar(20)"` // Transaction output type (default or "stake" output)
}
