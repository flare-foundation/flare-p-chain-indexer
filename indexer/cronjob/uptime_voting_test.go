//go:build integration
// +build integration

package cronjob

import (
	globalConfig "flare-indexer/config"
	"flare-indexer/database"
	"flare-indexer/indexer/config"
	"flare-indexer/indexer/context"
	"flare-indexer/indexer/pchain"
	"flare-indexer/indexer/shared"
	"flare-indexer/utils"
	"sort"
	"testing"
	"time"

	"github.com/ava-labs/libevm/common"
	"github.com/bradleyjkemp/cupaloy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func uptimeVotingCronjobTestConfig(epochStart time.Time) *config.Config {
	cfg := &config.Config{
		Chain: globalConfig.ChainConfig{
			ChainAddressHRP: "localflare",
			ChainID:         31337,
			EthRPCURL:       "http://127.0.0.1:8545",
			PrivateKey:      "0xd49743deccbccc5dc7baa8e69e5be03298da8688a15dd202e20f15d5e0e9a9fb",
		},
		UptimeCronjob: config.UptimeConfig{
			CronjobConfig: config.CronjobConfig{
				Enabled: true,
				Timeout: 30 * time.Second,
			},
			EpochConfig: globalConfig.EpochConfig{
				Start:  utils.Timestamp{Time: epochStart},
				Period: 90 * time.Second,
			},
			VotingInterval:  60 * time.Second,
			EnableVoting:    true,
			UptimeThreshold: 0.8,
		},
		VotingCronjob: config.VotingConfig{
			ContractAddress: common.HexToAddress("0x7c2C195CD6D34B8F845992d380aADB2730bB9C6F"),
		},
		PChainIndexer: config.IndexerConfig{
			Enabled:    true,
			Timeout:    3000 * time.Millisecond,
			BatchSize:  200,
			StartIndex: 0,
		},
		DB: globalConfig.DBConfig{
			Username:   database.MysqlTestUser,
			Password:   database.MysqlTestPassword,
			Host:       database.MysqlTestHost,
			Port:       database.MysqlTestPort,
			Database:   "flare_indexer_indexer",
			LogQueries: false,
		},
	}
	return cfg

}

func createTestUptimeVotingCronjob(epochStart time.Time) (*uptimeVotingCronjob, *shared.ChainIndexerBase, error) {
	ctx, err := context.BuildTestContext(uptimeVotingCronjobTestConfig(epochStart))
	if err != nil {
		return nil, nil, err
	}
	cronjob, err := NewUptimeVotingCronjob(ctx)
	if err != nil {
		return nil, nil, err
	}

	indexer := &shared.ChainIndexerBase{
		StateName:    pchain.StateName,
		IndexerName:  "P-chain Blocks Test",
		Client:       testClient,
		DB:           ctx.DB(),
		Config:       ctx.Config().PChainIndexer,
		BatchIndexer: pchain.NewPChainBatchIndexer(ctx, testClient, testRPCClient, nil),
	}
	return cronjob, indexer, nil
}

// Requires a running hardhat node
// from the flare-smart-contracts project, branch origin/staking-tests
// with yarn staking_test
func TestUptimeVoting(t *testing.T) {
	now := time.Unix(1675348249, 0)

	// Epoch starts "now"
	votingCronjob, indexer, err := createTestUptimeVotingCronjob(now)
	require.NoError(t, err)

	uptimeCronjob, err := createTestUptimeCronjob()
	require.NoError(t, err)

	// Run indexer to allow uptime client test to fetch validator data
	err = indexer.IndexBatch()
	require.NoError(t, err)

	testUptimeClient.SetNow(now)
	votingCronjob.time.SetNow(now)
	for i := 0; i < 10; i++ {
		if err := uptimeCronjob.Call(); err != nil {
			t.Fatal(err)
		}
		if err := votingCronjob.Call(); err != nil {
			t.Fatal(err)
		}
		testUptimeClient.Time.AdvanceNow(10 * time.Second)
		votingCronjob.time.AdvanceNow(10 * time.Second)
	}
	aggr, err := database.FetchAggregations(votingCronjob.db)
	require.NoError(t, err)
	assert.Equal(t, 4, len(aggr))

	// Sort by nodeID and compare to snapshots
	sort.Slice(aggr, func(i, j int) bool {
		return aggr[i].NodeID < aggr[j].NodeID
	})
	aggrNodeIDs := utils.Map(aggr, func(a *database.UptimeAggregation) string {
		return a.NodeID
	})
	aggrValue := utils.Map(aggr, func(a *database.UptimeAggregation) int64 {
		return a.Value
	})
	cupaloy.SnapshotT(t, aggrNodeIDs, aggrValue)
}
