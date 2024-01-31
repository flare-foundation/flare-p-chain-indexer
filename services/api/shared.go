package api

import "flare-indexer/database"

type ApiResStatusEnum string

const (
	ApiResStatusOk                ApiResStatusEnum = "OK"
	ApiResStatusError             ApiResStatusEnum = "ERROR"
	ApiResStatusRequestBodyError  ApiResStatusEnum = "REQUEST_BODY_ERROR"
	ApiResStatusValidationError   ApiResStatusEnum = "VALIDATION_ERROR"
	ApiResStatusTooManyRequests   ApiResStatusEnum = "TOO_MANY_REQUESTS"
	ApiResStatusUnauthorized      ApiResStatusEnum = "UNAUTHORIZED"
	ApiResStatusAuthError         ApiResStatusEnum = "AUTH_ERROR"
	ApiResStatusUpstreamHttpError ApiResStatusEnum = "UPSTREAM_HTTP_ERROR"
	ApiResStatusInvalidRequest    ApiResStatusEnum = "INVALID_REQUEST"
	ApiResStatusNotImplemented    ApiResStatusEnum = "NOT_IMPLEMENTED"
	ApiResStatusPending           ApiResStatusEnum = "PENDING"
)

type ApiResponseWrapper[T any] struct {
	Data T `json:"data"`

	// Optional details for unexpected error responses.
	ErrorDetails string `json:"errorDetails"`

	// Simple message to explain client developers the reason for error.
	ErrorMessage string `json:"errorMessage"`

	// Response status. OK for successful responses.
	Status ApiResStatusEnum `json:"status"`

	ValidationErrorDetails *ApiValidationErrorDetails `json:"validationErrorDetails"`
}

type ApiValidationErrorDetails struct {
	ClassName   string            `json:"className"`
	FieldErrors map[string]string `json:"fieldErrors"`
}

type ApiAddress struct {
	EthAddress  string `json:"ethAddress"`
	BechAddress string `json:"bechAddress"`
}

func NewApiAddress(address *database.Address) *ApiAddress {
	if address == nil {
		return nil
	} else {
		return &ApiAddress{
			EthAddress:  "0x" + address.EthAddress,
			BechAddress: address.BechAddress,
		}
	}
}
