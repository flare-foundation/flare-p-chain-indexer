package pchain

import (
	globalConfig "flare-indexer/config"
	"flare-indexer/database"
	"flare-indexer/indexer/config"
	"flare-indexer/utils/chain"
	"log"
	"testing"
)

var (
	testClient    *chain.RecordedIndexerClient //:= chain.PChainTestClient(t)
	testRPCClient *chain.RecordedRPCClient     //:= chain.PChainTestRPCClient(t)
)

func TestMain(m *testing.M) {
	var err error
	testClient, err = chain.PChainTestClient()
	if err != nil {
		log.Fatal(err)
	}

	testRPCClient, err = chain.PChainTestRPCClient()
	if err != nil {
		log.Fatal(err)
	}
	m.Run()
}

func pchainIndexerTestConfig(batchSize int, startIndex uint64) *config.Config {
	cfg := &config.Config{
		Chain: globalConfig.ChainConfig{
			ChainAddressHRP: "localflare",
			ChainID:         162,
		},
		PChainIndexer: config.IndexerConfig{
			Enabled:       true,
			TimeoutMillis: 3000,
			BatchSize:     batchSize,
			StartIndex:    startIndex,
		},
		UptimeCronjob: config.UptimeConfig{
			CronjobConfig: config.CronjobConfig{
				Enabled:        true,
				TimeoutSeconds: 60,
			},
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
