package cronjob

import (
	"flare-indexer/database"
	"flare-indexer/logger"
	"sort"
	"time"

	"gorm.io/gorm"
)

type UptimeAggregator struct {
	// epoch start timestamp (unix seconds)
	start uint64

	// Epoch duration in seconds
	interval uint64

	// Lock to prevent concurrent aggregation
	running bool

	// Last aggregation epoch, -1 if no aggregation has been done yet while running this instance
	// It is set to the last finished aggregation epoch
	lastAggregationEpoch int

	db *gorm.DB
}

func NewUptimeAggregator(start uint64, interval uint, db *gorm.DB) *UptimeAggregator {
	return &UptimeAggregator{
		start:                start,
		interval:             uint64(interval),
		running:              false,
		lastAggregationEpoch: -1,
		db:                   db,
	}
}

func (a *UptimeAggregator) Run() {
	if a.running {
		return
	}
	a.running = true
	defer func() {
		a.running = false
	}()

	now := time.Now()
	currentAggregationEpoch := currentAggregationEpoch(now, a.start, a.interval)

	if currentAggregationEpoch < 0 || currentAggregationEpoch <= a.lastAggregationEpoch {
		return
	}

	epochStart := a.start + a.interval*uint64(currentAggregationEpoch)
	epochEnd := epochStart + a.interval

	// Last aggregation epoch for each of the nodes
	aggregations, err := database.FetchLastUptimeAggregation(a.db)
	if err != nil {
		logger.Error("Failed to fetch last uptime aggregations %w", err)
		return
	}

	// Get node start and end times
	stakingIntervals, err := fetchNodeStakingIntervals(a.db, epochStart, epochEnd)
	if err != nil {
		logger.Error("Failed to fetch node staking intervals %w", err)
		return
	}

	logger.Info("", stakingIntervals)

	// Aggregate each node
	for _, a := range aggregations {
		// Aggregation is up to date
		if a.Epoch >= currentAggregationEpoch {
			continue
		}

		// sort.

		// if a.Epoch < currentAggregationEpoch {
		// 	err := aggregateNodeUptime(
		// 		a.NodeID,
		// 		a.NodeStartTime,
		// 		a.NodeEndTime,
		// 		a.EpochStartTime,
		// 		a.EpochEndTime,
		// 	)
		// 	if err != nil {
		// 		logger.Error("Failed to aggregate node uptime %v", err)
		// 		continue
		// 	}
		// }
	}

}

// Return the current aggregation epoch index
func currentAggregationEpoch(now time.Time, startTimestamp uint64, interval uint64) int {
	return int((now.Unix() - int64(startTimestamp)) / int64(interval))
}

type nodeStakingInterval struct {
	nodeID string
	start  uint64
	end    uint64
}

// Return the staking intervals for each node, sorted by nodeID, note that it is possible
// that a node has multiple intervals
func fetchNodeStakingIntervals(db *gorm.DB, start uint64, end uint64) ([]nodeStakingInterval, error) {
	txs, err := database.FetchNodeStakingIntervals(db, database.PChainAddValidatorTx, time.Unix(int64(start), 0), time.Unix(int64(end), 0))
	if err != nil {
		return nil, err
	}
	intervals := make([]nodeStakingInterval, len(txs))
	for i, tx := range txs {
		intervals[i] = nodeStakingInterval{
			nodeID: tx.NodeID,
			start:  uint64(tx.StartTime.Unix()),
			end:    uint64(tx.EndTime.Unix()),
		}
	}
	sort.Slice(intervals, func(i, j int) bool {
		return intervals[i].nodeID < intervals[j].nodeID
	})
	return intervals, nil
}

func aggregateNodeUptime(
	db *gorm.DB,
	nodeID string,
	nodeStartTime time.Time,
	nodeEndTime time.Time,
	epochStartTime time.Time,
	epochEndTime time.Time,
) (*database.UptimeAggregation, error) {
	// uptimes are sorted by timestamp
	// uptimes, err := database.FetchNodeUptimes(db, nodeID, startTime, endTime)
	// if err != nil {
	// 	return nil, err
	// }

	// if epochEndTime

	// for _, uptime := range uptimes {
	// }
	return nil, nil
}
