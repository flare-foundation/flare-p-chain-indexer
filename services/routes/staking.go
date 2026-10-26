package routes

import (
	"flare-indexer/database"
	"flare-indexer/services/context"
	"flare-indexer/services/utils"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"
)

type GetStakerTxRequest struct {
	PaginatedRequest
	NodeID  string    `json:"nodeId"`
	Address string    `json:"address"`
	Time    time.Time `json:"time"`
}

type GetStakerRequest struct {
	PaginatedRequest
	Time time.Time `json:"time"`
}

type GetStakerResponse struct {
	TxID           string    `json:"txID"`
	NodeID         string    `json:"nodeID"`
	StartTime      time.Time `json:"startTime"`
	EndTime        time.Time `json:"endTime"`
	Weight         uint64    `json:"weight"`
	FeePercentage  uint32    `json:"feePercentage"`
	InputAddresses []string  `json:"inputAddresses"`
}

type stakerRouteHandlers struct {
	db *gorm.DB
}

func newStakerRouteHandlers(ctx context.ServicesContext) *stakerRouteHandlers {
	return &stakerRouteHandlers{
		db: ctx.DB(),
	}
}

func (rh *stakerRouteHandlers) listStakingTransactions(txTypes ...database.PChainTxType) utils.RouteHandler {
	handler := func(request GetStakerTxRequest) (TxIDsResponse, *utils.ErrorHandler) {
		txIDs, err := database.FetchPChainStakingTransactions(rh.db, txTypes, request.NodeID,
			request.Address, request.Time, request.Offset, request.Limit)
		if err != nil {
			return TxIDsResponse{}, utils.InternalServerErrorHandler(err)
		}
		return TxIDsResponse{TxIDs: txIDs}, nil
	}
	return utils.NewRouteHandler(handler, http.MethodPost, GetStakerTxRequest{}, TxIDsResponse{})
}

func (rh *stakerRouteHandlers) listStakers(txTypes ...database.PChainTxType) utils.RouteHandler {
	handler := func(request GetStakerRequest) ([]GetStakerResponse, *utils.ErrorHandler) {
		stakerTxData, err := database.FetchPChainStakingData(rh.db, request.Time, txTypes, request.Offset, request.Limit)
		if err != nil {
			return nil, utils.InternalServerErrorHandler(err)
		}
		stakers := make([]GetStakerResponse, len(stakerTxData))
		for i, tx := range stakerTxData {
			stakers[i] = GetStakerResponse{
				TxID:           *tx.TxID,
				NodeID:         tx.NodeID,
				StartTime:      *tx.StartTime,
				EndTime:        *tx.EndTime,
				Weight:         tx.Weight,
				FeePercentage:  tx.FeePercentage,
				InputAddresses: strings.Split(tx.InputAddress, ","),
			}
		}
		return stakers, nil
	}
	return utils.NewRouteHandler(handler, http.MethodPost, GetStakerRequest{}, []GetStakerResponse{})
}

func AddStakerRoutes(router utils.Router, ctx context.ServicesContext) {
	vr := newStakerRouteHandlers(ctx)

	validatorSubrouter := router.WithPrefix("/validators", "Staking")
	// Auto-renewed validators are listed by validation cycle: the add tx for the first one, the reward tx
	// that started it for every renewed one
	validatorTxTypes := []database.PChainTxType{
		database.PChainAddValidatorTx, database.PChainAddPermissionlessValidatorTx,
		database.PChainAddAutoRenewedValidatorTx, database.PChainRewardAutoRenewedValidatorTx,
	}
	validatorSubrouter.AddRoute("/transactions", vr.listStakingTransactions(validatorTxTypes...))
	validatorSubrouter.AddRoute("/list", vr.listStakers(validatorTxTypes...))

	delegatorSubrouter := router.WithPrefix("/delegators", "Staking")
	delegatorSubrouter.AddRoute("/transactions",
		vr.listStakingTransactions(database.PChainAddDelegatorTx, database.PChainAddPermissionlessDelegatorTx))
	delegatorSubrouter.AddRoute("/list",
		vr.listStakers(database.PChainAddDelegatorTx, database.PChainAddPermissionlessDelegatorTx))
}
