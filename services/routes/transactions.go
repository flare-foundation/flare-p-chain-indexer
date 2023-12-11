package routes

import (
	"flare-indexer/database"
	"flare-indexer/services/api"
	"flare-indexer/services/context"
	"flare-indexer/services/utils"
	"flare-indexer/utils/staking"
	"net/http"
	"strconv"

	"gorm.io/gorm"
)

type GetTransactionsByAddressRequest struct {
	PaginatedRequest
	Address string `json:"address"`
}

type transactionRouteHandlers struct {
	db     *gorm.DB
	epochs staking.EpochInfo
}

func newTransactionRouteHandlers(ctx context.ServicesContext, epochs staking.EpochInfo) *transactionRouteHandlers {
	return &transactionRouteHandlers{
		db:     ctx.DB(),
		epochs: epochs,
	}
}

func (rh *transactionRouteHandlers) getTransaction() utils.RouteHandler {
	handler := func(params map[string]string) (*api.ApiPChainTx, *utils.ErrorHandler) {
		txID := params["tx_id"]
		var resp *api.ApiPChainTx = nil
		err := database.DoInTransaction(rh.db, func(dbTx *gorm.DB) error {
			tx, inputs, outputs, err := database.FetchPChainTxFull(rh.db, txID)
			if err == nil {
				resp = api.NewApiPChainTx(tx, inputs, outputs)
			}
			return err
		})
		if err != nil {
			return nil, utils.InternalServerErrorHandler(err)
		}
		return resp, nil
	}
	return utils.NewParamRouteHandler(handler, http.MethodGet,
		map[string]string{"tx_id:[0-9a-zA-Z]+": "Transaction ID"},
		&api.ApiPChainTx{})
}

func (rh *transactionRouteHandlers) listTransactionsByEpoch() utils.RouteHandler {
	handler := func(params map[string]string) ([]api.ApiPChainTxListItem, *utils.ErrorHandler) {
		epoch, err := strconv.ParseInt(params["epoch"], 10, 64)
		if err != nil {
			return nil, utils.HttpErrorHandler(http.StatusBadRequest, "Invalid epoch")
		}

		startTimestamp, endTimestamp := rh.epochs.GetTimeRange(epoch)
		txs, err := database.GetPChainTxsForEpoch(&database.GetPChainTxsForEpochInput{
			DB:             rh.db,
			StartTimestamp: startTimestamp,
			EndTimestamp:   endTimestamp,
		})
		if err != nil {
			return nil, utils.InternalServerErrorHandler(err)
		}

		return api.NewApiPChainTxList(txs), nil
	}

	return utils.NewParamRouteHandler(handler, http.MethodGet,
		map[string]string{"epoch:[0-9]+": "Epoch"},
		[]api.ApiPChainTxListItem{},
	)
}

func (rh *transactionRouteHandlers) listTransactionsByAddress() utils.RouteHandler {
	handler := func(pathParams map[string]string, query GetTransactionsByAddressRequest, body interface{}) ([]api.ApiPChainTxListItem, *utils.ErrorHandler) {
		txs, err := database.FetchPChainTransactionsByInputAddress(rh.db, query.Address, query.Offset, query.Limit)
		if err != nil {
			return nil, utils.InternalServerErrorHandler(err)
		}
		return api.NewApiPChainTxList(txs), nil
	}
	return utils.NewGeneralRouteHandler(handler, http.MethodGet,
		nil,
		GetTransactionsByAddressRequest{},
		nil,
		[]api.ApiPChainTxListItem{},
	)
}

func AddTransactionRoutes(
	router utils.Router, ctx context.ServicesContext, epochs staking.EpochInfo,
) {
	vr := newTransactionRouteHandlers(ctx, epochs)
	subrouter := router.WithPrefix("/transactions", "Transactions")
	subrouter.AddRoute("/get/{tx_id:[0-9a-zA-Z]+}", vr.getTransaction())
	subrouter.AddRoute("/list/{epoch:[0-9]+}", vr.listTransactionsByEpoch())
	subrouter.AddRoute("/list", vr.listTransactionsByAddress())
}
