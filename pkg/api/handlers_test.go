package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/ArianAr/Gantry/pkg/db"
)

func setupTestServer(t *testing.T) (*Server, *httptest.ResponseRecorder, func(method, path string, body any) *httptest.ResponseRecorder) {
	t.Helper()
	pathDB := filepath.Join(t.TempDir(), "t.db")
	database, err := db.Open(pathDB)
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	r, srv := NewRouter(Options{DB: database, Mode: "test"})
	do := func(method, path string, body any) *httptest.ResponseRecorder {
		var buf bytes.Buffer
		if body != nil {
			_ = json.NewEncoder(&buf).Encode(body)
		}
		req := httptest.NewRequest(method, path, &buf)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	return srv, nil, do
}

func TestProvidersCRUD(t *testing.T) {
	_, _, do := setupTestServer(t)

	w := do("POST", "/api/providers", map[string]any{
		"name": "local", "provider_type": "minio", "endpoint": "http://127.0.0.1:9000",
		"region": "us-east-1", "access_key_id": "ak", "secret_access_key": "sk-secret",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create status %d body %s", w.Code, w.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	if created["secret_access_key"] != "********" {
		t.Fatalf("secret not redacted: %v", created["secret_access_key"])
	}
	id, _ := created["id"].(string)

	w = do("GET", "/api/providers", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list %d", w.Code)
	}

	w = do("DELETE", "/api/providers/"+id, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete %d %s", w.Code, w.Body.String())
	}
}

func TestRulesAndVersion(t *testing.T) {
	_, _, do := setupTestServer(t)

	// Create two providers
	mk := func(name string) string {
		w := do("POST", "/api/providers", map[string]any{
			"name": name, "provider_type": "aws", "region": "us-east-1",
			"access_key_id": "a", "secret_access_key": "b",
		})
		var m map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &m)
		return m["id"].(string)
	}
	src := mk("src")
	dst := mk("dst")

	w := do("POST", "/api/rules", map[string]any{
		"name": "pipe", "source_provider_id": src, "source_bucket": "in",
		"target_provider_id": dst, "target_bucket": "out", "concurrency_limit": 8,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("rule %d %s", w.Code, w.Body.String())
	}

	w = do("GET", "/api/rules", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list rules %d", w.Code)
	}

	w = do("GET", "/api/version", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("version %d", w.Code)
	}

	w = do("GET", "/healthz", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("healthz %d", w.Code)
	}
}
