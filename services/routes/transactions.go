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

func (rh *transactionRouteHandlers) getTransactionHandler() utils.RouteHandler {
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

func (rh *transactionRouteHandlers) listTransactionsHandler() utils.RouteHandler {
	handler := func(params map[string]string) ([]api.ApiPChainTxListItem, *utils.ErrorHandler) {
		txs, errHandler := rh.listTransactionsForEpoch(params)
		if errHandler != nil {
			return nil, errHandler
		}

		return api.NewApiPChainTxList(txs), nil
	}

	return utils.NewParamRouteHandler(handler, http.MethodGet,
		map[string]string{"epoch:[0-9]+": "Epoch"},
		[]api.ApiPChainTxListItem{},
	)
}

func (rh *transactionRouteHandlers) merkleRootHandler() utils.RouteHandler {
	handler := func(params map[string]string) (*api.ApiMerkleRoot, *utils.ErrorHandler) {
		txs, errHandler := rh.listTransactionsForEpoch(params)
		if errHandler != nil {
			return nil, errHandler
		}

		merkleRoot, err := staking.GetMerkleRoot(txs)
		if err != nil {
			return nil, utils.InternalServerErrorHandler(err)
		}

		return api.NewApiMerkleRoot(merkleRoot), nil
	}

	return utils.NewParamRouteHandler(handler, http.MethodGet,
		map[string]string{"epoch:[0-9]+": "Epoch"},
		new(api.ApiMerkleRoot),
	)

}

func (rh *transactionRouteHandlers) listTransactionsForEpoch(
	params map[string]string,
) ([]database.PChainTxData, *utils.ErrorHandler) {
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

	return txs, nil
}

func AddTransactionRoutes(
	router utils.Router, ctx context.ServicesContext, epochs staking.EpochInfo,
) {
	vr := newTransactionRouteHandlers(ctx, epochs)
	subrouter := router.WithPrefix("/transactions", "Transactions")
	subrouter.AddRoute("/get/{tx_id:[0-9a-zA-Z]+}", vr.getTransactionHandler())
	subrouter.AddRoute("/list/{epoch:[0-9]+}", vr.listTransactionsHandler())
	subrouter.AddRoute("/merkle-root/{epoch:[0-9]+}", vr.merkleRootHandler())
}
