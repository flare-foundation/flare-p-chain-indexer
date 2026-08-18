package pchain

import (
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/platformvm/fx"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
)

type StakerTx interface {
	txs.UnsignedTx
	txs.Staker

	Stake() []*avax.TransferableOutput
}

type BoundedStakerTx interface {
	StakerTx

	StartTime() time.Time
	EndTime() time.Time
}

type ValidatorTx interface {
	ValidationRewardsOwner() fx.Owner
	DelegationRewardsOwner() fx.Owner
	Shares() uint32
}

type BoundedValidatorTx interface {
	ValidatorTx
	BoundedStakerTx
}

type DelegatorTx interface {
	BoundedStakerTx

	RewardsOwner() fx.Owner
}

type RewardTx interface {
	txs.UnsignedTx

	StakerTxID() ids.ID
}
