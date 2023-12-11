package routes

import (
	"errors"
	"flare-indexer/config"
	"flare-indexer/database"
	"flare-indexer/services/api"
	"flare-indexer/services/context"
	"flare-indexer/services/utils"
	globalUtils "flare-indexer/utils"
	"net/http"

	"github.com/ava-labs/avalanchego/ids"
	"gorm.io/gorm"
)

func AddQueryRoutes(router utils.Router, ctx context.ServicesContext) {
	qr := newQueryRouteHandlers(ctx)
	subrouter := router.WithPrefix("/query", "Query")

	subrouter.AddRoute("", qr.processAttestationRequest(), "",
		"Verifies attestation request")
	subrouter.AddRoute("/prepare", qr.prepareRequest(), "",
		"Given parsed request in JSON with possibly invalid message integrity code it returns the verification object")
	subrouter.AddRoute("/integrity", qr.integrityRequest(), "",
		"Given parsed request in JSON with possibly invalid message integrity code it returns the message integrity code.")
	subrouter.AddRoute("/prepareAttestation", qr.prepareAttestationRequest(), "",
		"Given parsed request in JSON with possibly invalid message integrity code it returns the byte encoded  attestation request with the correct message integrity code. The response can be directly used for submitting attestation request to StateConnector smart contract.")
}

type queryRouteHandlers struct {
	db  *gorm.DB
	cfg config.ChainConfig
}

func newQueryRouteHandlers(ctx context.ServicesContext) *queryRouteHandlers {
	return &queryRouteHandlers{
		db:  ctx.DB(),
		cfg: ctx.Config().ChainConfig(),
	}
}

// Verifies attestation request
//
// Request type: api.APIAttestationRequest
// Response type: api.ApiResponseWrapper[api.APIVerification[api.ARPChainStaking, api.DHPChainStaking]]
func (qr *queryRouteHandlers) processAttestationRequest() utils.RouteHandler {
	handler := func(request api.APIAttestationRequest) (*api.APIVerification[api.ARPChainStaking, api.DHPChainStaking], *utils.ErrorHandler) {
		unpackedReq, err := utils.UnpackPChainStakingRequest(request.Request)
		if err != nil {
			return nil, utils.ApiResponseErrorHandler(api.ApiResStatusInvalidRequest, "invalid request", err.Error())
		}
		return qr.processPChainStakingRequest(unpackedReq)
	}
	return utils.NewBodyRouteHandler(handler, http.MethodPost, api.APIAttestationRequest{}, &api.APIVerification[api.ARPChainStaking, api.DHPChainStaking]{})
}

// Given parsed request in JSON with possibly invalid message integrity code it returns the verification object.
//
// Request type: api.ARPChainStaking
// Response type: api.ApiResponseWrapper[api.APIVerification[api.ARPChainStaking, api.DHPChainStaking]]
func (qr *queryRouteHandlers) prepareRequest() utils.RouteHandler {
	handler := func(request api.ARPChainStaking) (*api.APIVerification[api.ARPChainStaking, api.DHPChainStaking], *utils.ErrorHandler) {
		return qr.processPChainStakingRequest(&request)
	}
	return utils.NewBodyRouteHandler(handler, http.MethodPost, api.ARPChainStaking{}, &api.APIVerification[api.ARPChainStaking, api.DHPChainStaking]{})
}

// Given parsed request in JSON with possibly invalid message integrity code it returns the message
// integrity code.
//
// Request type: api.ARPChainStaking
// Response type: api.ApiResponseWrapper[string]
func (qr *queryRouteHandlers) integrityRequest() utils.RouteHandler {
	handler := func(request api.ARPChainStaking) (string, *utils.ErrorHandler) {
		response, errHandler := qr.processPChainStakingRequest(&request)
		if errHandler != nil {
			return "", errHandler
		}
		code, err := utils.HashPChainStaking(&request, response.Response, "")
		if err != nil {
			return "", utils.ApiResponseErrorHandler(api.ApiResStatusError, "internal error", err.Error())
		}
		return code, nil
	}
	return utils.NewBodyRouteHandler(handler, http.MethodPost, api.ARPChainStaking{}, "")
}

// Given parsed @param request in JSON with possibly invalid message integrity code it returns the byte encoded
// attestation request with the correct message integrity code. The response can be directly used for submitting
// attestation request to StateConnector smart contract.
//
// Request type: api.ARPChainStaking
// Response type: api.ApiResponseWrapper[string]
func (qr *queryRouteHandlers) prepareAttestationRequest() utils.RouteHandler {
	handler := func(request api.ARPChainStaking) (string, *utils.ErrorHandler) {
		response, errHandler := qr.processPChainStakingRequest(&request)
		if errHandler != nil {
			return "", errHandler
		}
		code, err := utils.HashPChainStaking(&request, response.Response, "")
		if err != nil {
			return "", utils.ApiResponseErrorHandler(api.ApiResStatusError, "internal error", err.Error())
		}
		request.MessageIntegrityCode = code
		packedRequest, err := utils.PackPChainStakingRequest(&request)
		if err != nil {
			return "", utils.ApiResponseErrorHandler(api.ApiResStatusError, "internal error", err.Error())
		}
		return packedRequest, nil
	}
	return utils.NewBodyRouteHandler(handler, http.MethodPost, api.ARPChainStaking{}, "")
}

// Process attestation request. Write errors into w, if any, otherwise return the response.
func (qr *queryRouteHandlers) processPChainStakingRequest(
	request *api.ARPChainStaking,
) (*api.APIVerification[api.ARPChainStaking, api.DHPChainStaking], *utils.ErrorHandler) {
	response, err1, err2 := qr.executePChainStakingRequest(request)
	if err1 != nil {
		return nil, utils.ApiResponseErrorHandler(api.ApiResStatusInvalidRequest, "invalid request", err1.Error())
	}
	if err2 != nil {
		return nil, utils.ApiResponseErrorHandler(api.ApiResStatusError, "internal error", err2.Error())
	}
	return response, nil
}

// Execute attestation request and return attestation response.
// Returns an error if the request is invalid (1st error),
// or if there is an error querying the database (2nd error)
func (qr *queryRouteHandlers) executePChainStakingRequest(
	request *api.ARPChainStaking,
) (*api.APIVerification[api.ARPChainStaking, api.DHPChainStaking], error, error) {
	if request.AttestationType != api.AttestationTypePChainStaking {
		return nil, errors.New("invalid attestation type: attestation type must be pchain staking"), nil
	}
	if int(request.SourceId) != qr.cfg.ChainID {
		return nil, errors.New("invalid source chain id: source chain id must be the same as the chain id of the indexer"), nil
	}

	// Ignore error, because it's already validated
	bytes, _ := globalUtils.DecodeHexString(request.Id)
	id, _ := ids.ToID(bytes)
	txID := id.String()

	tx, blockExists, err := database.FindPChainTxInBlockHeight(qr.db, txID, request.BlockNumber)
	if err != nil {
		return nil, nil, err
	}

	response := api.APIVerification[api.ARPChainStaking, api.DHPChainStaking]{}
	switch {
	case !blockExists:
		response.Status = api.VerificationStatusNonExistentBlock
	case tx == nil:
		response.Status = api.VerificationStatusNonExistentTransaction
	case tx.Type != database.PChainAddValidatorTx && tx.Type != database.PChainAddDelegatorTx:
		response.Status = api.VerificationStatusNonExistentTransaction
	default:
		var txType byte
		if tx.Type == database.PChainAddValidatorTx {
			txType = 0
		} else {
			txType = 1
		}

		// Ignore error, should be valid for add validator/delegator transactions
		nodeID, _ := globalUtils.NodeIDToHex(tx.NodeID)

		address, err := globalUtils.AddressToHex(tx.InputAddress)
		if err != nil {
			// Handle the case where the address is a node ID (genesis validator)
			address, err = globalUtils.IdToHex(tx.InputAddress)
			if err != nil {
				return nil, nil, errors.New("failed to convert address to hex")
			}
		}

		response.Status = api.VerificationStatusOK
		response.Request = request
		response.Response = &api.DHPChainStaking{
			BlockNumber:     request.BlockNumber,
			TransactionHash: request.Id,
			TransactionType: txType,
			NodeId:          nodeID,
			StartTime:       tx.StartTime.Unix(),
			EndTime:         tx.EndTime.Unix(),
			Weight:          tx.Weight,
			SourceAddress:   address,
		}
	}
	return &response, nil, nil
}
