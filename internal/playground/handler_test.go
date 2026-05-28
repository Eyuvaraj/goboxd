package playground_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/thesouldev/goboxd/internal/playground"
)

func TestHandler_ServesIndexHTML(t *testing.T) {
	h := playground.Handler()

	// http.FileServer redirects /index.html → /; request / directly.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /: want 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type: want text/html, got %q", ct)
	}
	if w.Body.Len() == 0 {
		t.Error("response body is empty")
	}
}

func TestHandler_ServesFavicon(t *testing.T) {
	h := playground.Handler()

	req := httptest.NewRequest(http.MethodGet, "/favicon.svg", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /favicon.svg: want 200, got %d", w.Code)
	}
}

func TestHandler_404OnUnknownPath(t *testing.T) {
	h := playground.Handler()

	req := httptest.NewRequest(http.MethodGet, "/does-not-exist.txt", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("GET /does-not-exist.txt: want 404, got %d", w.Code)
	}
}
