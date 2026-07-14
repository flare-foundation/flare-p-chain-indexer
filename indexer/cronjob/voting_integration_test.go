//go:build integration
// +build integration

package cronjob

import (
	sysContext "context"
	globalConfig "flare-indexer/config"
	"flare-indexer/database"
	"flare-indexer/indexer/config"
	"flare-indexer/indexer/context"
	"flare-indexer/indexer/pchain"
	"flare-indexer/indexer/shared"
	"flare-indexer/utils"
	"flare-indexer/utils/contracts/voting"
	"math/big"
	"testing"
	"time"

	"github.com/ava-labs/libevm/accounts/abi/bind"
	"github.com/ava-labs/libevm/common"
	"github.com/bradleyjkemp/cupaloy"
	"github.com/stretchr/testify/require"
)

func votingCronjobTestConfig(epochStart time.Time, dbName string, privateKey string) *config.Config {
	cfg := &config.Config{
		Chain: globalConfig.ChainConfig{
			ChainAddressHRP: "localflare",
			ChainID:         31337,
			EthRPCURL:       "http://127.0.0.1:8545",
			PrivateKey:      privateKey,
		},
		VotingCronjob: config.VotingConfig{
			CronjobConfig: config.CronjobConfig{
				Enabled: true,
				Timeout: 30 * time.Second,
			},
			ContractAddress: common.HexToAddress("0x7c2C195CD6D34B8F845992d380aADB2730bB9C6F"),
		},
		Mirror: config.MirrorConfig{
			CronjobConfig: config.CronjobConfig{
				Enabled: true,
				Timeout: 30 * time.Second,
			},
			MirroringContract: common.HexToAddress("0x8858eeB3DfffA017D4BCE9801D340D36Cf895CCf"),
		},
		Epochs: config.EpochConfig{
			Start:  utils.Timestamp{Time: epochStart},
			Period: 90 * time.Second,
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
			Database:   dbName,
			LogQueries: false,
		},
		Logger: globalConfig.LoggerConfig{
			Level: "debug",
		},
	}
	return cfg
}

// Transform p-chain txs before persisting:
// Extend end time of a validator tx past test start time to prevent mirror contract to fail
func transformPChainTx(tx *database.PChainTx) *database.PChainTx {
	if tx.Type == database.PChainAddValidatorTx || tx.Type == database.PChainAddDelegatorTx {
		minEndTime := time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)
		tx.EndTime = &minEndTime
	}
	return tx
}

func createTestVotingClients(epochStart time.Time) (*votingCronjob, *votingCronjob, *mirrorCronJob, *shared.ChainIndexerBase, *shared.ChainIndexerBase, error) {
	ctx1, err := context.BuildTestContext(votingCronjobTestConfig(epochStart, "flare_indexer_indexer", privateKey1))
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	cronjob1, err := NewVotingCronjob(ctx1)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	ctx2, err := context.BuildTestContext(votingCronjobTestConfig(epochStart, "flare_indexer_indexer_2", privateKey2))
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	cronjob2, err := NewVotingCronjob(ctx2)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	mirror, err := NewMirrorCronjob(ctx1)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	indexer1 := &shared.ChainIndexerBase{
		StateName:   pchain.StateName,
		IndexerName: "P-chain Blocks Test",
		Client:      testClient,
		DB:          ctx1.DB(),
		Config:      ctx1.Config().PChainIndexer,
		BatchIndexer: pchain.NewPChainBatchIndexer(
			ctx1, testClient, testRPCClient,
			pchain.NewPChainDataTransformer(transformPChainTx),
		),
	}
	indexer2 := &shared.ChainIndexerBase{
		StateName:   pchain.StateName,
		IndexerName: "P-chain Blocks Test",
		Client:      testClient,
		DB:          ctx2.DB(),
		Config:      ctx2.Config().PChainIndexer,
		BatchIndexer: pchain.NewPChainBatchIndexer(
			ctx1, testClient, testRPCClient,
			pchain.NewPChainDataTransformer(transformPChainTx),
		),
	}
	return cronjob1, cronjob2, mirror.(*mirrorCronJob), indexer1, indexer2, nil
}

func getMerkleRootFromContract(votingContract *voting.Voting, epoch int64) ([32]byte, error) {
	ctx := sysContext.Background()
	opts := &bind.CallOpts{Context: ctx}
	merkleRoot, err := votingContract.GetMerkleRoot(opts, big.NewInt(epoch))
	if err != nil {
		return [32]byte{}, err
	}
	return merkleRoot, nil
}

func TestVoting(t *testing.T) {
	now := time.Unix(1675349340, 0) // 2023-02-02 14:49:00 UTC
	vCronjob1, vCronjob2, mCronjob, indexer1, indexer2, err := createTestVotingClients(now)
	require.NoError(t, err)

	// Run indexer to allow voting client test to fetch validator data
	// We need two indexers, each one for a different voting client,
	// since the progress is stored in the DB
	t.Run("Run indexer 1", func(t *testing.T) {
		err := indexer1.IndexBatch()
		require.NoError(t, err)
	})
	t.Run("Run indexer 2", func(t *testing.T) {
		err := indexer2.IndexBatch()
		require.NoError(t, err)
	})

	t.Run("Run voting clients 1 and 2", func(t *testing.T) {
		vCronjob1.time.SetNow(now)
		vCronjob2.time.SetNow(now)
		for i := 0; i < 10; i++ {
			err := vCronjob1.Call()
			require.NoError(t, err)
			err = vCronjob2.Call()
			require.NoError(t, err)
			vCronjob1.time.AdvanceNow(30 * time.Second)
			vCronjob2.time.AdvanceNow(30 * time.Second)
		}
	})
	t.Run("Verify merkle root", func(t *testing.T) {
		root, err := getMerkleRootFromContract(vCronjob1.votingContract, 0)
		require.NoError(t, err)
		cupaloy.SnapshotT(t, root)
	})
	t.Run("Run mirroring client", func(t *testing.T) {
		mCronjob.time.SetNow(now)
		mCronjob.time.AdvanceNow(10 * 30 * time.Second)
		err := mCronjob.Call()
		require.NoError(t, err)
	})
}
