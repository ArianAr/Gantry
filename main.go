package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
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
	"github.com/ArianAr/Gantry/pkg/schedule"
)

//go:embed all:frontend/dist
var embeddedFrontend embed.FS

func main() {
	addr := flag.String("addr", envOr("GANTRY_ADDR", ":8080"), "HTTP listen address")
	dbPath := flag.String("db", envOr("GANTRY_DB", "gantry.db"), "SQLite database path")
	apiToken := flag.String("api-token", envOr("GANTRY_API_TOKEN", ""), "Shared API token (empty disables auth)")
	secretsKey := flag.String("secrets-key", envOr("GANTRY_SECRETS_KEY", ""), "Encrypt provider secrets at rest (empty = plaintext in DB)")
	trustProxy := flag.Bool("trust-proxy-headers", envBool("GANTRY_TRUST_PROXY_HEADERS", false), "Trust Remote-User / X-Remote-User from reverse proxy")
	logJSON := flag.Bool("log-json", envBool("GANTRY_LOG_JSON", false), "Emit logs as JSON lines to stdout")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("gantry %s (commit %s, built %s)\n", version.Version, version.Commit, version.BuildDate)
		os.Exit(0)
	}

	if *logJSON {
		log.SetFlags(0)
		log.SetOutput(jsonLogWriter{w: os.Stdout})
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

	sched := schedule.New(database, nil) // engine set after router construction
	router, apiSrv := api.NewRouter(api.Options{
		DB:        database,
		StaticFS:  staticRoot,
		Auth:      auth,
		Scheduler: sched,
	})
	sched.Engine = apiSrv.Engine
	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()
	sched.Start(rootCtx)

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
		log.Printf("Gantry %s listening on %s (db=%s, auth=%s, secrets=%s, scheduler=on)", version.Version, *addr, *dbPath, authMode, secMode)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	rootCancel()
	sched.Stop()
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

// jsonLogWriter wraps log output as {"ts","level","msg"} JSON lines.
type jsonLogWriter struct{ w io.Writer }

func (j jsonLogWriter) Write(p []byte) (int, error) {
	msg := strings.TrimSpace(string(p))
	line, _ := json.Marshal(map[string]string{
		"ts":    time.Now().UTC().Format(time.RFC3339Nano),
		"level": "info",
		"msg":   msg,
	})
	n, err := j.w.Write(append(line, '\n'))
	if err != nil {
		return 0, err
	}
	// Report original length so log package is satisfied.
	if n > 0 {
		return len(p), nil
	}
	return 0, err
}
