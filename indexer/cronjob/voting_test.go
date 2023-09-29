package cronjob

import (
	"flare-indexer/database"
	"flare-indexer/indexer/config"
	"flare-indexer/indexer/pchain"
	"flare-indexer/utils/staking"
	"math/big"
	"testing"
	"time"

	"github.com/bradleyjkemp/cupaloy"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

type votingDBTest struct {
	states     map[string]database.State
	votingData map[timeRange][]database.PChainTxData
}

type timeRange struct {
	start time.Time
	end   time.Time
}

func (db *votingDBTest) FetchState(name string) (database.State, error) {
	state, ok := db.states[name]
	if ok {
		return state, nil
	}

	return database.State{Name: name}, nil
}

func (db *votingDBTest) FetchPChainVotingData(start, end time.Time) ([]database.PChainTxData, error) {
	return db.votingData[timeRange{start, end}], nil
}

func (db *votingDBTest) UpdateState(epoch int64, force bool) error {
	if state, ok := db.states[votingStateName]; ok {
		if !force && state.NextDBIndex >= uint64(epoch) {
			return nil
		}
	}
	db.states[votingStateName] = database.State{
		Name:        votingStateName,
		NextDBIndex: uint64(epoch),
	}
	return nil
}

type votingContractTest struct {
	shouldVote     map[int64]bool
	submittedVotes map[int64][32]byte
}

func (c *votingContractTest) ShouldVote(epoch *big.Int) (bool, error) {
	return c.shouldVote[epoch.Int64()], nil
}

func (c *votingContractTest) SubmitVote(epoch *big.Int, merkleRoot [32]byte) error {
	epochInt := epoch.Int64()

	if _, ok := c.submittedVotes[epochInt]; ok {
		return errors.New("already submitted vote")
	}

	c.submittedVotes[epochInt] = merkleRoot
	c.shouldVote[epochInt] = false
	return nil
}

func (c *votingContractTest) EpochConfig() (time.Time, time.Duration, error) {
	return time.Now(), 180 * time.Second, nil
}

func TestNoVotes(t *testing.T) {
	db := &votingDBTest{}
	contract := &votingContractTest{}
	cronjob := &votingCronjob{
		db:           db,
		contract:     contract,
		epochCronjob: initEpochCronjob(),
	}

	err := cronjob.Call()
	require.NoError(t, err)
	require.Empty(t, contract.submittedVotes)
}

func TestVotes(t *testing.T) {
	epochs := initEpochCronjob()

	db := votingDBTest{
		states: map[string]database.State{
			pchain.StateName: {
				Updated:        time.Now(),
				NextDBIndex:    3,
				LastChainIndex: 2,
			},
		},
		votingData: map[timeRange][]database.PChainTxData{
			timeRangeForEpoch(epochs, 1): {newTxData(0)},
			timeRangeForEpoch(epochs, 2): {newTxData(1), newTxData(2)},
		},
	}

	contract := votingContractTest{
		shouldVote: map[int64]bool{
			1: true,
			2: true,
		},
		submittedVotes: make(map[int64][32]byte),
	}

	cronjob := votingCronjob{
		db:           &db,
		contract:     &contract,
		epochCronjob: epochs,
	}

	err := cronjob.Call()
	require.NoError(t, err)
	require.NotEmpty(t, contract.submittedVotes)

	cupaloy.SnapshotT(t, contract.submittedVotes)

	updatedState := db.states[votingStateName]
	require.Equal(t, updatedState.NextDBIndex, uint64(1))

	err = cronjob.Call()
	require.NoError(t, err)

	updatedState = db.states[votingStateName]
	require.Equal(t, updatedState.NextDBIndex, uint64(6))
}

func timeRangeForEpoch(cj epochCronjob, epoch int64) timeRange {
	start, end := cj.epochs.GetTimeRange(epoch)

	return timeRange{start, end}
}

var txIDs = []string{
	"XnfV79XVMyuXbTw8iNreQ9FrUgy9csYBJp1xRscay3oDzhyq8",
	"nsPmyQbm4oo77jyykxbjf7s4Zp4urNptkyAouxVWZ2EB2kw1z",
	"2p32tpqNrfzP3SStbP9bQGHZtJkCxjV3iHNssVnkcpUWxHMSuj",
}

func newTxData(id int) database.PChainTxData {
	startTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)

	return database.PChainTxData{
		PChainTx: database.PChainTx{
			ChainID:   "costwo",
			NodeID:    "NodeID-CZYx3on11wwYXFoHwZtAQZT5unZ9JHMf6",
			StartTime: &startTime,
			EndTime:   &endTime,
			TxID:      &txIDs[id],
			Type:      database.PChainAddDelegatorTx,
		},
		InputAddress: "costwo18atl0e95w5ym6t8u5yrjpz35vqqzxfzrrsnq8u",
	}
}

func initEpochCronjob() epochCronjob {
	cronjobCfg := config.CronjobConfig{
		Enabled:   true,
		Timeout:   180,
		BatchSize: 5,
	}

	epochInfo := staking.EpochInfo{
		Period: 180 * time.Second,
		Start:  time.Now().Add(-time.Hour),
	}

	return newEpochCronjob(&cronjobCfg, epochInfo)
}
