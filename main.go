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
	"strings"
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
	apiToken := flag.String("api-token", envOr("GANTRY_API_TOKEN", ""), "Shared API token (empty disables auth)")
	secretsKey := flag.String("secrets-key", envOr("GANTRY_SECRETS_KEY", ""), "Encrypt provider secrets at rest (empty = plaintext in DB)")
	trustProxy := flag.Bool("trust-proxy-headers", envBool("GANTRY_TRUST_PROXY_HEADERS", false), "Trust Remote-User / X-Remote-User from reverse proxy")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("gantry %s (commit %s, built %s)\n", version.Version, version.Commit, version.BuildDate)
		os.Exit(0)
	}

	database, err := db.Open(*dbPath, *secretsKey)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer database.Close()

	staticRoot, err := fs.Sub(embeddedFrontend, "frontend/dist")
	if err != nil {
		log.Fatalf("embed frontend: %v", err)
	}

	auth := api.AuthConfig{
		Token:             *apiToken,
		TrustProxyHeaders: *trustProxy,
	}
	router, _ := api.NewRouter(api.Options{
		DB:       database,
		StaticFS: staticRoot,
		Auth:     auth,
	})

	srv := &http.Server{
		Addr:              *addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		authMode := "open"
		if auth.Token != "" && auth.TrustProxyHeaders {
			authMode = "token+proxy"
		} else if auth.Token != "" {
			authMode = "token"
		} else if auth.TrustProxyHeaders {
			authMode = "proxy-headers"
		}
		secMode := "plaintext"
		if *secretsKey != "" {
			secMode = "encrypted"
		}
		log.Printf("Gantry %s listening on %s (db=%s, auth=%s, secrets=%s)", version.Version, *addr, *dbPath, authMode, secMode)
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

func envBool(key string, fallback bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}
