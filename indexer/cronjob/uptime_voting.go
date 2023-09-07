package cronjob

import (
	"flare-indexer/database"
	"flare-indexer/indexer/context"
	"flare-indexer/logger"
	"flare-indexer/utils"
	"flare-indexer/utils/contracts/voting"
	"fmt"
	"math/big"
	"sort"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

var (
	errNoEpochsToAggregate = errors.New("no epochs to aggregate")
)

type uptimeVotingCronjob struct {
	// General cronjob settings (read from config for uptime cronjob)
	enabled bool
	timeout int

	// epoch start timestamp (unix seconds)
	epochs epochInfo

	// Last aggregation epoch, -1 if no aggregation has been done yet while running this instance
	// It is set to the last finished aggregation epoch
	lastAggregatedEpoch int64

	// A limited period after the end of the reward epoch to send the uptimes to the node
	votingInterval time.Duration

	uptimeThreshold float64

	votingContract *voting.Voting
	txOpts         *bind.TransactOpts

	db *gorm.DB

	// For testing to set "now" to some past date
	time utils.ShiftedTime
}

func NewUptimeVotingCronjob(ctx context.IndexerContext) (*uptimeVotingCronjob, error) {
	cfg := ctx.Config()

	if !cfg.UptimeCronjob.EnableVoting {
		return &uptimeVotingCronjob{}, nil
	}

	votingContract, err := newVotingContract(cfg)
	if err != nil {
		return nil, err
	}
	txOpts, err := TransactOptsFromPrivateKey(cfg.Chain.PrivateKey, cfg.Chain.ChainID)
	if err != nil {
		return nil, err
	}

	config := ctx.Config().UptimeCronjob
	return &uptimeVotingCronjob{
		epochs:              newEpochInfo(&ctx.Config().UptimeCronjob.EpochConfig),
		timeout:             config.TimeoutSeconds,
		enabled:             config.EnableVoting,
		lastAggregatedEpoch: -1,
		uptimeThreshold:     config.UptimeThreshold,
		votingInterval:      config.VotingInterval,
		votingContract:      votingContract,
		txOpts:              txOpts,
		db:                  ctx.DB(),
	}, nil

}

func (c *uptimeVotingCronjob) Name() string {
	return "uptime_aggregator"
}

func (c *uptimeVotingCronjob) TimeoutSeconds() int {
	return c.timeout
}

func (c *uptimeVotingCronjob) Enabled() bool {
	return c.enabled
}

func (c *uptimeVotingCronjob) OnStart() error {
	return nil
}

func (c *uptimeVotingCronjob) Call() error {
	now := c.time.Now()
	firstEpochToAggregate, lastEpochToAggregate, err := c.aggregationRange(now)
	if err != nil {
		if err == errNoEpochsToAggregate {
			return nil
		}
		return err
	}

	var aggregations []*database.UptimeAggregation
	lastAggregatedEpoch := c.lastAggregatedEpoch

	// Aggregate missing epochs for all nodes
	for epoch := firstEpochToAggregate; epoch <= lastEpochToAggregate; epoch++ {
		nodeAggregations, err := c.aggregateEpoch(epoch)
		if err != nil {
			return err
		}

		// One can submit votes even if they were submitted before, so we do not need to
		// handle potential errors when persisting the aggregations
		submitErr := c.submitVotes(epoch, nodeAggregations)
		if submitErr != nil {
			logger.Error("Failed submitting uptime votes for epoch %d: %v", epoch, submitErr)
			break
		}

		aggregations = append(aggregations, nodeAggregations...)
		lastAggregatedEpoch = epoch
		logger.Info("Aggregated uptime for epoch %d", epoch)
	}

	// Persist all aggregations at once, so we have a complete set of aggregations for each epoch
	// TODO: at the same time, remove uptimes that are not needed anymore to prevent the database
	//       from growing too large
	err = database.PersistUptimeAggregations(c.db, aggregations)
	if err != nil {
		return fmt.Errorf("failed persisting uptime aggregations %w", err)
	}
	c.lastAggregatedEpoch = lastAggregatedEpoch
	return nil
}

func (c *uptimeVotingCronjob) aggregationRange(now time.Time) (firstEpochToAggregate int64, lastEpochToAggregate int64, err error) {
	currentAggregationEpoch := c.epochs.getEpochIndex(now)
	lastEpochToAggregate = currentAggregationEpoch - 1

	// If we are sure that we have aggregated all the epochs up to lastEpochToAggregate, we can skip
	if lastEpochToAggregate < 0 || lastEpochToAggregate <= c.lastAggregatedEpoch {
		err = errNoEpochsToAggregate
		return
	}

	// Last aggregation epoch (epoch of the last persisted aggregation of any node since we
	// store all epoch aggregations at once)
	lastAggregation, dbErr := database.FetchLastUptimeAggregation(c.db)
	if dbErr != nil {
		err = fmt.Errorf("failed fetching last uptime aggregation %w", dbErr)
		return
	}
	if lastAggregation == nil {
		firstEpochToAggregate = 0
	} else {
		firstEpochToAggregate = int64(lastAggregation.Epoch) + 1
	}
	return
}

func (c *uptimeVotingCronjob) aggregateEpoch(epoch int64) ([]*database.UptimeAggregation, error) {
	epochStart, epochEnd := c.epochs.getTimeRange(epoch)

	// Get start and end times for all staking intervals that overlap with the current epoch
	stakingIntervals, err := fetchNodeStakingIntervals(c.db, epochStart, epochEnd)
	if err != nil {
		return nil, fmt.Errorf("failed fetching node staking intervals %w", err)
	}

	epochNodes := mapset.NewSet[string]()
	for _, interval := range stakingIntervals {
		epochNodes.Add(interval.nodeID)
	}

	// Aggregate each node
	nodeAggregations := make([]*database.UptimeAggregation, 0, epochNodes.Cardinality())
	for nodeID := range epochNodes.Iter() {
		nodeAggregation, err := c.aggregateNode(epoch, nodeID, stakingIntervals)
		if err != nil {
			return nil, err
		}

		nodeAggregations = append(nodeAggregations, nodeAggregation)
	}
	return nodeAggregations, nil
}

// Aggregate the uptime for a node in the given epoch, stakingIntervals are the staking intervals for
// all nodes that overlap with the epoch (sorted by nodeID)
func (c *uptimeVotingCronjob) aggregateNode(epoch int64, nodeID string, stakingIntervals []nodeStakingInterval) (*database.UptimeAggregation, error) {
	// Find (the first) staking interval for the node
	idx := sort.Search(len(stakingIntervals), func(i int) bool {
		return stakingIntervals[i].nodeID >= nodeID
	})

	epochStart, epochEnd := c.epochs.getTimeRange(epoch)
	nodeConnectedTime := int64(0)
	stakingDuration := int64(0)
	for ; idx < len(stakingIntervals) && stakingIntervals[idx].nodeID == nodeID; idx++ {
		start, end := utils.IntervalIntersection(stakingIntervals[idx].start, stakingIntervals[idx].end, epochStart.Unix(), epochEnd.Unix())
		if end <= start {
			continue
		}
		ct, err := aggregateNodeUptime(c.db, nodeID, start, end)
		if err != nil {
			return nil, fmt.Errorf("failed aggregating node uptime %w", err)
		}
		nodeConnectedTime += ct
		stakingDuration += end - start
	}

	return &database.UptimeAggregation{
		NodeID:          nodeID,
		Epoch:           int(epoch),
		StartTime:       epochStart,
		EndTime:         epochEnd,
		Value:           nodeConnectedTime,
		StakingDuration: stakingDuration,
	}, nil
}

func (c *uptimeVotingCronjob) submitVotes(epoch int64, nodeAggregations []*database.UptimeAggregation) error {
	nodeIDs := make([][20]byte, 0, len(nodeAggregations))
	for _, a := range nodeAggregations {
		if a.StakingDuration == 0 {
			continue
		}

		uptimePercent := float64(a.Value) / float64(a.StakingDuration)
		if uptimePercent < c.uptimeThreshold {
			continue
		}

		nodeID, err := ids.NodeIDFromString(a.NodeID)
		if err != nil {
			return errors.Wrap(err, "ids.NodeIDFromString")
		}
		nodeIDs = append(nodeIDs, nodeID)
	}
	_, err := c.votingContract.SubmitValidatorUptimeVote(c.txOpts, big.NewInt(epoch), nodeIDs)
	return err
}

type nodeStakingInterval struct {
	nodeID string
	start  int64
	end    int64
}

// Return the staking intervals for each node, sorted by nodeID, note that it is possible
// that a node has multiple intervals
func fetchNodeStakingIntervals(db *gorm.DB, start time.Time, end time.Time) ([]nodeStakingInterval, error) {
	txs, err := database.FetchNodeStakingIntervals(db, database.PChainAddValidatorTx, start, end)
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