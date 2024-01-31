package routes

import (
	"flare-indexer/database"
	"flare-indexer/services/api"
	"flare-indexer/services/context"
	"flare-indexer/services/utils"
	"net/http"
	"strings"

	"gorm.io/gorm"
)

type miscRouteHandlers struct {
	db *gorm.DB
}

func newMiscRouteHandlers(ctx context.ServicesContext) *miscRouteHandlers {
	return &miscRouteHandlers{
		db: ctx.DB(),
	}
}

func (rh *miscRouteHandlers) getAddress() utils.RouteHandler {
	handler := func(params map[string]string) (*api.ApiAddress, *utils.ErrorHandler) {
		addr := strings.TrimPrefix(params["address"], "0x")
		address, err := database.FetchAddress(rh.db, addr)
		if err != nil {
			return nil, utils.InternalServerErrorHandler(err)
		}
		return api.NewApiAddress(address), nil
	}
	return utils.NewParamRouteHandler(handler, http.MethodGet,
		map[string]string{"address:[0-9a-zA-Z]+": "Bech address"},
		&api.ApiAddress{})
}

func AddMiscRoutes(router utils.Router, ctx context.ServicesContext) {
	mr := newMiscRouteHandlers(ctx)

	subrouter := router.WithPrefix("/misc", "Miscellaneous")
	subrouter.AddRoute("/addresses//{address:[0-9a-zA-Z]+}", mr.getAddress(),
		"Get eth address (C-chain address) from bech address (P-chain, X-chain address - unprefixed) or vice versa")
}
