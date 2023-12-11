package utils

import (
	"flare-indexer/logger"
	"flare-indexer/services/api"
	"log"
	"net/http"

	swagger "github.com/davidebianchi/gswagger"
	"github.com/davidebianchi/gswagger/support/gorilla"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gorilla/mux"
	v3 "github.com/swaggest/swgui/v3"
)

type RouteHandler struct {
	Handler            func(w http.ResponseWriter, r *http.Request)
	SwaggerDefinitions swagger.Definitions
	Method             string
}

type ErrorHandler struct {
	Handler func(w http.ResponseWriter)
}

type Router interface {
	AddRoute(path string, handler RouteHandler, description ...string)
	WithPrefix(prefix string, tag string) Router
	Finalize()
}

// Default router implementation using gorilla/mux
type defaultRouter struct {
	router *mux.Router
}

func (r *defaultRouter) AddRoute(path string, handler RouteHandler, description ...string) {
	r.router.HandleFunc(path, handler.Handler).Methods(handler.Method)
}

func (r *defaultRouter) WithPrefix(prefix string, tag string) Router {
	return &defaultRouter{
		router: r.router.PathPrefix(prefix).Subrouter(),
	}
}

func (r *defaultRouter) Finalize() {
}

func NewDefaultRouter(mRouter *mux.Router) Router {
	return &defaultRouter{
		router: mRouter,
	}
}

// Router implementation with swagger support
type swaggerRouter struct {
	mRouter *mux.Router
	router  *swagger.Router[gorilla.HandlerFunc, *mux.Route]
	tag     string
}

func NewSwaggerRouter(mRouter *mux.Router, title string, version string) Router {
	router, _ := swagger.NewRouter(gorilla.NewRouter(mRouter), swagger.Options{
		Openapi: &openapi3.T{
			Info: &openapi3.Info{
				Title:   title,
				Version: version,
			},
		},
	})
	return &swaggerRouter{
		mRouter: mRouter,
		router:  router,
		tag:     "",
	}
}

// Add a route to the router and generate openapi definitions from the handler
// The first item in the description parameter is used to set the openapi summary field and
// the second item is used to set the openapi description field
func (r *swaggerRouter) AddRoute(path string, handler RouteHandler, description ...string) {
	swaggerDefinitions := handler.SwaggerDefinitions
	swaggerDefinitions.Tags = []string{r.tag}
	if len(description) > 0 {
		swaggerDefinitions.Summary = description[0]
		if len(description) > 1 {
			swaggerDefinitions.Description = description[1]
		}
	}

	_, err := r.router.AddRoute(handler.Method, path, handler.Handler, swaggerDefinitions)
	if err != nil {
		log.Fatal(err)
	}
}

func (r *swaggerRouter) WithPrefix(prefix string, tag string) Router {
	mSubRouter := r.mRouter.NewRoute().Subrouter()
	subRouter, _ := r.router.SubRouter(gorilla.NewRouter(mSubRouter), swagger.SubRouterOptions{
		PathPrefix: prefix,
	})
	return &swaggerRouter{
		mRouter: mSubRouter,
		router:  subRouter,
		tag:     tag,
	}
}

func (r *swaggerRouter) Finalize() {
	if err := r.router.GenerateAndExposeOpenapi(); err != nil {
		log.Fatal(err)
	}

	handler := v3.NewHandler("Flare P-chain indexer API", "/documentation/json", "/swagger")
	r.mRouter.PathPrefix("/swagger").HandlerFunc(handler.ServeHTTP)
}

// Route handler factory
// Request passed to handler is the request body parsed to a struct of type R. The response of handler is wrapped to
// an ApiResponseWrapper object and returned as json
// Openapi definitions are generated from the request and response objects
func NewBodyRouteHandler[B interface{}, R interface{}](handler func(request B) (R, *ErrorHandler), method string, requestObject B, respObject R) RouteHandler {
	generalHandler := func(pathParams map[string]string, query interface{}, request B) (R, *ErrorHandler) {
		return handler(request)
	}
	return NewGeneralRouteHandler(generalHandler, http.MethodPost, nil, nil, requestObject, respObject)
}

// Route handler factory
// The value passed to handler are the path parameters parsed to a map of string. The response of handler is wrapped to
// an ApiResponseWrapper object and returned as json. Openapi definitionas for the path parameters are generated from the
// paramDescriptions map, definitions for the response object are generated from the response object.
func NewParamRouteHandler[T interface{}](
	handler func(params map[string]string) (T, *ErrorHandler),
	method string,
	paramDescriptions map[string]string,
	respObject T,
) RouteHandler {
	generalHandler := func(pathParams map[string]string, query interface{}, request interface{}) (T, *ErrorHandler) {
		return handler(pathParams)
	}
	return NewGeneralRouteHandler(generalHandler, method, paramDescriptions, nil, nil, respObject)
}

// Route handler factory
// The value passed to handler are the path parameters parsed to a map of string, the query parameters parsed to a struct
// of type Q and the request body parsed to a struct of type B. The response of handler is wrapped to an
// ApiResponseWrapper object and returned as json. Openapi definitions for the path parameters are generated from the
// paramDescriptions map, definitions for the query parameters are generated from the queryObject and definitions for the
// request body are generated from the bodyObject.
func NewGeneralRouteHandler[Q interface{}, B interface{}, R interface{}](
	handler func(map[string]string, Q, B) (R, *ErrorHandler),
	method string,
	paramDescriptions map[string]string, // Path params descriptions for openapi
	queryObject Q,
	bodyObject B,
	respObject R,
) RouteHandler {
	routeHandler := func(w http.ResponseWriter, r *http.Request) {
		var body B
		if !IsNil(bodyObject) && !DecodeBody(w, r, &body) {
			return
		}
		var query Q
		if !IsNil(queryObject) && !DecodeQueryParams(w, r, &query) {
			return
		}
		params := mux.Vars(r)

		resp, err := handler(params, query, body)
		if err != nil {
			err.Handler(w)
			return
		}
		WriteApiResponseOk(w, resp)
	}
	pathParams := createPathParamsDescription(paramDescriptions)
	querystring := createQueryDescription(queryObject)
	requestBody := createRequestBodyDescription(bodyObject)

	swaggerDefinitions := swagger.Definitions{
		RequestBody: requestBody,
		PathParams:  pathParams,
		Querystring: querystring,
		Responses: map[int]swagger.ContentValue{
			200: {
				Content: swagger.Content{
					"application/json": {Value: respObject},
				},
			},
		},
	}
	return RouteHandler{
		Handler:            routeHandler,
		SwaggerDefinitions: swaggerDefinitions,
		Method:             method,
	}
}

func InternalServerErrorHandler(err error) *ErrorHandler {
	return &ErrorHandler{
		Handler: func(w http.ResponseWriter) {
			logger.Error("Internal error: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		},
	}
}

func HttpErrorHandler(code int, err string) *ErrorHandler {
	return &ErrorHandler{
		Handler: func(w http.ResponseWriter) {
			http.Error(w, err, code)
		},
	}
}

func ApiResponseErrorHandler(
	status api.ApiResStatusEnum,
	errorMessage string,
	errorDetails string,
) *ErrorHandler {
	return &ErrorHandler{
		Handler: func(w http.ResponseWriter) {
			WriteApiResponseError(w, status, errorMessage, errorDetails)
		},
	}
}

func createPathParamsDescription(paramDescriptions map[string]string) map[string]swagger.Parameter {
	if len(paramDescriptions) == 0 {
		return nil
	}

	pathParams := make(map[string]swagger.Parameter)
	for name, description := range paramDescriptions {
		pathParams[name] = swagger.Parameter{
			Schema:      &swagger.Schema{Value: ""},
			Description: description,
		}
	}
	return pathParams
}

func createQueryDescription(queryObject interface{}) swagger.ParameterValue {
	if queryObject == nil {
		return nil
	}
	fields := StructFields(queryObject)
	if len(fields) == 0 {
		return nil
	}

	querystring := make(swagger.ParameterValue)
	for _, field := range fields {
		name := field.Tag.Get("json")
		if name == "" {
			name = field.Name
		}
		querystring[name] = swagger.Parameter{
			Schema:      &swagger.Schema{Value: ""},
			Description: field.Tag.Get("jsonschema"),
		}
	}
	return querystring
}

func createRequestBodyDescription(bodyObject interface{}) *swagger.ContentValue {
	if bodyObject == nil {
		return nil
	}
	return &swagger.ContentValue{
		Content: swagger.Content{
			"application/json": {Value: bodyObject},
		},
	}
}
