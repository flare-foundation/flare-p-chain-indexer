package cronjob

import (
	"flare-indexer/database"
	"flare-indexer/indexer/context"
	"flare-indexer/logger"
	"flare-indexer/utils"
	"fmt"
	"sort"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"gorm.io/gorm"
)

type uptimeAggregator struct {
	// General cronjob settings (read from config for uptime cronjob)
	enabled bool
	timeout int

	// epoch start timestamp (unix seconds)
	start int64

	// Epoch duration in seconds
	interval int64

	// Lock to prevent concurrent aggregation
	running bool

	// Last aggregation epoch, -1 if no aggregation has been done yet while running this instance
	// It is set to the last finished aggregation epoch
	lastAggregatedEpoch int64

	db *gorm.DB
}

func NewAggregationCronjob(ctx context.IndexerContext) Cronjob {
	config := ctx.Config().UptimeCronjob
	return &uptimeAggregator{
		start:               config.AggregateStartTimestamp,
		interval:            config.AggregateIntervalSeconds,
		timeout:             config.TimeoutSeconds,
		enabled:             config.Enabled,
		running:             false,
		lastAggregatedEpoch: -1,
		db:                  ctx.DB(),
	}

}

func (a *uptimeAggregator) Name() string {
	return "uptime_aggregator"
}

func (a *uptimeAggregator) TimeoutSeconds() int {
	return a.timeout
}

func (a *uptimeAggregator) Enabled() bool {
	return a.enabled
}

func (a *uptimeAggregator) OnStart() error {
	return nil
}

func (a *uptimeAggregator) Call() error {
	if a.running {
		return nil
	}
	a.running = true
	defer func() {
		a.running = false
	}()

	now := time.Now()
	currentAggregationEpoch := currentAggregationEpoch(now, a.start, a.interval)
	lastEpochToAggregate := currentAggregationEpoch - 1

	// If we are sure that we have aggregated all the epochs up to lastEpochToAggregate, we can skip
	if lastEpochToAggregate < 0 || lastEpochToAggregate <= a.lastAggregatedEpoch {
		return nil
	}

	// Last aggregation epoch (epoch of the last persisted aggregation of any node since we
	// store all epoch aggregations at once)
	lastAggregation, err := database.FetchLastUptimeAggregation(a.db)
	if err != nil {
		return fmt.Errorf("failed fetching last uptime aggregation %w", err)
	}
	var firstEpochToAggregate int64
	if lastAggregation == nil {
		firstEpochToAggregate = 0
	} else {
		firstEpochToAggregate = int64(lastAggregation.Epoch) + 1
	}

	aggregations := make([]*database.UptimeAggregation, 0)

	// Aggregate missing epochs for all nodes

	// Minimal non-aggregated epoch for each of the nodes
	for epoch := firstEpochToAggregate; epoch <= lastEpochToAggregate; epoch++ {
		epochStart := a.start + a.interval*epoch
		epochEnd := epochStart + a.interval

		// Get start and end times for all staking intervals that overlap with the current epoch
		stakingIntervals, err := fetchNodeStakingIntervals(a.db, epochStart, epochEnd)
		if err != nil {
			return fmt.Errorf("failed fetching node staking intervals %w", err)
		}

		epochNodes := mapset.NewSet[string]()
		for _, interval := range stakingIntervals {
			epochNodes.Add(interval.nodeID)
		}

		// Aggregate each node
		for nodeID := range epochNodes.Iter() {

			// Find (the first) staking interval for the node
			idx := sort.Search(len(stakingIntervals), func(i int) bool {
				return stakingIntervals[i].nodeID >= nodeID
			})

			nodeConnectedTime := int64(0)
			stakingDuration := int64(0)
			for ; idx < len(stakingIntervals) && stakingIntervals[idx].nodeID == nodeID; idx++ {
				start, end := utils.IntervalIntersection(stakingIntervals[idx].start, stakingIntervals[idx].end, epochStart, epochEnd)
				if end <= start {
					continue
				}
				ct, err := aggregateNodeUptime(a.db, nodeID, start, end)
				if err != nil {
					return fmt.Errorf("failed aggregating node uptime %w", err)
				}
				nodeConnectedTime += ct
				stakingDuration += end - start
			}
			aggregations = append(aggregations, &database.UptimeAggregation{
				NodeID:          nodeID,
				Epoch:           int(epoch),
				StartTime:       time.Unix(epochStart, 0),
				EndTime:         time.Unix(epochEnd, 0),
				Value:           nodeConnectedTime,
				StakingDuration: stakingDuration,
			})
		}
		logger.Info("Aggregated uptime for epoch %d", epoch)
	}

	// Persist all aggregations at once, so we have a complete set of aggregations for each epoch
	err = database.PersistUptimeAggregations(a.db, aggregations)
	if err != nil {
		return fmt.Errorf("failed persisting uptime aggregations %w", err)
	}
	a.lastAggregatedEpoch = lastEpochToAggregate
	return nil
}

// Return the current aggregation epoch index
func currentAggregationEpoch(now time.Time, startTimestamp int64, interval int64) int64 {
	return (now.Unix() - startTimestamp) / interval
}

type nodeStakingInterval struct {
	nodeID string
	start  int64
	end    int64
}

// Return the staking intervals for each node, sorted by nodeID, note that it is possible
// that a node has multiple intervals
func fetchNodeStakingIntervals(db *gorm.DB, start int64, end int64) ([]nodeStakingInterval, error) {
	txs, err := database.FetchNodeStakingIntervals(db, database.PChainAddValidatorTx, time.Unix(start, 0), time.Unix(end, 0))
	if err != nil {
		return nil, err
	}
	intervals := make([]nodeStakingInterval, len(txs))
	for i, tx := range txs {
		intervals[i] = nodeStakingInterval{
			nodeID: tx.NodeID,
			start:  tx.StartTime.Unix(),
			end:    tx.EndTime.Unix(),
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
	startTimestamp int64,
	endTimestamp int64,
) (int64, error) {
	// uptimes are sorted by timestamp
	uptimes, err := database.FetchNodeUptimes(db, nodeID, time.Unix(startTimestamp, 0), time.Unix(endTimestamp, 0))
	if err != nil {
		return 0, err
	}
	connectedTime := int64(0)
	prev := startTimestamp
	for _, uptime := range uptimes {
		curr := uptime.Timestamp.Unix()
		// Consider all states (connected, errors) as connected
		if uptime.Status != database.UptimeCronjobStatusDisconnected {
			connectedTime += curr - prev
		}
		prev = curr
	}
	if prev < endTimestamp {
		// Assume that the node is connected until the end of the epoch
		connectedTime += endTimestamp - prev
	}
	return connectedTime, nil
}
