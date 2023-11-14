package routes

import (
	"errors"
	globalConfig "flare-indexer/config"
	"flare-indexer/database"
	"flare-indexer/services/config"
	"flare-indexer/services/context"
	"flare-indexer/services/utils"
	"flare-indexer/utils/contracts/mirroring"
	"flare-indexer/utils/contracts/voting"
	"flare-indexer/utils/staking"
	"fmt"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"gorm.io/gorm"
)

type GetMirroringRequest struct {
	TxID string `json:"txId"`
}

type MirroringStakeData struct {
	TxID         string `json:"txId"`
	StakingType  uint8  `json:"stakingType"`
	InputAddress string `json:"inputAddress"`
	NodeId       string `json:"nodeId"`
	StartTime    uint64 `json:"startTime"`
	EndTime      uint64 `json:"endTime"`
	Weight       uint64 `json:"weight"`
}

type MirroringResponse struct {
	StakeData   MirroringStakeData `json:"stakeData"`
	MerkleProof []string           `json:"merkleProof"`
	TxInput     string             `json:"txInput"`
}

type GetMirroringResponse []MirroringResponse

type mirrorDB interface {
	GetPChainTxsForEpoch(start, end time.Time) ([]database.PChainTxData, error)
	GetPChainTx(txID string) (*database.PChainTx, error)
}

type mirroringRouteHandlers struct {
	db     mirrorDB
	epochs staking.EpochInfo
}

func newMirroringRouteHandlers(ctx context.ServicesContext) (*mirroringRouteHandlers, error) {
	cfg := ctx.Config()

	start, period, err := getEpochStartAndPeriod(cfg)
	if err != nil {
		return nil, err
	}

	return &mirroringRouteHandlers{
		db:     NewMirrorDBGorm(ctx.DB()),
		epochs: staking.NewEpochInfo(&globalConfig.EpochConfig{}, start, period),
	}, nil
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

func (rh *mirroringRouteHandlers) listMirroringTransactions() utils.RouteHandler {
	handler := func(params map[string]string) (GetMirroringResponse, *utils.ErrorHandler) {
		txID := params["tx_id"]
		tx, err := rh.db.GetPChainTx(txID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return GetMirroringResponse{}, utils.HttpErrorHandler(http.StatusBadRequest, "tx not found")
			} else {
				return GetMirroringResponse{}, utils.InternalServerErrorHandler(err)
			}
		}
		response, err := rh.createMirroringData(tx)
		if err != nil {
			return GetMirroringResponse{}, utils.InternalServerErrorHandler(err)
		}
		return response, nil
	}

	return utils.NewParamRouteHandler(handler, http.MethodGet,
		map[string]string{"tx_id:[0-9a-zA-Z]+": "Transaction ID"},
		GetMirroringResponse{})
}

func AddMirroringRoutes(router utils.Router, ctx context.ServicesContext) error {
	rh, err := newMirroringRouteHandlers(ctx)
	if err != nil {
		return err
	}

	mirroringSubrouter := router.WithPrefix("/mirroring", "Mirroring")
	mirroringSubrouter.AddRoute("/tx_data/{tx_id:[0-9a-zA-Z]+}", rh.listMirroringTransactions())

	return nil
}

func (rh *mirroringRouteHandlers) createMirroringData(tx *database.PChainTx) ([]MirroringResponse, error) {
	epoch := rh.epochs.GetEpochIndex(*tx.StartTime)
	startTimestamp, endTimestamp := rh.epochs.GetTimeRange(epoch)
	txs, err := rh.db.GetPChainTxsForEpoch(startTimestamp, endTimestamp)
	if err != nil {
		return nil, err
	}
	txs = staking.DedupeTxs(txs)
	merkleTree, err := staking.BuildTree(txs)
	if err != nil {
		return nil, err
	}

	var mirroringData []MirroringResponse
	for _, txData := range txs {
		if txData.ID != tx.ID {
			continue
		}
		stakeData, err := staking.ToStakeData(&txData)
		if err != nil {
			return nil, err
		}
		merkleProof, err := staking.GetMerkleProof(merkleTree, &txData)
		if err != nil {
			return nil, err
		}
		txDataBytes, err := createMirrorTransactionBytes(stakeData, merkleProof)
		if err != nil {
			return nil, err
		}
		merkleProofStrings := make([]string, len(merkleProof))
		for i, proof := range merkleProof {
			merkleProofStrings[i] = hexutil.Encode(proof[:])
		}
		mirroringData = append(mirroringData, MirroringResponse{
			StakeData: MirroringStakeData{
				TxID:         hexutil.Encode(stakeData.TxId[:]),
				StakingType:  stakeData.StakingType,
				InputAddress: hexutil.Encode(stakeData.InputAddress[:]),
				NodeId:       hexutil.Encode(stakeData.NodeId[:]),
				StartTime:    stakeData.StartTime,
				EndTime:      stakeData.EndTime,
				Weight:       stakeData.Weight,
			},
			MerkleProof: merkleProofStrings,
			TxInput:     hexutil.Encode(txDataBytes),
		})
	}
	if len(mirroringData) == 0 {
		return nil, fmt.Errorf("no mirroring data found for tx %s", *tx.TxID)
	}
	return mirroringData, nil
}

func createMirrorTransactionBytes(stakeData *mirroring.IPChainStakeMirrorVerifierPChainStake, merkleProof [][32]byte) ([]byte, error) {
	abi, err := mirroring.MirroringMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	packed, err := abi.Pack("mirrorStake", *stakeData, merkleProof)
	if err != nil {
		return nil, err
	}
	return packed, nil
}

type mirrorDBGorm struct {
	db *gorm.DB
}

func NewMirrorDBGorm(db *gorm.DB) mirrorDBGorm {
	return mirrorDBGorm{db: db}
}

func (m mirrorDBGorm) GetPChainTxsForEpoch(start, end time.Time) ([]database.PChainTxData, error) {
	return database.GetPChainTxsForEpoch(&database.GetPChainTxsForEpochInput{
		DB:             m.db,
		StartTimestamp: start,
		EndTimestamp:   end,
	})
}

func (m mirrorDBGorm) GetPChainTx(txID string) (*database.PChainTx, error) {
	return database.FetchPChainTx(m.db, txID)
}
