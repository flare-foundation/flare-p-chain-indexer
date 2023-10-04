// Stubs for the mirror cronjob. These handle the direct interactions with DB
// and contracts. The actual logic is in mirror.go, which is unit-tested.
package cronjob

import (
	"flare-indexer/database"
	"flare-indexer/indexer/config"
	"flare-indexer/logger"
	"flare-indexer/utils/contracts/mirroring"
	"flare-indexer/utils/contracts/voting"
	"flare-indexer/utils/staking"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

type mirrorDBGorm struct {
	db *gorm.DB
}

func NewMirrorDBGorm(db *gorm.DB) mirrorDB {
	return mirrorDBGorm{db: db}
}

func (m mirrorDBGorm) FetchState(name string) (database.State, error) {
	return database.FetchState(m.db, name)
}

func (m mirrorDBGorm) UpdateJobState(epoch int64, force bool) error {
	return m.db.Transaction(func(tx *gorm.DB) error {
		jobState, err := database.FetchState(tx, mirrorStateName)
		if err != nil {
			return errors.Wrap(err, "database.FetchState")
		}

		if !force && jobState.NextDBIndex >= uint64(epoch) {
			logger.Debug("job state already up to date")
			return nil
		}

		jobState.NextDBIndex = uint64(epoch)

		return database.UpdateState(tx, &jobState)
	})
}

func (m mirrorDBGorm) GetPChainTxsForEpoch(start, end time.Time) ([]database.PChainTxData, error) {
	return database.GetPChainTxsForEpoch(&database.GetPChainTxsForEpochInput{
		DB:             m.db,
		StartTimestamp: start,
		EndTimestamp:   end,
	})
}

func (m mirrorDBGorm) GetPChainTx(txID string, address string) (*database.PChainTxData, error) {
	return database.FetchPChainTxData(m.db, txID, address)
}

type mirrorContractsCChain struct {
	mirroring *mirroring.Mirroring
	txOpts    *bind.TransactOpts
	voting    *voting.Voting
}

func initMirrorJobContracts(cfg *config.Config) (mirrorContracts, error) {
	if cfg.ContractAddresses.Mirroring == (common.Address{}) {
		return nil, errors.New("mirroring contract address not set")
	}

	if cfg.ContractAddresses.Voting == (common.Address{}) {
		return nil, errors.New("voting contract address not set")
	}

	eth, err := ethclient.Dial(cfg.Chain.EthRPCURL)
	if err != nil {
		return nil, err
	}

	mirroringContract, err := mirroring.NewMirroring(cfg.ContractAddresses.Mirroring, eth)
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

	return &mirrorContractsCChain{
		mirroring: mirroringContract,
		txOpts:    txOpts,
		voting:    votingContract,
	}, nil
}

func (m mirrorContractsCChain) GetMerkleRoot(epoch int64) ([32]byte, error) {
	return m.voting.GetMerkleRoot(new(bind.CallOpts), big.NewInt(epoch))
}

func (m mirrorContractsCChain) MirrorStake(
	stakeData *mirroring.IPChainStakeMirrorVerifierPChainStake,
	merkleProof [][32]byte,
) error {
	_, err := m.mirroring.MirrorStake(m.txOpts, *stakeData, merkleProof)
	return err
}

func (m mirrorContractsCChain) EpochConfig() (start time.Time, period time.Duration, err error) {
	return staking.GetEpochConfig(m.voting)
}
