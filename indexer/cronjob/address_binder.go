package cronjob

import (
	"flare-indexer/database"
	indexerctx "flare-indexer/indexer/context"
	"flare-indexer/logger"
	"flare-indexer/utils"
	"flare-indexer/utils/chain"
	"flare-indexer/utils/staking"
	"time"

	"github.com/ava-labs/avalanchego/utils/crypto"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/pkg/errors"
)

const addressBinderStateName = "address_binder_cronjob"

type addressBinderCronJob struct {
	epochCronjob
	db        addressBinderDB
	contracts addressBinderContracts
	time      utils.ShiftedTime
}

type addressBinderDB interface {
	FetchState(name string) (database.State, error)
	UpdateJobState(epoch int64, force bool) error
	GetPChainTxsForEpoch(start, end time.Time) ([]database.PChainTxData, error)
	GetPChainTx(txID string, address string) (*database.PChainTxData, error)
}

type addressBinderContracts interface {
	GetMerkleRoot(epoch int64) ([32]byte, error)
	IsAddressRegistered(address string) (bool, error)
	RegisterPublicKey(publicKey crypto.PublicKey) error
	EpochConfig() (time.Time, time.Duration, error)
}

func NewAddressBinderCronjob(ctx indexerctx.IndexerContext) (Cronjob, error) {
	cfg := ctx.Config()

	if !cfg.Mirror.Enabled {
		return &addressBinderCronJob{}, nil
	}

	contracts, err := initAddressBinderJobContracts(cfg)
	if err != nil {
		return nil, err
	}

	start, period, err := contracts.EpochConfig()
	if err != nil {
		return nil, err
	}

	epochs := staking.NewEpochInfo(&cfg.Mirror.EpochConfig, start, period)

	mc := &addressBinderCronJob{
		epochCronjob: newEpochCronjob(&cfg.Mirror.CronjobConfig, epochs),
		db:           NewAddressBinderDBGorm(ctx.DB()),
		contracts:    contracts,
	}

	err = mc.reset(ctx.Flags().ResetMirrorCronjob)

	return mc, err
}

func (c *addressBinderCronJob) Name() string {
	return "address_binder"
}

func (c *addressBinderCronJob) OnStart() error {
	return nil
}

func (c *addressBinderCronJob) Call() error {
	epochRange, err := c.getEpochRange()
	if err != nil {
		if errors.Is(err, errNoEpochsToRegisterAddresses) {
			logger.Debug("no epochs to register addresses")
			return nil
		}

		return err
	}

	logger.Debug("registering addresses for epochs %d-%d", epochRange.start, epochRange.end)
	registered := mapset.NewSet[string]()
	for epoch := epochRange.start; epoch <= epochRange.end; epoch++ {
		logger.Debug("registering addresses for epoch %d", epoch)
		if err := c.registerEpoch(registered, epoch); err != nil {
			return err
		}
	}

	logger.Debug("successfully registered addresses for epochs %d-%d", epochRange.start, epochRange.end)

	if err := c.db.UpdateJobState(epochRange.end+1, false); err != nil {
		return err
	}

	return nil
}

var errNoEpochsToRegisterAddresses = errors.New("no epochs to register addresses")

func (c *addressBinderCronJob) getEpochRange() (*epochRange, error) {
	startEpoch, err := c.getStartEpoch()
	if err != nil {
		return nil, err
	}

	logger.Debug("start epoch: %d", startEpoch)

	endEpoch, err := c.getEndEpoch(startEpoch)
	if err != nil {
		return nil, err
	}
	logger.Debug("Registering addresses needed for epochs [%d, %d]", startEpoch, endEpoch)
	return c.getTrimmedEpochRange(startEpoch, endEpoch), nil
}

func (c *addressBinderCronJob) getStartEpoch() (int64, error) {
	jobState, err := c.db.FetchState(addressBinderStateName)
	if err != nil {
		return 0, err
	}

	return int64(jobState.NextDBIndex), nil
}

func (c *addressBinderCronJob) getEndEpoch(startEpoch int64) (int64, error) {
	currEpoch := c.epochs.GetEpochIndex(c.time.Now())
	logger.Debug("current epoch: %d", currEpoch)

	for epoch := currEpoch; epoch > startEpoch; epoch-- {
		confirmed, err := c.isEpochConfirmed(epoch)
		if err != nil {
			return 0, err
		}

		if confirmed {
			return epoch, nil
		}
	}

	return 0, errNoEpochsToRegisterAddresses
}

func (c *addressBinderCronJob) isEpochConfirmed(epoch int64) (bool, error) {
	merkleRoot, err := c.contracts.GetMerkleRoot(epoch)
	if err != nil {
		return false, errors.Wrap(err, "votingContract.GetMerkleRoot")
	}

	return merkleRoot != [32]byte{}, nil
}

func (c *addressBinderCronJob) registerEpoch(registered mapset.Set[string], epoch int64) error {
	txs, err := c.getEpochTxs(epoch)
	if err != nil {
		return err
	}

	if len(txs) == 0 {
		logger.Debug("no unregistered txs found")
		return nil
	}

	logger.Info("registering %d txs", len(txs))
	if err := c.registerTxs(registered, txs, epoch); err != nil {
		return err
	}

	return nil
}

func (c *addressBinderCronJob) getEpochTxs(epoch int64) ([]database.PChainTxData, error) {
	startTimestamp, endTimestamp := c.epochs.GetTimeRange(epoch)

	txs, err := c.db.GetPChainTxsForEpoch(startTimestamp, endTimestamp)
	if err != nil {
		return nil, err
	}

	return staking.DedupeTxs(txs), nil
}

func (c *addressBinderCronJob) registerTxs(registered mapset.Set[string], txs []database.PChainTxData, epochID int64) error {
	for _, tx := range txs {
		if registered.Contains(tx.InputAddress) {
			continue
		}
		if registered, err := c.registerAddress(*tx.TxID, tx.InputAddress); err != nil {
			logger.Error("error registering address: %s", err.Error())
		} else if registered {
			logger.Info("registered address %s on address binder contract", tx.InputAddress)
		}
		registered.Add(tx.InputAddress)
	}
	return nil
}

// Return false if the address is already registered, true if it was registered successfully
func (c *addressBinderCronJob) registerAddress(txID string, address string) (bool, error) {
	registered, err := c.contracts.IsAddressRegistered(address)
	if err != nil || registered {
		return false, err
	}
	tx, err := c.db.GetPChainTx(txID, address)
	if err != nil {
		return false, err
	}
	if tx == nil {
		return false, errors.New("tx not found")
	}
	publicKeys, err := chain.PublicKeysFromPChainBlock(tx.Bytes)
	if err != nil {
		return false, err
	}
	if tx.InputIndex >= uint32(len(publicKeys)) {
		return false, errors.New("input index out of range")
	}
	publicKey := publicKeys[tx.InputIndex]
	for _, k := range publicKey {
		err := c.contracts.RegisterPublicKey(k)
		if err != nil {
			return false, errors.Wrap(err, "mirroringContract.RegisterPublicKey")
		}
	}
	return true, nil
}

func (c *addressBinderCronJob) reset(firstEpoch int64) error {
	if firstEpoch <= 0 {
		return nil
	}

	logger.Info("Resetting address binder cronjob state to epoch %d", firstEpoch)
	err := c.db.UpdateJobState(firstEpoch, true)
	if err != nil {
		return err
	}
	c.epochs.First = firstEpoch
	return nil
}
