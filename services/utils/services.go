package utils

import (
	"encoding/json"
	"flare-indexer/services/api"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
)

// Decode body from the request into value.
// Any error is written into the response and false is returned.
// (It is enough to just return from the request handler on false value)
func DecodeBody(w http.ResponseWriter, r *http.Request, value interface{}) bool {
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&value)
	if err != nil {
		WriteApiResponseError(w, api.ApiResStatusRequestBodyError,
			"error parsing request body", err.Error())
		return false
	}
	err = validate.Struct(value)
	if err != nil {
		WriteApiResponseError(w, api.ApiResStatusRequestBodyError,
			"error validating request body", err.Error())
		return false
	}
	return true
}

func DecodeQueryParams(w http.ResponseWriter, r *http.Request, value interface{}) bool {
	decoder := schema.NewDecoder()
	err := decoder.Decode(value, r.URL.Query())
	if err != nil {
		WriteApiResponseError(w, api.ApiResStatusInvalidRequest,
			"error parsing query params", err.Error())
		return false
	}
	err = validate.Struct(value)
	if err != nil {
		WriteApiResponseError(w, api.ApiResStatusRequestBodyError,
			"error validating query params", err.Error())
		return false
	}
	return true
}

// Write value into w as json. Handles possible error as internal server error
func WriteResponse(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(&value)
	if err != nil {
		http.Error(w, fmt.Sprint("error writing reponse: %w", err), http.StatusInternalServerError)
	}
}

// Writes value as data field in ApiResponse
// Handles error as internal server error
func WriteApiResponse[T any](w http.ResponseWriter, status api.ApiResStatusEnum, value T) {
	response := api.ApiResponseWrapper[T]{
		Status: status,
		Data:   value,
	}
	WriteResponse(w, response)
}

// Equivalent to WriteApiResponse with status ApiResponseStatusOk
func WriteApiResponseOk(w http.ResponseWriter, value any) {
	WriteApiResponse(w, api.ApiResStatusOk, value)
}

// Use for error responses
func WriteApiResponseError(
	w http.ResponseWriter,
	status api.ApiResStatusEnum,
	errorMessage string,
	errorDetails string,
) {
	response := api.ApiResponseWrapper[any]{
		Status:       status,
		ErrorDetails: errorDetails,
		ErrorMessage: errorMessage,
	}
	WriteResponse(w, response)
}

// Set InternalServerError to output if err is not nil. Return true if err is not nil
func HandleInternalServerError(w http.ResponseWriter, err error) bool {
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return true
	}
	return false
}

// Add route to router with path, handler, method, request and response types
func AddRoute(
	router *mux.Router,
	path string,
	f func(http.ResponseWriter, *http.Request),
	method string,
	requestType any, responseType any,
) {
	router.HandleFunc(path, f).Methods(method)
}
