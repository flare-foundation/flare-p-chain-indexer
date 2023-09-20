//go:build !integration
// +build !integration

package cronjob

import (
	globalConfig "flare-indexer/config"
	"flare-indexer/database"
	"flare-indexer/indexer/config"
	"flare-indexer/indexer/pchain"
	"flare-indexer/utils"
	"flare-indexer/utils/contracts/mirroring"
	"flare-indexer/utils/staking"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/bradleyjkemp/cupaloy"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	cfg := config.Config{
		Chain: globalConfig.ChainConfig{
			ChainAddressHRP: "costwo",
		},
		Logger: globalConfig.LoggerConfig{
			Level:   "DEBUG",
			Console: true,
		},
	}
	globalConfig.GlobalConfigCallback.Call(cfg)

	m.Run()
}

func TestOneTransaction(t *testing.T) {
	epochs := initEpochs()

	startTime := epochs.GetStartTime(3)
	endTime := epochs.GetEndTime(999)

	txid := "5uZETr5SUKqGJLzFP5BeGxbXU5CFcCBQYPu288eX9R1QDQMjn"
	tx := database.PChainTxData{
		PChainTx: database.PChainTx{
			ChainID:   "costwo",
			NodeID:    "NodeID-CZYx3on11wwYXFoHwZtAQZT5unZ9JHMf6",
			StartTime: &startTime,
			EndTime:   &endTime,
			TxID:      &txid,
			Type:      database.PChainAddDelegatorTx,
		},
		InputAddress: "costwo18atl0e95w5ym6t8u5yrjpz35vqqzxfzrrsnq8u",
	}

	txs := map[int64][]database.PChainTxData{
		3: {tx},
	}

	txHash, err := staking.HashTransaction(&tx)
	require.NoError(t, err)

	merkleRoots := map[int64][32]byte{
		3: txHash,
	}

	contracts := testContracts{
		merkleRoots: merkleRoots,
	}

	db := testMirror(t, txs, contracts, epochs)

	require.Equal(t, db.states[mirrorStateName].NextDBIndex, uint64(3))
}

func TestMultipleTransactionsInEpoch(t *testing.T) {
	epochs := initEpochs()

	startTime := epochs.GetStartTime(3)
	endTime := epochs.GetEndTime(999)

	txs := make([]database.PChainTxData, 3)
	txIDs := []string{
		"XnfV79XVMyuXbTw8iNreQ9FrUgy9csYBJp1xRscay3oDzhyq8",
		"nsPmyQbm4oo77jyykxbjf7s4Zp4urNptkyAouxVWZ2EB2kw1z",
		"2p32tpqNrfzP3SStbP9bQGHZtJkCxjV3iHNssVnkcpUWxHMSuj",
	}

	for i := 0; i < 3; i++ {
		txs[i] = database.PChainTxData{
			PChainTx: database.PChainTx{
				ChainID:   "costwo",
				NodeID:    "NodeID-CZYx3on11wwYXFoHwZtAQZT5unZ9JHMf6",
				StartTime: &startTime,
				EndTime:   &endTime,
				TxID:      &txIDs[i],
				Type:      database.PChainAddDelegatorTx,
			},
			InputAddress: "costwo18atl0e95w5ym6t8u5yrjpz35vqqzxfzrrsnq8u",
		}
	}

	txsMap := map[int64][]database.PChainTxData{
		3: txs,
	}

	root := common.HexToHash("b3ec965b802c71f9058d2ed4d80bdf5af902a3741a75221992c5eb2f879a116c")

	merkleRoots := map[int64][32]byte{
		3: root,
	}

	contracts := testContracts{
		merkleRoots: merkleRoots,
	}

	db := testMirror(t, txsMap, contracts, epochs)

	require.Equal(t, db.states[mirrorStateName].NextDBIndex, uint64(3))
}

func TestMultipleTransactionsInSeparateEpochs(t *testing.T) {
	epochs := initEpochs()

	startTime := epochs.GetStartTime(3)
	endTime := epochs.GetEndTime(999)

	txs := make([]database.PChainTxData, 3)
	txIDs := []string{
		"XnfV79XVMyuXbTw8iNreQ9FrUgy9csYBJp1xRscay3oDzhyq8",
		"nsPmyQbm4oo77jyykxbjf7s4Zp4urNptkyAouxVWZ2EB2kw1z",
		"2p32tpqNrfzP3SStbP9bQGHZtJkCxjV3iHNssVnkcpUWxHMSuj",
	}

	for i := 0; i < 3; i++ {
		txs[i] = database.PChainTxData{
			PChainTx: database.PChainTx{
				ChainID:   "costwo",
				NodeID:    "NodeID-CZYx3on11wwYXFoHwZtAQZT5unZ9JHMf6",
				StartTime: &startTime,
				EndTime:   &endTime,
				TxID:      &txIDs[i],
				Type:      database.PChainAddDelegatorTx,
			},
			InputAddress: "costwo18atl0e95w5ym6t8u5yrjpz35vqqzxfzrrsnq8u",
		}
	}

	txsMap := make(map[int64][]database.PChainTxData, 3)
	for i := 0; i < 3; i++ {
		txsMap[int64(i)] = []database.PChainTxData{txs[i]}
	}

	merkleRoots := make(map[int64][32]byte, 3)
	for i := 0; i < 3; i++ {
		txHash, err := staking.HashTransaction(&txs[i])
		require.NoError(t, err)

		merkleRoots[int64(i)] = txHash
	}

	contracts := testContracts{
		merkleRoots: merkleRoots,
	}

	db := testMirror(t, txsMap, contracts, epochs)

	require.Equal(t, db.states[mirrorStateName].NextDBIndex, uint64(2))
}

func TestAlreadyMirrored(t *testing.T) {
	testMirrorErrors(t, "transaction already mirrored")
}

func TestStakingEnded(t *testing.T) {
	testMirrorErrors(t, "staking already ended")
}

func testMirrorErrors(t *testing.T, errorMsg string) {
	epochs := initEpochs()

	startTime := epochs.GetStartTime(3)
	endTime := epochs.GetEndTime(999)

	txid := "5uZETr5SUKqGJLzFP5BeGxbXU5CFcCBQYPu288eX9R1QDQMjn"
	tx := database.PChainTxData{
		PChainTx: database.PChainTx{
			ChainID:   "costwo",
			NodeID:    "NodeID-CZYx3on11wwYXFoHwZtAQZT5unZ9JHMf6",
			StartTime: &startTime,
			EndTime:   &endTime,
			TxID:      &txid,
			Type:      database.PChainAddDelegatorTx,
		},
		InputAddress: "costwo18atl0e95w5ym6t8u5yrjpz35vqqzxfzrrsnq8u",
	}

	txs := map[int64][]database.PChainTxData{
		3: {tx},
	}

	txHash, err := staking.HashTransaction(&tx)
	require.NoError(t, err)

	merkleRoots := map[int64][32]byte{
		3: txHash,
	}

	txidBytes, err := ids.FromString(*tx.TxID)
	require.NoError(t, err)

	contracts := testContracts{
		merkleRoots: merkleRoots,
		mirrorErrors: map[[32]byte]error{
			txidBytes: errors.New(errorMsg),
		},
	}

	db := testMirror(t, txs, contracts, epochs)

	require.Equal(t, db.states[mirrorStateName].NextDBIndex, uint64(3))
}

func initEpochs() staking.EpochInfo {
	epochCfg := globalConfig.EpochConfig{
		Period: 180 * time.Second,
		Start:  utils.Timestamp{Time: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)},
	}

	return staking.NewEpochInfo(&epochCfg)
}

func testMirror(
	t *testing.T,
	txs map[int64][]database.PChainTxData,
	contracts testContracts,
	epochs staking.EpochInfo,
) *testDB {
	db := testDB{
		epochs: epochs,
		states: map[string]database.State{
			pchain.StateName: {
				Updated: epochs.GetEndTime(999),
			},
			mirrorStateName: {},
		},
		txs: txs,
	}

	j := mirrorCronJob{
		db:        db,
		contracts: &contracts,
		epochCronjob: epochCronjob{
			enabled: true,
			epochs:  epochs,
		},
	}

	err := j.Call()
	require.NoError(t, err)

	cupaloy.SnapshotT(t, contracts.mirroredStakes)

	return &db
}

type testDB struct {
	epochs staking.EpochInfo
	states map[string]database.State
	txs    map[int64][]database.PChainTxData
}

func (db testDB) FetchState(name string) (database.State, error) {
	state, ok := db.states[name]
	if !ok {
		return state, errors.New("not found")
	}

	return state, nil
}

func (db testDB) UpdateJobState(epoch int64) error {
	db.states[mirrorStateName] = database.State{
		Name:        mirrorStateName,
		NextDBIndex: uint64(epoch),
	}

	return nil
}

func (db testDB) GetPChainTxsForEpoch(start, end time.Time) ([]database.PChainTxData, error) {
	epoch := db.epochs.GetEpochIndex(start)
	return db.txs[epoch], nil
}

func (db testDB) GetPChainTx(txID string) (*database.PChainTx, error) {
	return nil, nil
}

type testContracts struct {
	merkleRoots    map[int64][32]byte
	mirroredStakes []mirrorStakeInput
	mirrorErrors   map[[32]byte]error
}

type mirrorStakeInput struct {
	stakeData   *mirroring.IPChainStakeMirrorVerifierPChainStake
	merkleProof [][32]byte
}

func (c testContracts) GetMerkleRoot(epoch int64) ([32]byte, error) {
	return c.merkleRoots[epoch], nil
}

func (c *testContracts) MirrorStake(
	stakeData *mirroring.IPChainStakeMirrorVerifierPChainStake,
	merkleProof [][32]byte,
) error {
	if err := c.mirrorErrors[stakeData.TxId]; err != nil {
		return err
	}

	c.mirroredStakes = append(c.mirroredStakes, mirrorStakeInput{
		stakeData:   stakeData,
		merkleProof: merkleProof,
	})
	return nil
}

func (c testContracts) IsAddressRegistered(address string) (bool, error) {
	return false, nil
}

func (c testContracts) RegisterPublicKey(publicKey []byte) error {
	return nil
}
