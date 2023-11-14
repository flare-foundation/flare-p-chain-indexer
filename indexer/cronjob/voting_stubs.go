package cronjob

import (
	"flare-indexer/database"
	"flare-indexer/indexer/config"
	"flare-indexer/logger"
	"flare-indexer/utils/chain"
	"flare-indexer/utils/contracts/voting"
	"flare-indexer/utils/staking"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
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
	callOpts   *bind.CallOpts
	txOpts     *bind.TransactOpts
	voting     *voting.Voting
	txVerifier *chain.TxVerifier
}

func newVotingContractCChain(cfg *config.Config) (votingContract, error) {
	eth, err := cfg.Chain.DialETH()
	if err != nil {
		return nil, err
	}

	votingContract, err := voting.NewVoting(cfg.ContractAddresses.Voting, eth)
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
		callOpts:   callOpts,
		txOpts:     txOpts,
		voting:     votingContract,
		txVerifier: chain.NewTxVerifier(eth),
	}, nil
}

func (c *votingContractCChain) ShouldVote(epoch *big.Int) (bool, error) {
	return c.voting.ShouldVote(c.callOpts, epoch, c.callOpts.From)
}

func (c *votingContractCChain) SubmitVote(epoch *big.Int, merkleRoot [32]byte) error {
	tx, err := c.voting.SubmitVote(c.txOpts, epoch, merkleRoot)
	if err != nil {
		return err
	}
	err = c.txVerifier.WaitUntilMined(c.callOpts.From, tx, chain.DefaultTxTimeout)
	if err != nil {
		if strings.Contains(err.Error(), "epoch already finalized") {
			logger.Info("Epoch %s already finalized", epoch.String())
			return nil
		}
		return err
	}
	logger.Debug("Mined voting tx %s", tx.Hash().Hex())
	return nil
}

func (c *votingContractCChain) EpochConfig() (start time.Time, period time.Duration, err error) {
	return staking.GetEpochConfig(c.voting)
}
