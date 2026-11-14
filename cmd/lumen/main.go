package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jvanheerikhuize/lumen/internal/metadata"
	"github.com/jvanheerikhuize/lumen/internal/server"
	"github.com/jvanheerikhuize/lumen/internal/state"
	"github.com/jvanheerikhuize/lumen/web"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	dataDir := os.Getenv("LUMEN_DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	staticFS, err := web.DistFS()
	if err != nil {
		log.Fatalf("lumen: failed to initialize embedded web assets: %v", err)
	}

	store, err := state.NewStore(dataDir)
	if err != nil {
		log.Fatalf("lumen: failed to initialize state store: %v", err)
	}
	defer func() {
		_ = store.Close()
	}()

	srv, err := server.New(server.Config{
		SPAFS:       staticFS,
		StateStore:  store,
		TMDBService: metadata.NewTMDBService(os.Getenv("TMDB_API_KEY")),
	})
	if err != nil {
		log.Fatalf("lumen: failed to initialize server: %v", err)
	}

	httpServer := &http.Server{
		Addr:              ":" + port,
		Handler:           srv,
		ReadHeaderTimeout: 15 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("lumen: listening on http://0.0.0.0:%s (dataDir=%s)", port, dataDir)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("lumen: http server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Printf("lumen: shutting down gracefully...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
}
