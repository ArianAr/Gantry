package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ArianAr/Gantry/internal/version"
	"github.com/ArianAr/Gantry/pkg/api"
	"github.com/ArianAr/Gantry/pkg/db"
)

//go:embed all:frontend/dist
var embeddedFrontend embed.FS

func main() {
	addr := flag.String("addr", envOr("GANTRY_ADDR", ":8080"), "HTTP listen address")
	dbPath := flag.String("db", envOr("GANTRY_DB", "gantry.db"), "SQLite database path")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("gantry %s (commit %s, built %s)\n", version.Version, version.Commit, version.BuildDate)
		os.Exit(0)
	}

	database, err := db.Open(*dbPath)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer database.Close()

	staticRoot, err := fs.Sub(embeddedFrontend, "frontend/dist")
	if err != nil {
		log.Fatalf("embed frontend: %v", err)
	}

	router, _ := api.NewRouter(api.Options{
		DB:       database,
		StaticFS: staticRoot,
	})

	srv := &http.Server{
		Addr:              *addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("Gantry %s listening on %s (db=%s)", version.Version, *addr, *dbPath)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	log.Printf("shutting down...")
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
