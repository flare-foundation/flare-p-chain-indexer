//go:build integration
// +build integration

package database

import (
	"flare-indexer/config"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/stretchr/testify/require"
)

// Only primary-network stakes may be voted on, mirrored, or uptime-voted: a subnet
// stake passes the tx-type filters but must never enter the merkle tree, where it
// would be mirrored as primary-network voting power.
func TestStakingQueriesExcludeSubnetStakes(t *testing.T) {
	db, err := ConnectTestDB(&config.DBConfig{
		Username: MysqlTestUser,
		Password: MysqlTestPassword,
		Host:     MysqlTestHost,
		Port:     MysqlTestPort,
		Database: "flare_indexer_services",
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(PChainTx{}, PChainTxInput{}))

	// A far-future window so the rows cannot collide with the seeded test data.
	epochStart := time.Date(2050, 1, 1, 0, 0, 0, 0, time.UTC)
	epochEnd := epochStart.Add(24 * time.Hour)
	startTime := epochStart.Add(time.Hour)
	endTime := epochEnd.Add(1000 * time.Hour)

	stakingTx := func(txID string, subnetID string) *PChainTx {
		id := txID
		return &PChainTx{
			Type:      PChainAddPermissionlessValidatorTx,
			TxID:      &id,
			BlockID:   "block",
			NodeID:    "NodeID-CZYx3on11wwYXFoHwZtAQZT5unZ9JHMf6",
			StartTime: &startTime,
			EndTime:   &endTime,
			Weight:    100,
			SubnetID:  subnetID,
			Timestamp: startTime,
		}
	}

	const (
		primaryTxID    = "subnetfilter-primary"
		legacyTxID     = "subnetfilter-legacy"
		legacyNullTxID = "subnetfilter-legacy-null"
		subnetTxID     = "subnetfilter-subnet"
	)
	txs := []*PChainTx{
		// Regular primary-network stake, as the indexer writes it today.
		stakingTx(primaryTxID, constants.PrimaryNetworkID.String()),
		// Rows from before the subnet_id column existed: written as empty by old code,
		// or NULL where the column was added to existing rows by auto-migration.
		stakingTx(legacyTxID, ""),
		stakingTx(legacyNullTxID, ""),
		// Stake on some other subnet: must never be voted or mirrored.
		stakingTx(subnetTxID, "2b175hLJhGdj3CzgXENso9CmwMgejaCQXhMFzBsm8hXbH2MF7H"),
	}
	inputs := []*PChainTxInput{
		{TxInput: TxInput{InIdx: 0, TxID: primaryTxID, Address: "addr1", Amount: 1}},
		{TxInput: TxInput{InIdx: 0, TxID: legacyTxID, Address: "addr2", Amount: 1}},
		{TxInput: TxInput{InIdx: 0, TxID: legacyNullTxID, Address: "addr3", Amount: 1}},
		{TxInput: TxInput{InIdx: 0, TxID: subnetTxID, Address: "addr4", Amount: 1}},
	}
	require.NoError(t, db.Create(txs).Error)
	require.NoError(t, db.Create(inputs).Error)
	require.NoError(t,
		db.Exec("UPDATE p_chain_txes SET subnet_id = NULL WHERE tx_id = ?", legacyNullTxID).Error)
	defer func() {
		ids := []string{primaryTxID, legacyTxID, legacyNullTxID, subnetTxID}
		require.NoError(t, db.Where("tx_id IN ?", ids).Delete(&PChainTxInput{}).Error)
		require.NoError(t, db.Where("tx_id IN ?", ids).Delete(&PChainTx{}).Error)
	}()

	txIDsOf := func(rows []PChainTxData) []string {
		out := make([]string, len(rows))
		for i, r := range rows {
			out[i] = *r.TxID
		}
		return out
	}

	// The two epoch queries feeding voting, mirroring and the services proofs.
	votingRows, err := FetchPChainVotingData(db, epochStart, epochEnd)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{primaryTxID, legacyTxID, legacyNullTxID}, txIDsOf(votingRows))

	mirrorRows, err := GetPChainTxsForEpoch(&GetPChainTxsForEpochInput{
		DB: db, StartTimestamp: epochStart, EndTimestamp: epochEnd,
	})
	require.NoError(t, err)
	require.ElementsMatch(t, []string{primaryTxID, legacyTxID, legacyNullTxID}, txIDsOf(mirrorRows))

	// The staking intervals feeding uptime voting.
	intervals, err := FetchNodeStakingIntervals(db,
		[]PChainTxType{PChainAddValidatorTx, PChainAddPermissionlessValidatorTx},
		epochStart, epochEnd)
	require.NoError(t, err)
	intervalIDs := make([]string, len(intervals))
	for i, tx := range intervals {
		intervalIDs[i] = *tx.TxID
	}
	require.ElementsMatch(t, []string{primaryTxID, legacyTxID, legacyNullTxID}, intervalIDs)

	// Sanity: the exclusion is by subnet id, not by accident of the fixture.
	var subnetRows int64
	require.NoError(t, db.Model(&PChainTx{}).Where("tx_id = ?", subnetTxID).Count(&subnetRows).Error)
	require.EqualValues(t, 1, subnetRows, "the subnet stake row exists and was filtered by the queries")
}

// Auto-renewed validators: the first validation cycle is on the add tx row, every renewed cycle on the
// row of the reward tx that started it. A reward tx has no inputs of its own, so the staking queries
// take the input address (and input index) of a renewed cycle from the add tx.
func TestStakingQueriesIncludeAutoRenewedCycles(t *testing.T) {
	db, err := ConnectTestDB(&config.DBConfig{
		Username: MysqlTestUser,
		Password: MysqlTestPassword,
		Host:     MysqlTestHost,
		Port:     MysqlTestPort,
		Database: "flare_indexer_services",
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(PChainTx{}, PChainTxInput{}))

	// Two far-future epoch windows: the add tx starts in the first one, the renewed cycle in the second
	epoch1Start := time.Date(2051, 1, 1, 0, 0, 0, 0, time.UTC)
	epoch1End := epoch1Start.Add(24 * time.Hour)
	epoch2Start := time.Date(2051, 3, 1, 0, 0, 0, 0, time.UTC)
	epoch2End := epoch2Start.Add(24 * time.Hour)

	addStart := epoch1Start.Add(time.Hour)
	cycleEnd := epoch2Start.Add(30 * time.Minute) // end of the first cycle = start of the renewed one
	renewedEnd := cycleEnd.Add(60 * 24 * time.Hour)
	period := uint64(60 * 24 * 3600)
	exited := uint64(0)

	const (
		nodeID        = "NodeID-CZYx3on11wwYXFoHwZtAQZT5unZ9JHMf6"
		addTxID       = "autorenew-add"
		configTxID    = "autorenew-config"
		renewedTxID   = "autorenew-renewed"
		exitedTxID    = "autorenew-exited"
		addAddress    = "autorenew-addr-1"
		addAddress2   = "autorenew-addr-2"
		configAddress = "autorenew-config-addr"
	)
	// Block heights far above the seeded test data; the config tx is packed into the proposal block of
	// the reward tx, as the block builder does since Durango
	const (
		addHeight    uint64 = 950_001
		rewardHeight uint64 = 950_010
		exitHeight   uint64 = 950_020
	)
	strPtr := func(s string) *string { return &s }
	primary := constants.PrimaryNetworkID.String()

	txs := []*PChainTx{
		{Type: PChainAddAutoRenewedValidatorTx, TxID: strPtr(addTxID), BlockID: "block", BlockHeight: addHeight, Timestamp: addStart,
			NodeID: nodeID, Weight: 100, SubnetID: primary, StartTime: &addStart, EndTime: &cycleEnd, Period: &period},
		{Type: PChainSetAutoRenewedValidatorConfigTx, TxID: strPtr(configTxID), BlockID: "block", BlockHeight: rewardHeight, Timestamp: cycleEnd,
			RewardTxID: addTxID, Period: &period},
		// renewed cycle
		{Type: PChainRewardAutoRenewedValidatorTx, TxID: strPtr(renewedTxID), BlockID: "block", BlockHeight: rewardHeight, Timestamp: cycleEnd,
			RewardTxID: addTxID, BlockTime: &cycleEnd, Time: &cycleEnd,
			NodeID: nodeID, Weight: 100, SubnetID: primary, StartTime: &cycleEnd, EndTime: &renewedEnd, Period: &period},
		// the validator exited at the end of the renewed cycle
		{Type: PChainRewardAutoRenewedValidatorTx, TxID: strPtr(exitedTxID), BlockID: "block", BlockHeight: exitHeight, Timestamp: renewedEnd,
			RewardTxID: addTxID, BlockTime: &renewedEnd, Time: &renewedEnd, Period: &exited},
	}
	inputs := []*PChainTxInput{
		{TxInput: TxInput{InIdx: 0, TxID: addTxID, Address: addAddress, Amount: 1}},
		{TxInput: TxInput{InIdx: 1, TxID: addTxID, Address: addAddress2, Amount: 1}},
		{TxInput: TxInput{InIdx: 0, TxID: configTxID, Address: configAddress, Amount: 1}},
	}
	require.NoError(t, db.Create(txs).Error)
	require.NoError(t, db.Create(inputs).Error)
	defer func() {
		ids := []string{addTxID, configTxID, renewedTxID, exitedTxID}
		require.NoError(t, db.Where("tx_id IN ?", ids).Delete(&PChainTxInput{}).Error)
		require.NoError(t, db.Where("tx_id IN ?", ids).Delete(&PChainTx{}).Error)
	}()

	type leaf struct {
		txID    string
		address string
		index   uint32
	}
	leavesOf := func(rows []PChainTxData) []leaf {
		out := make([]leaf, len(rows))
		for i, r := range rows {
			out[i] = leaf{*r.TxID, r.InputAddress, r.InputIndex}
		}
		return out
	}
	addInputs := func(txID string) []leaf {
		return []leaf{{txID, addAddress, 0}, {txID, addAddress2, 1}}
	}

	// The first cycle is voted in the epoch of the add tx, with the add tx's inputs
	rows, err := FetchPChainVotingData(db, epoch1Start, epoch1End)
	require.NoError(t, err)
	require.ElementsMatch(t, addInputs(addTxID), leavesOf(rows))

	// The renewed cycle is voted in the epoch of its start, under the reward tx id, with the inputs of
	// the add tx (not those of the config tx)
	rows, err = FetchPChainVotingData(db, epoch2Start, epoch2End)
	require.NoError(t, err)
	require.ElementsMatch(t, addInputs(renewedTxID), leavesOf(rows))

	rows, err = GetPChainTxsForEpoch(&GetPChainTxsForEpochInput{DB: db, StartTimestamp: epoch2Start, EndTimestamp: epoch2End})
	require.NoError(t, err)
	require.ElementsMatch(t, addInputs(renewedTxID), leavesOf(rows))

	// An exited validator has no cycle to vote on
	rows, err = FetchPChainVotingData(db, renewedEnd.Add(-time.Hour), renewedEnd.Add(time.Hour))
	require.NoError(t, err)
	require.Empty(t, rows)

	// Uptime voting sees both cycles of the node in the epoch where they meet
	intervals, err := FetchNodeStakingIntervals(db,
		[]PChainTxType{PChainAddAutoRenewedValidatorTx, PChainRewardAutoRenewedValidatorTx}, epoch2Start, epoch2End)
	require.NoError(t, err)
	intervalIDs := make([]string, len(intervals))
	for i, tx := range intervals {
		intervalIDs[i] = *tx.TxID
	}
	require.ElementsMatch(t, []string{addTxID, renewedTxID}, intervalIDs)

	// Address registration looks the input index of an address up on the add tx
	txData, err := FetchPChainTxData(db, addTxID, addAddress2)
	require.NoError(t, err)
	require.NotNil(t, txData)
	require.Equal(t, uint32(1), txData.InputIndex)

	// Listing validator transactions by type skips the reward tx of an exited validator
	txIDs, err := FetchPChainStakingTransactions(db, []PChainTxType{PChainRewardAutoRenewedValidatorTx}, "", "", time.Time{}, 0, 100)
	require.NoError(t, err)
	require.Equal(t, []string{renewedTxID}, txIDs)
}
