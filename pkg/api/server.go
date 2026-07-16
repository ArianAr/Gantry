package api

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/ArianAr/Gantry/pkg/db"
	"github.com/ArianAr/Gantry/pkg/s3"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Options configures the HTTP server.
type Options struct {
	DB        *db.DB
	StaticFS  fs.FS // frontend dist (may be nil in tests)
	Mode      string
	Auth      AuthConfig
	Scheduler CronValidator // optional schedule validation
}

// NewRouter builds the Gin engine with API + optional SPA static hosting.
func NewRouter(opts Options) (*gin.Engine, *Server) {
	if opts.Mode == "" {
		opts.Mode = gin.ReleaseMode
	}
	gin.SetMode(opts.Mode)

	hub := NewHub()
	engine := s3.NewEngine(opts.DB, hub)
	srv := &Server{DB: opts.DB, Engine: engine, Hub: hub, Scheduler: opts.Scheduler}

	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger(), opts.Auth.Middleware())

	// Liveness: process is up
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	// Readiness: database reachable
	r.GET("/readyz", func(c *gin.Context) {
		if err := opts.DB.Ping(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
	// Prometheus metrics (unauthenticated; protect at network edge if needed)
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	api := r.Group("/api")
	srv.RegisterAPI(api)

	if opts.StaticFS != nil {
		// Serve embedded SPA with React-router fallback to index.html.
		fileServer := http.FileServer(http.FS(opts.StaticFS))
		r.NoRoute(func(c *gin.Context) {
			path := c.Request.URL.Path
			if strings.HasPrefix(path, "/api/") {
				c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
				return
			}
			// Try exact file
			clean := strings.TrimPrefix(path, "/")
			if clean == "" {
				clean = "index.html"
			}
			if f, err := opts.StaticFS.Open(clean); err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
			// SPA fallback
			c.Request.URL.Path = "/"
			fileServer.ServeHTTP(c.Writer, c.Request)
		})
	}

	return r, srv
}
