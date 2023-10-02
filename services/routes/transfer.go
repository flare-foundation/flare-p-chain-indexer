package routes

import (
	"flare-indexer/database"
	"flare-indexer/services/context"
	"flare-indexer/services/utils"
	"net/http"

	"gorm.io/gorm"
)

type GetTransferRequest struct {
	PaginatedRequest
	Address string `json:"address"`
}

type transferRouteHandlers struct {
	db *gorm.DB
}

func newTransferRouteHandlers(ctx context.ServicesContext) *transferRouteHandlers {
	return &transferRouteHandlers{
		db: ctx.DB(),
	}
}

func (rh *transferRouteHandlers) listTransferTransactions(txType database.PChainTxType) utils.RouteHandler {
	handler := func(request GetTransferRequest) (TxIDsResponse, *utils.ErrorHandler) {
		txIDs, err := database.FetchPChainTransferTransactions(rh.db, txType,
			request.Address, request.Offset, request.Limit)
		if err != nil {
			return TxIDsResponse{}, utils.InternalServerErrorHandler(err)
		}
		return TxIDsResponse{TxIDs: txIDs}, nil
	}
	return utils.NewRouteHandler(handler, http.MethodPost, GetTransferRequest{}, TxIDsResponse{})
}

func AddTransferRoutes(router utils.Router, ctx context.ServicesContext) {
	vr := newTransferRouteHandlers(ctx)

	importSubrouter := router.WithPrefix("/imports", "Transfers")
	importSubrouter.AddRoute("/transactions", vr.listTransferTransactions(database.PChainImportTx))

	exportSubrouter := router.WithPrefix("/exports", "Transfers")
	exportSubrouter.AddRoute("/transactions", vr.listTransferTransactions(database.PChainExportTx))
}
