//go:build integration
// +build integration

package pchain

import (
	"flare-indexer/database"
	"flare-indexer/indexer/context"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Block heights far above the recorded test blocks
const autoRenewTestHeight uint64 = 900_000

const (
	autoRenewNodeID = "NodeID-CZYx3on11wwYXFoHwZtAQZT5unZ9JHMf6"
	autoRenewWeight = uint64(5000)
	autoRenewFee    = uint32(20000)
)

// Fresh test database (BuildTestContext drops and recreates the tables)
func autoRenewTestDB(t *testing.T) *gorm.DB {
	ctx, err := context.BuildTestContext(pchainIndexerTestConfig(10, 0))
	require.NoError(t, err)
	return ctx.DB()
}

func strPtr(s string) *string { return &s }

// Rows as the batch indexer writes them

func addAutoRenewedTxRow(txID string, height uint64, blockTime time.Time, period uint64) *database.PChainTx {
	startTime := blockTime
	endTime := time.Unix(startTime.Unix()+int64(period), 0)
	return &database.PChainTx{
		Type: database.PChainAddAutoRenewedValidatorTx, TxID: strPtr(txID),
		BlockID: "block", BlockType: database.PChainStandardBlock, BlockHeight: height,
		Timestamp: blockTime, BlockTime: &blockTime,
		NodeID: autoRenewNodeID, Weight: autoRenewWeight, SubnetID: constants.PrimaryNetworkID.String(),
		FeePercentage: autoRenewFee,
		StartTime:     &startTime, EndTime: &endTime, Period: &period,
	}
}

func setConfigTxRow(txID string, height uint64, blockTime time.Time, stakerTxID string, period uint64) *database.PChainTx {
	return &database.PChainTx{
		Type: database.PChainSetAutoRenewedValidatorConfigTx, TxID: strPtr(txID),
		BlockID: "block", BlockType: database.PChainStandardBlock, BlockHeight: height,
		Timestamp: blockTime, BlockTime: &blockTime,
		RewardTxID: stakerTxID, Period: &period,
	}
}

// Pending: the outcome is applied once the commit/abort block is indexed
func rewardAutoRenewedTxRow(txID string, height uint64, cycleEnd time.Time, stakerTxID string) *database.PChainTx {
	return &database.PChainTx{
		Type: database.PChainRewardAutoRenewedValidatorTx, TxID: strPtr(txID),
		BlockID: "block", BlockType: database.PChainProposalBlock, BlockHeight: height,
		Timestamp: cycleEnd, BlockTime: &cycleEnd, Time: &cycleEnd,
		RewardTxID: stakerTxID,
	}
}

func optionBlockRow(height uint64, blockTime time.Time, blockType database.PChainBlockType) *database.PChainTx {
	return &database.PChainTx{
		BlockID: "block", BlockType: blockType, BlockHeight: height,
		Timestamp: blockTime, BlockTime: &blockTime,
	}
}

func fetchTx(t *testing.T, db *gorm.DB, txID string) *database.PChainTx {
	tx, err := database.FetchPChainTx(db, txID)
	require.NoError(t, err)
	return tx
}

func requireRenewedCycle(t *testing.T, tx *database.PChainTx, start time.Time, period uint64) {
	t.Helper()
	require.NotNil(t, tx.Period)
	require.Equal(t, period, *tx.Period)
	require.NotNil(t, tx.StartTime)
	require.True(t, start.Equal(*tx.StartTime), "start %v, expected %v", tx.StartTime, start)
	require.NotNil(t, tx.EndTime)
	require.True(t, start.Add(time.Duration(period)*time.Second).Equal(*tx.EndTime), "end %v", tx.EndTime)
	require.Equal(t, autoRenewNodeID, tx.NodeID)
	require.Equal(t, autoRenewWeight, tx.Weight)
	require.Equal(t, constants.PrimaryNetworkID.String(), tx.SubnetID)
	require.Equal(t, autoRenewFee, tx.FeePercentage)
}

func requireExited(t *testing.T, tx *database.PChainTx) {
	t.Helper()
	require.NotNil(t, tx.Period)
	require.Equal(t, uint64(0), *tx.Period)
	require.Nil(t, tx.StartTime)
	require.Nil(t, tx.EndTime)
}

func requirePending(t *testing.T, tx *database.PChainTx) {
	t.Helper()
	require.Nil(t, tx.Period)
	require.Nil(t, tx.StartTime)
	require.Nil(t, tx.EndTime)
}

func TestCompleteAutoRenewedRewardTxs(t *testing.T) {
	db := autoRenewTestDB(t)
	h := autoRenewTestHeight
	t0 := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	cycle1End := t0.Add(3600 * time.Second)
	cycle2End := cycle1End.Add(7200 * time.Second)

	rows := []*database.PChainTx{
		addAutoRenewedTxRow("ar-add", h, t0, 3600),
		// changes the period of the next cycle
		setConfigTxRow("ar-config-1", h+3, t0.Add(10*time.Minute), "ar-add", 7200),
		rewardAutoRenewedTxRow("ar-reward-1", h+10, cycle1End, "ar-add"),
		optionBlockRow(h+11, cycle1End, database.PChainCommitBlock),
		// stop at the end of the second cycle
		setConfigTxRow("ar-config-2", h+12, cycle1End.Add(time.Minute), "ar-add", 0),
		rewardAutoRenewedTxRow("ar-reward-2", h+20, cycle2End, "ar-add"),
		optionBlockRow(h+21, cycle2End, database.PChainCommitBlock),
	}
	require.NoError(t, database.CreatePChainEntities(db, rows, nil, nil))

	require.NoError(t, completeAutoRenewedRewardTxs(db, 0, h+100))

	requireRenewedCycle(t, fetchTx(t, db, "ar-reward-1"), cycle1End, 7200)
	requireExited(t, fetchTx(t, db, "ar-reward-2"))
	// the add tx row is untouched
	requireRenewedCycle(t, fetchTx(t, db, "ar-add"), t0, 3600)

	// Completed rows are not pending any more: a second run changes nothing
	before := fetchTx(t, db, "ar-reward-1")
	require.NoError(t, completeAutoRenewedRewardTxs(db, 0, h+100))
	after := fetchTx(t, db, "ar-reward-1")
	require.Equal(t, before.Period, after.Period)
	require.True(t, before.StartTime.Equal(*after.StartTime))
	require.True(t, before.EndTime.Equal(*after.EndTime))
}

func TestAutoRenewedRewardTxAborted(t *testing.T) {
	db := autoRenewTestDB(t)
	h := autoRenewTestHeight
	t0 := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	cycleEnd := t0.Add(3600 * time.Second)

	rows := []*database.PChainTx{
		addAutoRenewedTxRow("ar-add", h, t0, 3600),
		rewardAutoRenewedTxRow("ar-reward", h+10, cycleEnd, "ar-add"),
		// insufficient uptime: the validator is removed although a period is configured
		optionBlockRow(h+11, cycleEnd, database.PChainAbortBlock),
	}
	require.NoError(t, database.CreatePChainEntities(db, rows, nil, nil))

	require.NoError(t, completeAutoRenewedRewardTxs(db, 0, h+100))
	requireExited(t, fetchTx(t, db, "ar-reward"))
}

// The proposal block may be the last block of a batch and its commit block the first one of the next
func TestAutoRenewedRewardTxCompletedInNextBatch(t *testing.T) {
	db := autoRenewTestDB(t)
	h := autoRenewTestHeight
	t0 := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	cycleEnd := t0.Add(3600 * time.Second)

	batch1 := []*database.PChainTx{
		addAutoRenewedTxRow("ar-add", h, t0, 3600),
		rewardAutoRenewedTxRow("ar-reward", h+10, cycleEnd, "ar-add"),
	}
	require.NoError(t, database.CreatePChainEntities(db, batch1, nil, nil))
	require.NoError(t, completeAutoRenewedRewardTxsInBatch(db, batch1))
	requirePending(t, fetchTx(t, db, "ar-reward"))

	batch2 := []*database.PChainTx{
		optionBlockRow(h+11, cycleEnd, database.PChainCommitBlock),
	}
	require.NoError(t, database.CreatePChainEntities(db, batch2, nil, nil))
	require.NoError(t, completeAutoRenewedRewardTxsInBatch(db, batch2))
	requireRenewedCycle(t, fetchTx(t, db, "ar-reward"), cycleEnd, 3600)
}

// The latest config tx accepted before the reward tx counts, including one packed into the same
// proposal block (executed before the reward tx); a later config tx does not
func TestAutoRenewedConfigPrecedence(t *testing.T) {
	db := autoRenewTestDB(t)
	h := autoRenewTestHeight
	t0 := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	cycleEnd := t0.Add(3600 * time.Second)

	rows := []*database.PChainTx{
		addAutoRenewedTxRow("ar-add", h, t0, 3600),
		setConfigTxRow("ar-config-earlier", h+3, t0.Add(10*time.Minute), "ar-add", 7200),
		// same proposal block as the reward tx, inserted (and executed) before it
		setConfigTxRow("ar-config-same-block", h+10, cycleEnd, "ar-add", 5400),
		rewardAutoRenewedTxRow("ar-reward", h+10, cycleEnd, "ar-add"),
		optionBlockRow(h+11, cycleEnd, database.PChainCommitBlock),
		setConfigTxRow("ar-config-later", h+12, cycleEnd.Add(time.Minute), "ar-add", 9000),
	}
	require.NoError(t, database.CreatePChainEntities(db, rows, nil, nil))

	require.NoError(t, completeAutoRenewedRewardTxs(db, 0, h+100))
	requireRenewedCycle(t, fetchTx(t, db, "ar-reward"), cycleEnd, 5400)
}
