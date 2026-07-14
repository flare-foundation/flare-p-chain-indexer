package staking

import (
	"flare-indexer/config"
	"flare-indexer/utils/contracts/voting"
	"time"

	"github.com/ava-labs/libevm/accounts/abi/bind"
)

type EpochInfo struct {
	Period time.Duration
	Start  time.Time
	First  int64
}

func NewEpochInfo(cfg *config.EpochConfig, start time.Time, period time.Duration) EpochInfo {
	return EpochInfo{
		Period: period,
		Start:  start,
		First:  cfg.First,
	}
}

func (e EpochInfo) GetStartTime(epoch int64) time.Time {
	return e.Start.Add(time.Duration(epoch) * e.Period)
}

func (e EpochInfo) GetEndTime(epoch int64) time.Time {
	return e.GetStartTime(epoch + 1)
}

func (e EpochInfo) GetTimeRange(epoch int64) (time.Time, time.Time) {
	return e.GetStartTime(epoch), e.GetEndTime(epoch)
}

func (e EpochInfo) GetEpochIndex(t time.Time) int64 {
	return int64(t.Sub(e.Start) / e.Period)
}

func GetEpochConfig(votingContract *voting.Voting) (time.Time, time.Duration, error) {
	chainCfg, err := votingContract.GetEpochConfiguration(&bind.CallOpts{})
	if err != nil {
		return time.Time{}, 0, err
	}

	start := time.Unix(chainCfg.FirstEpochStartTs.Int64(), 0)
	period := time.Duration(chainCfg.EpochDurationSeconds.Int64()) * time.Second

	return start, period, nil
}
