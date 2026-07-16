package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/ArianAr/Gantry/pkg/db"
)

func TestAuthDisabledOpen(t *testing.T) {
	pathDB := filepath.Join(t.TempDir(), "t.db")
	database, err := db.Open(pathDB)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	r, _ := NewRouter(Options{DB: database, Mode: "test", Auth: AuthConfig{}})
	req := httptest.NewRequest(http.MethodGet, "/api/providers", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
}

func TestAuthTokenRequired(t *testing.T) {
	pathDB := filepath.Join(t.TempDir(), "t.db")
	database, err := db.Open(pathDB)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	r, _ := NewRouter(Options{
		DB: database, Mode: "test",
		Auth: AuthConfig{Token: "secret-token"},
	})

	// health open
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("healthz %d", w.Code)
	}

	// no creds
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/providers", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	// bearer
	req := httptest.NewRequest(http.MethodGet, "/api/providers", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("bearer %d body %s", w.Code, w.Body.String())
	}

	// api key
	req = httptest.NewRequest(http.MethodGet, "/api/providers", nil)
	req.Header.Set("X-API-Key", "secret-token")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("api key %d", w.Code)
	}

	// query (SSE)
	req = httptest.NewRequest(http.MethodGet, "/api/jobs/stream?access_token=secret-token", nil)
	w = httptest.NewRecorder()
	// stream will hang if fully connected; just check we don't get 401 immediately
	// Use version endpoint with query instead
	req = httptest.NewRequest(http.MethodGet, "/api/version?access_token=secret-token", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("query token %d", w.Code)
	}

	// wrong token
	req = httptest.NewRequest(http.MethodGet, "/api/providers", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token %d", w.Code)
	}
}

func TestAuthProxyHeader(t *testing.T) {
	pathDB := filepath.Join(t.TempDir(), "t.db")
	database, err := db.Open(pathDB)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	r, _ := NewRouter(Options{
		DB: database, Mode: "test",
		Auth: AuthConfig{TrustProxyHeaders: true},
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/providers", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without header, got %d", w.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/providers", nil)
	req.Header.Set("X-Remote-User", "alice")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("proxy user %d", w.Code)
	}
}
