package utils

import (
	"flare-indexer/logger"
	"flare-indexer/services/api"
	"log"
	"net/http"
	"strings"

	swagger "github.com/davidebianchi/gswagger"
	"github.com/davidebianchi/gswagger/apirouter"
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

// oasGorillaRouter wraps the gswagger gorilla/mux adapter so that path variables with a
// gorilla pattern, e.g. {tx_id:[0-9a-zA-Z]+}, are documented as plain OpenAPI template
// variables, e.g. {tx_id}. Swagger UI can only substitute a parameter into a plain
// template variable. The pattern is still used by gorilla/mux for routing.
type oasGorillaRouter struct {
	apirouter.Router[gorilla.HandlerFunc, gorilla.Route]
}

func newOasGorillaRouter(mRouter *mux.Router) apirouter.Router[gorilla.HandlerFunc, gorilla.Route] {
	return oasGorillaRouter{Router: gorilla.NewRouter(mRouter)}
}

func (r oasGorillaRouter) TransformPathToOasPath(path string) string {
	return gorillaPathToOasPath(path)
}

// gorillaPathToOasPath converts a gorilla/mux path to an OpenAPI path by removing the
// pattern part of path variables: {name:pattern} -> {name}. Braces inside patterns
// (e.g. {id:[0-9]{3}}) are matched.
func gorillaPathToOasPath(path string) string {
	var sb strings.Builder
	depth := 0
	inPattern := false
	for _, c := range path {
		switch c {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 && inPattern {
				inPattern = false
				sb.WriteRune(c)
				continue
			}
		case ':':
			if depth == 1 && !inPattern {
				inPattern = true
				continue
			}
		}
		if !inPattern {
			sb.WriteRune(c)
		}
	}
	return sb.String()
}

func NewSwaggerRouter(mRouter *mux.Router, title string, version string) Router {
	router, _ := swagger.NewRouter(newOasGorillaRouter(mRouter), swagger.Options{
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
	subRouter, _ := r.router.SubRouter(newOasGorillaRouter(mSubRouter), swagger.SubRouterOptions{
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
func NewRouteHandler[R interface{}, T interface{}](handler func(request R) (T, *ErrorHandler), method string, requestObject R, respObject T) RouteHandler {
	wrappedRespObject := api.ApiResponseWrapper[T]{Data: respObject}
	return wrappedRouteHandler(handler, func(w http.ResponseWriter, resp T) {
		WriteApiResponseOk(w, resp)
	}, method, requestObject, wrappedRespObject)
}

// Route handler factory
// Request passed to handler is the request body parsed to a struct of type R.
// The response of handler is an object of type T
// Openapi definitions are generated from the request and response objects
func NewClassicRouteHandler[R interface{}, T interface{}](handler func(request R) (T, *ErrorHandler), method string, requestObject R, respObject T) RouteHandler {
	return wrappedRouteHandler(handler, func(w http.ResponseWriter, resp T) {
		WriteResponse(w, resp)
	}, method, requestObject, respObject)
}

func wrappedRouteHandler[R interface{}, T interface{}, S interface{}](
	handler func(R) (T, *ErrorHandler),
	responseWriter func(http.ResponseWriter, T),
	method string,
	requestObject R,
	respObject S,
) RouteHandler {
	routeHandler := func(w http.ResponseWriter, r *http.Request) {
		var request R
		if !DecodeBody(w, r, &request) {
			return
		}
		resp, err := handler(request)
		if err != nil {
			err.Handler(w)
			return
		}
		responseWriter(w, resp)
	}
	swaggerDefinitions := swagger.Definitions{
		RequestBody: &swagger.ContentValue{
			Content: swagger.Content{
				"application/json": {Value: requestObject},
			},
		},
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
	routeHandler := func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		resp, err := handler(params)
		if err != nil {
			err.Handler(w)
			return
		}
		WriteApiResponseOk(w, resp)
	}
	pathParams := make(map[string]swagger.Parameter)
	for name, description := range paramDescriptions {
		pathParams[name] = swagger.Parameter{
			Schema:      &swagger.Schema{Value: ""},
			Description: description,
		}
	}
	wrappedRespObject := api.ApiResponseWrapper[T]{Data: respObject}
	swaggerDefinitions := swagger.Definitions{
		PathParams: pathParams,
		Responses: map[int]swagger.ContentValue{
			200: {
				Content: swagger.Content{
					"application/json": {Value: wrappedRespObject},
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
