package main

import (
	"flare-indexer/logger"
	"flare-indexer/services/context"
	"flare-indexer/services/routes"
	"flare-indexer/services/utils"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

func main() {
	ctx, err := context.BuildContext()
	if err != nil {
		log.Fatal(err) // logger possibly not initialized here so use builtin log
	}

	epochs, err := utils.NewEpochInfo(ctx)
	if err != nil {
		log.Fatal(err)
	}

	muxRouter := mux.NewRouter()
	router := utils.NewSwaggerRouter(muxRouter, "Flare P-Chain Indexer", "0.1.1")
	routes.AddTransferRoutes(router, ctx)
	routes.AddStakerRoutes(router, ctx)
	routes.AddTransactionRoutes(router, ctx, epochs)
	routes.AddMirroringRoutes(router, ctx, epochs)

	// Disabled -- state connector routes are currently not used
	// routes.AddQueryRoutes(router, ctx)

	router.Finalize()

	cors := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
	})
	corsMuxRouter := cors.Handler(muxRouter)
	address := ctx.Config().Services.Address
	srv := &http.Server{
		Handler: corsMuxRouter,
		Addr:    address,
		// Good practice: enforce timeouts for servers you create -- config?
		// WriteTimeout: 15 * time.Second,
		// ReadTimeout:  15 * time.Second,
	}

	cancelChan := make(chan os.Signal, 1)
	signal.Notify(cancelChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("Starting server on %s", address)
		err := srv.ListenAndServe()
		if err != nil {
			logger.Error("Server error: %v", err)
		}
	}()

	<-cancelChan
	logger.Info("Shutting down server")
}
