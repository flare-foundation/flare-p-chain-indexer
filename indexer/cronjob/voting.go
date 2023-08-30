package cronjob

import (
	"context"
	"flare-indexer/database"
	"flare-indexer/indexer/config"
	idxCtx "flare-indexer/indexer/context"
	"flare-indexer/indexer/pchain"
	"flare-indexer/utils/contracts/voting"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"gorm.io/gorm"
)

const (
	VotingStateName string = "voting_cronjob"
)

var (
	zeroBytes     [32]byte    = [32]byte{}
	zeroBytesHash common.Hash = crypto.Keccak256Hash(zeroBytes[:])
)

type votingCronjob struct {
	enabled bool
	epochs  epochInfo
	timeout int

	// Lock to prevent concurrent aggregation
	running bool

	db             *gorm.DB
	votingContract *voting.Voting
	txOpts         *bind.TransactOpts
}

func NewVotingCronjob(ctx idxCtx.IndexerContext) (Cronjob, error) {
	cfg := ctx.Config()

	if !cfg.VotingCronjob.Enabled {
		return &votingCronjob{}, nil
	}

	votingContract, err := newVotingContract(cfg)
	if err != nil {
		return nil, err
	}
	txOpts, err := TransactOptsFromPrivateKey(cfg.Chain.PrivateKey, cfg.Chain.ChainID)
	if err != nil {
		return nil, err
	}

	return &votingCronjob{
		enabled:        cfg.VotingCronjob.Enabled,
		timeout:        cfg.VotingCronjob.TimeoutSeconds,
		running:        false,
		db:             ctx.DB(),
		epochs:         newEpochInfo(&cfg.Epochs),
		votingContract: votingContract,
		txOpts:         txOpts,
	}, nil
}

func newVotingContract(cfg *config.Config) (*voting.Voting, error) {
	eth, err := ethclient.Dial(cfg.Chain.EthRPCURL)
	if err != nil {
		return nil, err
	}
	return voting.NewVoting(cfg.VotingCronjob.ContractAddress, eth)
}

func (c *votingCronjob) Name() string {
	return "mirror"
}

func (c *votingCronjob) Enabled() bool {
	return c.enabled
}

func (c *votingCronjob) TimeoutSeconds() int {
	return c.timeout
}

func (c *votingCronjob) Call() error {
	idxState, err := database.FetchState(c.db, pchain.StateName)
	if err != nil {
		return err
	}
	state, err := database.FetchState(c.db, VotingStateName)
	if err != nil {
		return err
	}
	now := time.Now()

	// Last epoch that was submitted to the contract
	nextEpochToSubmit := state.NextDBIndex
	lastEpochToSubmit := c.epochs.getEpochIndex(now) - 1
	for e := int64(nextEpochToSubmit); e <= lastEpochToSubmit; e++ {
		start, end := c.epochs.getTimeRange(e)

		if end.After(idxState.Updated) {
			break
		}

		votingData, err := database.FetchPChainVotingData(c.db, start, end)
		if err != nil {
			return err
		}
		err = c.submitVotes(e, votingData)
		if err != nil {
			return err
		}

		// Update state
		state.NextDBIndex = uint64(e + 1)
		if err := database.UpdateState(c.db, &state); err != nil {
			return err
		}
	}
	return nil
}

func (c *votingCronjob) submitVotes(e int64, votingData []database.PChainTxData) error {
	votingData = dedupeTxs(votingData)
	callOpts := &bind.CallOpts{
		From:    c.txOpts.From,
		Context: context.Background(),
	}

	shouldVote, err := c.votingContract.ShouldVote(callOpts, big.NewInt(e), c.txOpts.From)
	if err != nil {
		return err
	}
	if !shouldVote {
		return nil
	}

	var merkleRoot common.Hash
	if len(votingData) == 0 {
		merkleRoot = zeroBytesHash
	} else {
		merkleRoot, err = getMerkleRoot(votingData)
		if err != nil {
			return err
		}
	}
	_, err = c.votingContract.SubmitVote(c.txOpts, big.NewInt(e), [32]byte(merkleRoot))
	return err
}
