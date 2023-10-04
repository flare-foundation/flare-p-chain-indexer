package cronjob

import (
	"flare-indexer/database"
	"flare-indexer/indexer/config"
	"flare-indexer/utils/contracts/voting"
	"flare-indexer/utils/staking"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/ethclient"
	"gorm.io/gorm"
)

type votingDBGorm struct {
	g *gorm.DB
}

func (db *votingDBGorm) FetchState(name string) (database.State, error) {
	return database.FetchState(db.g, name)
}

func (db *votingDBGorm) FetchPChainVotingData(start, end time.Time) ([]database.PChainTxData, error) {
	return database.FetchPChainVotingData(db.g, start, end)
}

func (db *votingDBGorm) UpdateState(state *database.State) error {
	return database.UpdateState(db.g, state)
}

type votingContractCChain struct {
	callOpts *bind.CallOpts
	txOpts   *bind.TransactOpts
	voting   *voting.Voting
}

func newVotingContractCChain(cfg *config.Config) (votingContract, error) {
	votingContract, err := newVotingContract(cfg)
	if err != nil {
		return nil, err
	}

	privateKey, err := cfg.Chain.GetPrivateKey()
	if err != nil {
		return nil, err
	}

	txOpts, err := TransactOptsFromPrivateKey(privateKey, cfg.Chain.ChainID)
	if err != nil {
		return nil, err
	}
	txOpts.GasLimit = cfg.VotingCronjob.GasLimit

	callOpts := &bind.CallOpts{From: txOpts.From}

	return &votingContractCChain{
		callOpts: callOpts,
		txOpts:   txOpts,
		voting:   votingContract,
	}, nil
}

func newVotingContract(cfg *config.Config) (*voting.Voting, error) {
	eth, err := ethclient.Dial(cfg.Chain.EthRPCURL)
	if err != nil {
		return nil, err
	}
	return voting.NewVoting(cfg.ContractAddresses.Voting, eth)
}

func (c *votingContractCChain) ShouldVote(epoch *big.Int) (bool, error) {
	return c.voting.ShouldVote(c.callOpts, epoch, c.callOpts.From)
}

func (c *votingContractCChain) SubmitVote(epoch *big.Int, merkleRoot [32]byte) error {
	_, err := c.voting.SubmitVote(c.txOpts, epoch, merkleRoot)
	return err
}

func (c *votingContractCChain) EpochConfig() (start time.Time, period time.Duration, err error) {
	return staking.GetEpochConfig(c.voting)
}
