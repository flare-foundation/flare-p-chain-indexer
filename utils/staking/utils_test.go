package staking

import (
	"flare-indexer/database"
	"testing"
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
