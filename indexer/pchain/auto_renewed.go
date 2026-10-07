package pchain

import (
	"flare-indexer/database"
	"flare-indexer/logger"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Auto-renewed validators (avalanchego v1.15.0, Helicon upgrade, ACP-236) validate in consecutive
// cycles. The chain settles each cycle with a RewardAutoRenewedValidatorTx in a proposal block: its
// commit option renews the validator for StakingInfo.NextPeriod seconds (the Period of the latest
// SetAutoRenewedValidatorConfigTx accepted before the reward tx, or the Period of the
// AddAutoRenewedValidatorTx if none was issued), starting exactly when the finished cycle ends. The
// abort option, or a next period of 0, removes the validator and returns the stake.
//
// The indexer stores every cycle as an ordinary staking row, so that voting, mirroring and uptime
// voting treat it as a (fixed-term) validator stake:
//   - the first cycle on the ADD_AUTO_RENEWED_VALIDATOR_TX row (start_time = block time,
//     end_time = start_time + period), see updateUnboundedValidatorTx;
//   - every renewed cycle on the row of the REWARD_AUTO_RENEWED_VALIDATOR_TX that started it: node id,
//     weight, subnet and delegation fee copied from the add tx, start_time = block time (= end of the
//     previous cycle), end_time = start_time + next period, period = next period. The stake's input
//     address and signing credential are those of the add tx (PChainTx.StakerTxID).
//
// A reward tx row of a validator that exited gets period = 0 and no start/end time. Rows whose
// commit/abort block is not indexed yet keep period = NULL and are completed by
// completeAutoRenewedRewardTxs as soon as it is (normally in the same batch, otherwise in the next one).
//
// Weight: a share of the rewards may be restaked at renewal (AutoCompoundRewardShares), which is not
// tracked here. On Flare networks the P-chain mints no staking rewards, so the weight never changes.

// Apply the outcome of the finished validation cycles (reward auto renewed validator txs whose
// commit/abort block is indexed) in blocks [fromHeight, toHeight] to the reward tx rows.
// Must run in the same transaction as the insertion of the rows.
func completeAutoRenewedRewardTxs(db *gorm.DB, fromHeight uint64, toHeight uint64) error {
	pendingTxs, err := database.FetchPendingAutoRenewedRewardTxs(db, fromHeight, toHeight)
	if err != nil {
		return err
	}
	for i := range pendingTxs {
		rewardTx := &pendingTxs[i]
		addTx, err := database.FetchPChainTx(db, rewardTx.RewardTxID)
		if err != nil {
			return fmt.Errorf("add auto renewed validator tx %s referenced by reward tx %s: %w",
				rewardTx.RewardTxID, *rewardTx.TxID, err)
		}
		configTx, err := database.FetchLatestAutoRenewedConfigTx(db, rewardTx.RewardTxID, rewardTx.BlockHeight, rewardTx.ID)
		if err != nil {
			return err
		}
		committed := rewardTx.OptionBlockType == database.PChainCommitBlock
		if err := applyAutoRenewal(&rewardTx.PChainTx, addTx, configTx, committed); err != nil {
			return err
		}
		if err := database.UpdatePChainTx(db, &rewardTx.PChainTx); err != nil {
			return err
		}
		if rewardTx.StartTime != nil {
			logger.Info("auto renewed validator %s (node %s) renewed by tx %s for %d seconds",
				rewardTx.RewardTxID, rewardTx.NodeID, *rewardTx.TxID, *rewardTx.Period)
		} else {
			logger.Info("auto renewed validator %s exited at reward tx %s (committed: %v)",
				rewardTx.RewardTxID, *rewardTx.TxID, committed)
		}
	}
	return nil
}

// Period of the next validation cycle in effect at the end of a cycle: the Period of the latest
// config tx, or the add tx's Period if the config was never changed
func nextAutoRenewPeriod(addTx *database.PChainTx, configTx *database.PChainTx) (uint64, error) {
	if configTx != nil {
		if configTx.Period == nil {
			return 0, fmt.Errorf("set auto renewed validator config tx %s has no period", *configTx.TxID)
		}
		return *configTx.Period, nil
	}
	if addTx.Period == nil {
		return 0, fmt.Errorf("add auto renewed validator tx %s has no period", *addTx.TxID)
	}
	return *addTx.Period, nil
}

// Apply the outcome of the validation cycle ended by rewardTx (committed: the proposal block was
// committed, otherwise aborted) to the reward tx row, see the description above
func applyAutoRenewal(rewardTx *database.PChainTx, addTx *database.PChainTx, configTx *database.PChainTx, committed bool) error {
	if addTx.Type != database.PChainAddAutoRenewedValidatorTx {
		return fmt.Errorf("reward tx %s references tx %s of type %s, expected %s",
			*rewardTx.TxID, *addTx.TxID, addTx.Type, database.PChainAddAutoRenewedValidatorTx)
	}
	nextPeriod, err := nextAutoRenewPeriod(addTx, configTx)
	if err != nil {
		return err
	}

	if !committed || nextPeriod == 0 {
		// The validator was removed and its stake returned; there is no new cycle to vote on or mirror
		exited := uint64(0)
		rewardTx.Period = &exited
		rewardTx.StartTime = nil
		rewardTx.EndTime = nil
		return nil
	}

	if rewardTx.BlockTime == nil {
		return fmt.Errorf("reward auto renewed validator tx %s has no block time", *rewardTx.TxID)
	}
	// The renewed cycle starts when the finished one ends; the chain only accepts the reward tx in a
	// block whose time equals the end of the cycle, which the tx also carries in its Timestamp (Time)
	if rewardTx.Time != nil && !rewardTx.Time.Equal(*rewardTx.BlockTime) {
		logger.Warn("reward auto renewed validator tx %s: cycle end time %v differs from block time %v",
			*rewardTx.TxID, *rewardTx.Time, *rewardTx.BlockTime)
	}
	startTime := *rewardTx.BlockTime
	endTime := time.Unix(startTime.Unix()+int64(nextPeriod), 0)

	rewardTx.NodeID = addTx.NodeID
	rewardTx.Weight = addTx.Weight
	rewardTx.SubnetID = addTx.SubnetID
	rewardTx.FeePercentage = addTx.FeePercentage
	rewardTx.StartTime = &startTime
	rewardTx.EndTime = &endTime
	rewardTx.Period = &nextPeriod
	return nil
}
