//go:build !integration
// +build !integration

package cronjob

import (
	globalConfig "flare-indexer/config"
	"flare-indexer/database"
	"flare-indexer/indexer/config"
	"flare-indexer/indexer/pchain"
	"flare-indexer/utils/contracts/mirroring"
	"flare-indexer/utils/staking"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/secp256k1"
	"github.com/ava-labs/libevm/common"
	"github.com/bradleyjkemp/cupaloy"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

var epochInfo = staking.EpochInfo{
	Period: 180 * time.Second,
	Start:  time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
}

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
	startTime := epochInfo.GetStartTime(3)
	endTime := epochInfo.GetEndTime(999)

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
		InputIndex:   0,
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

	db, _ := testMirror(t, txs, contracts)

	require.Equal(t, db.states[mirrorStateName].NextDBIndex, uint64(4))
}

func TestMultipleTransactionsInEpoch(t *testing.T) {
	startTime := epochInfo.GetStartTime(3)
	endTime := epochInfo.GetEndTime(999)

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
			InputIndex:   0,
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

	db, _ := testMirror(t, txsMap, contracts)

	require.Equal(t, db.states[mirrorStateName].NextDBIndex, uint64(4))
}

func TestMultipleTransactionsInSeparateEpochs(t *testing.T) {
	startTime := epochInfo.GetStartTime(3)
	endTime := epochInfo.GetEndTime(999)

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
			InputIndex:   0,
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

	db, _ := testMirror(t, txsMap, contracts)

	require.Equal(t, db.states[mirrorStateName].NextDBIndex, uint64(3))
}

func TestAlreadyMirrored(t *testing.T) {
	testMirrorErrors(t, "transaction already mirrored")
}

func TestStakingEnded(t *testing.T) {
	testMirrorErrors(t, "staking already ended")
}

func testMirrorErrors(t *testing.T, errorMsg string) {
	startTime := epochInfo.GetStartTime(3)
	endTime := epochInfo.GetEndTime(999)

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
		InputIndex:   0,
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

	db, _ := testMirror(t, txs, contracts)

	require.Equal(t, db.states[mirrorStateName].NextDBIndex, uint64(4))
}

func testMirror(
	t *testing.T,
	txs map[int64][]database.PChainTxData,
	contracts testContracts,
) (*testDB, *testContracts) {
	db := testDB{
		epochs: epochInfo,
		states: map[string]database.State{
			pchain.StateName: {
				Updated:        epochInfo.GetEndTime(999),
				NextDBIndex:    3,
				LastChainIndex: 2,
			},
			mirrorStateName: {},
		},
		txs:        txs,
		txRequests: make(map[string]int),
	}

	j := mirrorCronJob{
		db:        db,
		contracts: &contracts,
		epochCronjob: epochCronjob{
			enabled: true,
			epochs:  epochInfo,
		},
		registeredAddresses: mapset.NewSet[string](),
	}

	err := j.Call()
	require.NoError(t, err)

	cupaloy.SnapshotT(t, contracts.mirroredStakes)

	return &db, &contracts
}

type testDB struct {
	epochs staking.EpochInfo
	states map[string]database.State
	txs    map[int64][]database.PChainTxData

	// Tx ids requested through GetPChainTx (address registration), by number of requests
	txRequests map[string]int
}

func (db testDB) FetchState(name string) (database.State, error) {
	state, ok := db.states[name]
	if !ok {
		return state, errors.New("not found")
	}

	return state, nil
}

func (db testDB) UpdateJobState(epoch int64, force bool) error {
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

func (db testDB) GetPChainTx(txID string, address string) (*database.PChainTxData, error) {
	if db.txRequests != nil {
		db.txRequests[txID]++
	}
	return nil, nil
}

type testContracts struct {
	merkleRoots    map[int64][32]byte
	mirroredStakes []mirrorStakeInput
	mirrorErrors   map[[32]byte]error

	// Addresses reported as not registered on the address binder (all others are registered)
	unregisteredAddresses map[string]bool
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
	return !c.unregisteredAddresses[address], nil
}

func (c testContracts) RegisterPublicKey(publicKey *secp256k1.PublicKey) error {
	return nil
}

func (c testContracts) EpochConfig() (time.Time, time.Duration, error) {
	return epochInfo.Start, epochInfo.Period, nil
}

// A renewed validation cycle of an auto-renewed validator is stored on the reward tx row and is mirrored
// as a validator stake under the reward tx id; the staker's address is registered from the add tx, which
// holds the inputs and the signing credential
func TestAutoRenewedCycleMirrored(t *testing.T) {
	startTime := epochInfo.GetStartTime(3)
	endTime := epochInfo.GetEndTime(999)
	period := uint64(endTime.Sub(startTime).Seconds())

	addTxID := "XnfV79XVMyuXbTw8iNreQ9FrUgy9csYBJp1xRscay3oDzhyq8"
	rewardTxID := "nsPmyQbm4oo77jyykxbjf7s4Zp4urNptkyAouxVWZ2EB2kw1z"
	address := "costwo18atl0e95w5ym6t8u5yrjpz35vqqzxfzrrsnq8u"
	tx := database.PChainTxData{
		PChainTx: database.PChainTx{
			ChainID:    "costwo",
			Type:       database.PChainRewardAutoRenewedValidatorTx,
			TxID:       &rewardTxID,
			RewardTxID: addTxID,
			NodeID:     "NodeID-CZYx3on11wwYXFoHwZtAQZT5unZ9JHMf6",
			StartTime:  &startTime,
			EndTime:    &endTime,
			Weight:     1000,
			Period:     &period,
		},
		InputAddress: address,
		InputIndex:   0,
	}

	txs := map[int64][]database.PChainTxData{
		3: {tx},
	}

	txHash, err := staking.HashTransaction(&tx)
	require.NoError(t, err)

	contracts := testContracts{
		merkleRoots:           map[int64][32]byte{3: txHash},
		unregisteredAddresses: map[string]bool{address: true},
	}

	db, mirrored := testMirror(t, txs, contracts)

	require.Equal(t, uint64(4), db.states[mirrorStateName].NextDBIndex)
	require.Equal(t, map[string]int{addTxID: 1}, db.txRequests, "the address is looked up on the add tx")

	require.Len(t, mirrored.mirroredStakes, 1)
	stake := mirrored.mirroredStakes[0].stakeData
	expectedTxID, err := ids.FromString(rewardTxID)
	require.NoError(t, err)
	require.Equal(t, [32]byte(expectedTxID), stake.TxId, "mirrored under the reward tx id")
	require.Equal(t, uint8(0), stake.StakingType, "mirrored as a validator stake")
	require.Equal(t, uint64(startTime.Unix()), stake.StartTime)
	require.Equal(t, uint64(endTime.Unix()), stake.EndTime)
	require.Equal(t, uint64(1000), stake.Weight)
}
