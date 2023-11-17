package utils

import (
	globalconfig "flare-indexer/config"
	"flare-indexer/services/config"
	"flare-indexer/services/context"
	"flare-indexer/utils/contracts/voting"
	"flare-indexer/utils/staking"
	"time"
)

func NewEpochInfo(ctx context.ServicesContext) (staking.EpochInfo, error) {
	cfg := ctx.Config()

	start, period, err := getEpochStartAndPeriod(cfg)
	if err != nil {
		return staking.EpochInfo{}, err
	}

	return staking.NewEpochInfo(&globalconfig.EpochConfig{}, start, period), nil
}

func getEpochStartAndPeriod(cfg *config.Config) (time.Time, time.Duration, error) {
	eth, err := cfg.Chain.DialETH()
	if err != nil {
		return time.Time{}, 0, err
	}

	votingContract, err := voting.NewVoting(cfg.ContractAddresses.Voting, eth)
	if err != nil {
		return time.Time{}, 0, err
	}

	return staking.GetEpochConfig(votingContract)
}
