package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ArianAr/Gantry/pkg/db"
)

func TestMetricsEndpointOpen(t *testing.T) {
	pathDB := filepath.Join(t.TempDir(), "t.db")
	database, err := db.Open(pathDB)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	r, _ := NewRouter(Options{
		DB: database, Mode: "test",
		Auth: AuthConfig{Token: "secret"},
	})
	// /metrics must work without token
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("metrics status %d body %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "gantry_jobs_started_total") {
		t.Fatalf("missing metric in body: %s", body[:min(200, len(body))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
