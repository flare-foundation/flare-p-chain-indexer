// Stubs for the address binder cronjob. These handle the direct interactions with DB
// and contracts. The actual logic is in address_binder.go, which is unit-tested.
package cronjob

import (
	"flare-indexer/database"
	"flare-indexer/indexer/config"
	"flare-indexer/logger"
	"flare-indexer/utils/chain"
	"flare-indexer/utils/contracts/addresses"
	"flare-indexer/utils/contracts/mirroring"
	"flare-indexer/utils/contracts/voting"
	"flare-indexer/utils/staking"
	"math/big"
	"time"

	"github.com/ava-labs/avalanchego/utils/crypto"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

type addressBinderDBGorm struct {
	db *gorm.DB
}

func NewAddressBinderDBGorm(db *gorm.DB) addressBinderDB {
	return addressBinderDBGorm{db: db}
}

func (m addressBinderDBGorm) FetchState(name string) (database.State, error) {
	return database.FetchState(m.db, name)
}

func (m addressBinderDBGorm) UpdateJobState(epoch int64, force bool) error {
	return m.db.Transaction(func(tx *gorm.DB) error {
		jobState, err := database.FetchState(tx, addressBinderStateName)
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

func (m addressBinderDBGorm) GetPChainTxsForEpoch(start, end time.Time) ([]database.PChainTxData, error) {
	return database.GetPChainTxsForEpoch(&database.GetPChainTxsForEpochInput{
		DB:             m.db,
		StartTimestamp: start,
		EndTimestamp:   end,
	})
}

func (m addressBinderDBGorm) GetPChainTx(txID string, address string) (*database.PChainTxData, error) {
	return database.FetchPChainTxData(m.db, txID, address)
}

type addressBinderContractsCChain struct {
	addressBinder *addresses.Binder
	txOpts        *bind.TransactOpts
	voting        *voting.Voting
}

func initAddressBinderJobContracts(cfg *config.Config) (addressBinderContracts, error) {
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

	addressBinderContract, err := newAddressBinderContract(eth, mirroringContract)
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

	return &addressBinderContractsCChain{
		addressBinder: addressBinderContract,
		txOpts:        txOpts,
		voting:        votingContract,
	}, nil
}

func newAddressBinderContract(
	eth *ethclient.Client, mirroringContract *mirroring.Mirroring,
) (*addresses.Binder, error) {
	addressBinderAddress, err := mirroringContract.AddressBinder(new(bind.CallOpts))
	if err != nil {
		return nil, err
	}

	return addresses.NewBinder(addressBinderAddress, eth)
}

func (m addressBinderContractsCChain) GetMerkleRoot(epoch int64) ([32]byte, error) {
	return m.voting.GetMerkleRoot(new(bind.CallOpts), big.NewInt(epoch))
}

func (m addressBinderContractsCChain) IsAddressRegistered(address string) (bool, error) {
	addressBytes, err := chain.ParseAddress(address)
	if err != nil {
		return false, err
	}
	boundAddress, err := m.addressBinder.PAddressToCAddress(new(bind.CallOpts), addressBytes)
	if err != nil {
		return false, err
	}
	return boundAddress != (common.Address{}), nil
}

func (m addressBinderContractsCChain) RegisterPublicKey(publicKey crypto.PublicKey) error {
	ethAddress, err := chain.PublicKeyToEthAddress(publicKey)
	if err != nil {
		return err
	}
	_, err = m.addressBinder.RegisterAddresses(m.txOpts, publicKey.Bytes(), publicKey.Address(), ethAddress)
	return err
}

func (m addressBinderContractsCChain) EpochConfig() (start time.Time, period time.Duration, err error) {
	return staking.GetEpochConfig(m.voting)
}
