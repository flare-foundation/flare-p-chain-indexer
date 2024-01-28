package database

// X-chain types

type XChainTxType string

const (
	XChainBaseTx   XChainTxType = "BASE_TX"
	XChainImportTx XChainTxType = "IMPORT_TX"
)

// P-chain types

type PChainTxType string

const (
	PChainRewardValidatorTx    PChainTxType = "REWARD_TX"
	PChainAddDelegatorTx       PChainTxType = "ADD_DELEGATOR_TX"
	PChainAddValidatorTx       PChainTxType = "ADD_VALIDATOR_TX"
	PChainImportTx             PChainTxType = "IMPORT_TX"
	PChainExportTx             PChainTxType = "EXPORT_TX"
	PChainAdvanceTimeTx        PChainTxType = "ADVANCE_TIME_TX"
	PChainCreateChainTx        PChainTxType = "CREATE_CHAIN_TX"
	PChainCreateSubnetTx       PChainTxType = "CREATE_SUBNET_TX"
	PChainAddSubnetValidatorTx PChainTxType = "ADD_SUBNET_VALIDATOR_TX"
	PChainUnknownTx            PChainTxType = "UNKNOWN_TX"
)

type PChainBlockType string

const (
	PChainProposalBlock PChainBlockType = "PROPOSAL_BLOCK"
	PChainCommitBlock   PChainBlockType = "COMMIT_BLOCK"
	PChainAbortBlock    PChainBlockType = "ABORT_BLOCK"
	PChainStandardBlock PChainBlockType = "STANDARD_BLOCK"
)

type OutputType string

const (
	DefaultOutput      OutputType = "TX"
	ExportOutput       OutputType = "EXPORT"
	PChainStakeOutput  OutputType = "STAKE"
	PChainRewardOutput OutputType = "REWARD"
	PChainExportOutput OutputType = "EXPORT"
)

type InputType string

const (
	DefaultInput InputType = "TX"
	ImportInput  InputType = "IMPORT"
)

// Misc other types

type MigrationStatus string

const (
	MigrationPending   MigrationStatus = "PENDING"
	MigrationCompleted MigrationStatus = "COMPLETED"
	MigrationFailed    MigrationStatus = "FAILED"
)
