package pchain

import (
	"flare-indexer/database"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/stretchr/testify/require"
)

func TestApplyAutoRenewal(t *testing.T) {
	const (
		addTxID    = "add"
		rewardTxID = "reward"
		configTxID = "config"
		nodeID     = "NodeID-CZYx3on11wwYXFoHwZtAQZT5unZ9JHMf6"
	)
	blockTime := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	period := func(p uint64) *uint64 { return &p }
	strPtr := func(s string) *string { return &s }

	addTx := func() *database.PChainTx {
		return &database.PChainTx{
			Type:          database.PChainAddAutoRenewedValidatorTx,
			TxID:          strPtr(addTxID),
			NodeID:        nodeID,
			Weight:        5000,
			SubnetID:      constants.PrimaryNetworkID.String(),
			FeePercentage: 20000,
			Period:        period(3600),
		}
	}
	rewardTx := func() *database.PChainTx {
		return &database.PChainTx{
			Type:       database.PChainRewardAutoRenewedValidatorTx,
			TxID:       strPtr(rewardTxID),
			RewardTxID: addTxID,
			BlockTime:  &blockTime,
			Time:       &blockTime,
		}
	}
	configTx := func(p uint64) *database.PChainTx {
		return &database.PChainTx{
			Type:       database.PChainSetAutoRenewedValidatorConfigTx,
			TxID:       strPtr(configTxID),
			RewardTxID: addTxID,
			Period:     period(p),
		}
	}

	requireRenewed := func(t *testing.T, tx *database.PChainTx, periodSeconds uint64) {
		t.Helper()
		require.NotNil(t, tx.Period)
		require.Equal(t, periodSeconds, *tx.Period)
		require.NotNil(t, tx.StartTime)
		require.True(t, blockTime.Equal(*tx.StartTime), "cycle starts at the block time")
		require.NotNil(t, tx.EndTime)
		require.True(t, blockTime.Add(time.Duration(periodSeconds)*time.Second).Equal(*tx.EndTime))
		require.Equal(t, nodeID, tx.NodeID)
		require.Equal(t, uint64(5000), tx.Weight)
		require.Equal(t, constants.PrimaryNetworkID.String(), tx.SubnetID)
		require.Equal(t, uint32(20000), tx.FeePercentage, "delegation fee preserved")
	}
	requireExited := func(t *testing.T, tx *database.PChainTx) {
		t.Helper()
		require.NotNil(t, tx.Period)
		require.Equal(t, uint64(0), *tx.Period)
		require.Nil(t, tx.StartTime)
		require.Nil(t, tx.EndTime)
		require.Empty(t, tx.NodeID, "an exited validator has no cycle to vote on")
	}

	t.Run("committed without config: renewed for the add tx period", func(t *testing.T) {
		tx := rewardTx()
		require.NoError(t, applyAutoRenewal(tx, addTx(), nil, true))
		requireRenewed(t, tx, 3600)
	})

	t.Run("committed with config: renewed for the config period", func(t *testing.T) {
		tx := rewardTx()
		require.NoError(t, applyAutoRenewal(tx, addTx(), configTx(7200), true))
		requireRenewed(t, tx, 7200)
	})

	t.Run("committed with config period 0: graceful exit", func(t *testing.T) {
		tx := rewardTx()
		require.NoError(t, applyAutoRenewal(tx, addTx(), configTx(0), true))
		requireExited(t, tx)
	})

	t.Run("aborted: exit even with a configured period", func(t *testing.T) {
		tx := rewardTx()
		require.NoError(t, applyAutoRenewal(tx, addTx(), configTx(7200), false))
		requireExited(t, tx)
	})

	t.Run("cycle end time differing from block time is tolerated", func(t *testing.T) {
		tx := rewardTx()
		other := blockTime.Add(time.Second)
		tx.Time = &other
		require.NoError(t, applyAutoRenewal(tx, addTx(), nil, true))
		requireRenewed(t, tx, 3600)
	})

	t.Run("referenced tx of another type is an error", func(t *testing.T) {
		add := addTx()
		add.Type = database.PChainAddPermissionlessValidatorTx
		require.Error(t, applyAutoRenewal(rewardTx(), add, nil, true))
	})

	t.Run("committed without block time is an error", func(t *testing.T) {
		tx := rewardTx()
		tx.BlockTime = nil
		require.Error(t, applyAutoRenewal(tx, addTx(), nil, true))
	})

	t.Run("missing periods are errors", func(t *testing.T) {
		add := addTx()
		add.Period = nil
		require.Error(t, applyAutoRenewal(rewardTx(), add, nil, true))
		cfg := configTx(0)
		cfg.Period = nil
		require.Error(t, applyAutoRenewal(rewardTx(), addTx(), cfg, true))
	})
}
