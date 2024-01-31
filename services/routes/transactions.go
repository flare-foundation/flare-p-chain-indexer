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
	InputAddress  string `json:"inputAddress"`
	OutputAddress string `json:"outputAddress"`
	BlockHeight   uint64 `json:"blockHeight" jsonschema:"Filter by transactions with block height >= blockHeight. Blocks start with 1. If not specified, 1 is used."`
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

func (rh *transactionRouteHandlers) listTransactionsByAddresses() utils.RouteHandler {
	handler := func(pathParams map[string]string, query GetTransactionsByAddressRequest, body interface{}) ([]api.ApiPChainTx, *utils.ErrorHandler) {
		txs, err := database.FetchPChainTransactionsByAddresses(rh.db, query.InputAddress, query.OutputAddress, query.BlockHeight, query.Offset, query.Limit)
		if err != nil {
			return nil, utils.InternalServerErrorHandler(err)
		}
		return api.NewApiPChainTxInOutList(txs), nil
	}
	return utils.NewGeneralRouteHandler(handler, http.MethodGet,
		nil,
		GetTransactionsByAddressRequest{},
		nil,
		[]api.ApiPChainTx{},
	)
}

func (rh *transactionRouteHandlers) maxBlockHeight() utils.RouteHandler {
	handler := func(pathParams map[string]string, query interface{}, body interface{}) (uint64, *utils.ErrorHandler) {
		blockNumber, err := database.GetMaxBlockHeight(rh.db)
		if err != nil {
			return 0, utils.InternalServerErrorHandler(err)
		}
		return blockNumber, nil
	}
	return utils.NewGeneralRouteHandler(handler, http.MethodGet,
		nil,
		nil,
		nil,
		uint64(0),
	)
}

func AddTransactionRoutes(
	router utils.Router, ctx context.ServicesContext, epochs staking.EpochInfo,
) {
	vr := newTransactionRouteHandlers(ctx, epochs)
	subrouter := router.WithPrefix("/transactions", "Transactions")
	subrouter.AddRoute("/get/{tx_id:[0-9a-zA-Z]+}", vr.getTransaction())
	subrouter.AddRoute("/list/{epoch:[0-9]+}", vr.listTransactionsByEpoch())
	subrouter.AddRoute("/list", vr.listTransactionsByAddresses(),
		"List all transactions by input or output address.")
	subrouter.AddRoute("/max_block_height", vr.maxBlockHeight())
}
