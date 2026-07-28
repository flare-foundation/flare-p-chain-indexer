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
