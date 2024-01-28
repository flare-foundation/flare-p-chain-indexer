package shared

import (
	"flare-indexer/database"
	"flare-indexer/indexer/config"
	"flare-indexer/logger"
	"flare-indexer/utils/chain"
	"time"

	"github.com/ava-labs/avalanchego/indexer"
	"gorm.io/gorm"
)

type ContainerBatchIndexer interface {
	Reset(containerLen int) error
	AddContainer(index uint64, container indexer.Container) error
	ProcessBatch() error
	PersistEntities(db *gorm.DB) error
}

type ChainIndexerBase struct {
	StateName   string
	IndexerName string

	DB     *gorm.DB
	Client chain.IndexerClient
	Config config.IndexerConfig

	BatchIndexer ContainerBatchIndexer

	metrics *metrics
}

func (ci *ChainIndexerBase) IndexBatch() error {
	startTime := time.Now()

	// Get current state of tx indexer from db
	currentState, err := database.FetchState(ci.DB, ci.StateName)
	if err != nil {
		return err
	}

	var nextIndex uint64
	if currentState.NextDBIndex < ci.Config.StartIndex {
		nextIndex = ci.Config.StartIndex
	} else {
		nextIndex = currentState.NextDBIndex
	}

	// Fetch last accepted index on chain
	_, lastIndex, err := chain.FetchLastAcceptedContainer(ci.Client)
	if err != nil {
		return err
	}
	if lastIndex < nextIndex {
		// Nothing to do; no new containers
		logger.Debug("Nothing to do. Last index %d < next to process %d", lastIndex, nextIndex)

		// Update time of last run (for other clients to know that the indexer is running)
		currentState.UpdateTime()
		if err := database.UpdateState(ci.DB, &currentState); err != nil {
			return err
		}

		if ci.metrics != nil {
			duration := time.Since(startTime).Milliseconds()
			ci.metrics.Update(currentState.LastChainIndex, currentState.NextDBIndex-1, duration)
		}
		return nil
	}

	// Get MaxBatch containers from the chain
	containers, err := chain.FetchContainerRangeFromIndexer(ci.Client, nextIndex, ci.Config.BatchSize)
	if err != nil {
		return err
	}

	lastProcessedIndex, err := ci.ProcessContainers(nextIndex, containers)
	if err != nil {
		return err
	}

	err = database.DoInTransaction(ci.DB,
		func(db *gorm.DB) error { return ci.BatchIndexer.PersistEntities(db) },
		func(db *gorm.DB) error {
			currentState.Update(lastProcessedIndex+1, lastIndex)
			return database.UpdateState(db, &currentState)
		},
	)
	if err != nil {
		return err
	}
	duration := time.Since(startTime).Milliseconds()
	logger.Info("Indexer '%s' processed to index %d, last accepted index is %d, duration %dms",
		ci.IndexerName,
		lastProcessedIndex, lastIndex, duration)

	if ci.metrics != nil {
		ci.metrics.Update(lastIndex, lastProcessedIndex, duration)
	}

	return nil
}

func (ci *ChainIndexerBase) ProcessContainers(nextIndex uint64, containers []indexer.Container) (uint64, error) {
	err := ci.BatchIndexer.Reset(len(containers))
	if err != nil {
		return 0, err
	}

	var index uint64
	for i, container := range containers {
		index = nextIndex + uint64(i)

		err := ci.BatchIndexer.AddContainer(index, container)
		if err != nil {
			return 0, err
		}
	}

	err = ci.BatchIndexer.ProcessBatch()
	if err != nil {
		return 0, err
	}

	return index, nil
}

func (ci *ChainIndexerBase) Run() {
	if !ci.Config.Enabled {
		logger.Debug("%s indexer is disabled", ci.IndexerName)
		ci.SetStatus(HealthStatusOk)
		return
	}

	ticker := time.NewTicker(ci.Config.Timeout)
	for range ticker.C {
		err := ci.IndexBatch()
		if err != nil {
			logger.Error("%s indexer error %v", ci.IndexerName, err)
			ci.SetStatus(HealthStatusError)
		}
	}
}

func (ci *ChainIndexerBase) InitMetrics(namespace string) {
	ci.metrics = newMetrics(namespace)
}

func (ci *ChainIndexerBase) SetStatus(status HealthStatus) {
	if ci.metrics != nil {
		ci.metrics.SetStatus(status)
	}
}
