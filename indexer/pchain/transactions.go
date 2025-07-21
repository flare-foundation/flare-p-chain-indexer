package pchain

import (
	"time"

	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/platformvm/fx"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
)

type StakerTx interface {
	txs.UnsignedTx
	txs.Staker

	StartTime() time.Time
	Stake() []*avax.TransferableOutput
}

type ValidatorTx interface {
	StakerTx

	ValidationRewardsOwner() fx.Owner
	DelegationRewardsOwner() fx.Owner
	Shares() uint32
}

type DelegatorTx interface {
	StakerTx

	RewardsOwner() fx.Owner
}
