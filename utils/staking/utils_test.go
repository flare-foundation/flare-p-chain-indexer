package staking

import (
	"flare-indexer/database"
	"flare-indexer/utils/chain"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"
)

func testPChainTxData(id uint64, txID string, inputAddress string, inputIndex int) database.PChainTxData {
	tx := database.PChainTxData{
		PChainTx: database.PChainTx{
			BaseEntity: database.BaseEntity{ID: id},
			TxID:       &txID,
		},
		InputAddress: inputAddress,
		InputIndex:   uint32(inputIndex),
	}
	return tx
}

func TestDedupeTxs(t *testing.T) {
	testPChainTxData := []database.PChainTxData{
		testPChainTxData(1, "tx1", "addr2", 0),
		testPChainTxData(2, "tx1", "addr1", 0),
		testPChainTxData(3, "tx1", "addr3", 1),
		testPChainTxData(4, "tx2", "addr4", 0),
		testPChainTxData(6, "tx3", "addr1", 0),
		testPChainTxData(5, "tx3", "addr2", 0),
		testPChainTxData(7, "tx3", "addr6", 1),
		testPChainTxData(8, "tx3", "addr7", 1),
		testPChainTxData(9, "tx4", "addr1", 0),
		testPChainTxData(10, "tx4", "addr2", 0),
	}

	dedupedTxs := DedupeTxs(testPChainTxData)

	expectedSequence := [][]string{
		{"tx1", "addr1"},
		{"tx2", "addr4"},
		{"tx3", "addr1"},
		{"tx4", "addr1"},
	}

	if len(dedupedTxs) != len(expectedSequence) {
		t.Fatalf("Expected %d deduped txs, got %d", len(expectedSequence), len(dedupedTxs))
	}

	for i, tx := range dedupedTxs {
		expectedTxID := expectedSequence[i][0]
		expectedAddress := expectedSequence[i][1]
		if *tx.TxID != expectedTxID {
			t.Errorf("At index %d, expected TxID %s, got %s", i, expectedTxID, *tx.TxID)
		}
		if tx.InputAddress != expectedAddress {
			t.Errorf("At index %d, expected InputAddress %s, got %s", i, expectedAddress, tx.InputAddress)
		}
	}

}

func TestGetTxType(t *testing.T) {
	validatorTypes := []database.PChainTxType{
		database.PChainAddValidatorTx, database.PChainAddPermissionlessValidatorTx,
		// first and renewed validation cycles of auto-renewed validators
		database.PChainAddAutoRenewedValidatorTx, database.PChainRewardAutoRenewedValidatorTx,
	}
	for _, txType := range validatorTypes {
		stakingType, err := GetTxType(txType)
		require.NoError(t, err, txType)
		require.Equal(t, uint8(0), stakingType, txType)
	}

	delegatorTypes := []database.PChainTxType{database.PChainAddDelegatorTx, database.PChainAddPermissionlessDelegatorTx}
	for _, txType := range delegatorTypes {
		stakingType, err := GetTxType(txType)
		require.NoError(t, err, txType)
		require.Equal(t, uint8(1), stakingType, txType)
	}

	for _, txType := range []database.PChainTxType{database.PChainRewardValidatorTx, database.PChainSetAutoRenewedValidatorConfigTx, database.PChainBaseTx} {
		_, err := GetTxType(txType)
		require.Error(t, err, txType)
	}
}

// A renewed validation cycle of an auto-renewed validator is stored on the reward tx row and enters the
// merkle tree as a validator stake under the reward tx id, with the input address of the add tx
func TestToStakeDataRenewedCycle(t *testing.T) {
	// ToStakeData parses the input address with the configured HRP
	previousHRP := chain.AddressHRP
	chain.AddressHRP = "costwo"
	defer func() { chain.AddressHRP = previousHRP }()

	startTime := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	endTime := startTime.Add(60 * 24 * time.Hour)
	rewardTxID := "2p32tpqNrfzP3SStbP9bQGHZtJkCxjV3iHNssVnkcpUWxHMSuj"
	period := uint64(endTime.Sub(startTime).Seconds())
	tx := database.PChainTxData{
		PChainTx: database.PChainTx{
			Type:       database.PChainRewardAutoRenewedValidatorTx,
			TxID:       &rewardTxID,
			RewardTxID: "XnfV79XVMyuXbTw8iNreQ9FrUgy9csYBJp1xRscay3oDzhyq8",
			NodeID:     "NodeID-CZYx3on11wwYXFoHwZtAQZT5unZ9JHMf6",
			StartTime:  &startTime,
			EndTime:    &endTime,
			Weight:     1_000_000_000_000,
			Period:     &period,
		},
		InputAddress: "costwo18atl0e95w5ym6t8u5yrjpz35vqqzxfzrrsnq8u",
		InputIndex:   0,
	}

	stakeData, err := ToStakeData(&tx)
	require.NoError(t, err)

	expectedTxID, err := ids.FromString(rewardTxID)
	require.NoError(t, err)
	expectedNodeID, err := ids.NodeIDFromString(tx.NodeID)
	require.NoError(t, err)
	require.Equal(t, [32]byte(expectedTxID), stakeData.TxId)
	require.Equal(t, uint8(0), stakeData.StakingType)
	require.Equal(t, [20]byte(expectedNodeID), stakeData.NodeId)
	require.Equal(t, uint64(startTime.Unix()), stakeData.StartTime)
	require.Equal(t, uint64(endTime.Unix()), stakeData.EndTime)
	require.Equal(t, tx.Weight, stakeData.Weight)
}
