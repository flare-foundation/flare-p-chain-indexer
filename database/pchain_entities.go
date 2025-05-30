package database

import (
	"time"
)

// Table with indexed data for a P-chain transaction
type PChainTx struct {
	BaseEntity
	Type                   PChainTxType    `gorm:"type:varchar(40);index"`    // Transaction type
	TxID                   *string         `gorm:"type:varchar(50);index"`    // Transaction ID
	BlockID                string          `gorm:"type:varchar(50);not null"` // Block ID
	BlockType              PChainBlockType `gorm:"type:varchar(20)"`          // Block type (proposal, accepted, rejected, etc.)
	RewardTxID             string          `gorm:"type:varchar(50)"`          // Referred transaction id in case of reward validator tx
	BlockHeight            uint64          `gorm:"index"`                     // Block height
	Timestamp              time.Time       // Time when indexed
	ChainID                string          `gorm:"type:varchar(50)"` // Filled in case of export or import transaction
	NodeID                 string          `gorm:"type:varchar(50)"` // Filled in case of add delegator or validator transaction
	StartTime              *time.Time      `gorm:"index"`            // Start time of validator or delegator (when NodeID is not null)
	EndTime                *time.Time      `gorm:"index"`            // End time of validator or delegator (when NodeID is not null)
	Time                   *time.Time      // Chain time (in case of advance time transaction)
	Weight                 uint64          // Weight (stake amount) (when NodeID is not null)
	RewardsOwner           string          `gorm:"type:varchar(60)"`  // Rewards owner address (in case of add delegator or validator transaction)
	DelegationRewardsOwner string          `gorm:"type:varchar(60)"`  // Delegation rewards owner address (in case of add validator transaction)
	SubnetID               string          `gorm:"type:varchar(50)"`  // Subnet ID (from Cortina update on, will be empty for pre-Cortina)
	SignerPublicKey        *string         `gorm:"type:varchar(256)"` // Signer public key (for PermissionlessStaker transactions)
	Memo                   string          `gorm:"type:varchar(256)"`
	Bytes                  []byte          `gorm:"type:mediumblob"`
	FeePercentage          uint32          // Fee percentage (in case of add validator transaction)
	BlockTime              *time.Time      `gorm:"index"` // Block time, non-null from Banff block activation on (Avalanche 1.9.0)
}

// Additional data for P-chain transactions, filled by specific transaction types not occurring frequently
type PChainTxDetails struct {
	BaseEntity
	TxID                     string `gorm:"type:varchar(50);unique;not null"` // Transaction ID
	ChainName                string `gorm:"type:varchar(128)"`                // Chain name (for CreateChainTx)
	VMID                     string `gorm:"type:varchar(50)"`                 // VM ID (for CreateChainTx)
	FxIDs                    string `gorm:"type:varchar(256)"`                // Feature extension IDs (for CreateChainTx)
	Genesis                  []byte `gorm:"type:mediumblob"`                  // Genesis data (for CreateChainTx)
	AssetID                  string `gorm:"type:varchar(50)"`                 // Asset ID (for TransformSubnetTx)
	InitialSupply            uint64 // Initial supply (for TransformSubnetTx)
	MaximumSupply            uint64 // Maximum supply (for TransformSubnetTx)
	MinConsumptionRate       uint64 // Min consumption rate (for TransformSubnetTx)
	MaxConsumptionRate       uint64 // Max consumption rate (for TransformSubnetTx)
	MinValidatorStake        uint64 // Min validator stake (for TransformSubnetTx)
	MaxValidatorStake        uint64 // Max validator stake (for TransformSubnetTx)
	MinStakeDuration         uint32 // Min stake duration (for TransformSubnetTx)
	MaxStakeDuration         uint32 // Max stake duration (for TransformSubnetTx)
	MinDelegationFee         uint32 // Min delegation fee (for TransformSubnetTx)
	MinDelegatorStake        uint64 // Min delegator stake (for TransformSubnetTx)
	MaxValidatorWeightFactor byte   // Max validator weight factor (for TransformSubnetTx)
	UptimeRequirement        uint32 // Uptime requirement (for TransformSubnetTx)
}

type PChainOwner struct {
	BaseEntity
	TxID    string          `gorm:"type:varchar(50);not null;index"` // Transaction ID
	Type    PChainOwnerType `gorm:"type:varchar(40);not null"`       // Owner type (validator, delegation, subnet)
	Address string          `gorm:"type:varchar(60);not null"`       // Owner address
}

type PChainTxInput struct {
	TxInput
}

type PChainTxOutput struct {
	TxOutput
	Type PChainOutputType `gorm:"type:varchar(20)"` // Transaction output type (default or "stake" output)
}
