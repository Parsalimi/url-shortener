package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"url-shortener/internal/shortener"
)

type fakeStore struct {
	findByURLError error

	urlToCode  map[string]string
	codeToLink map[string]shortener.Link
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		urlToCode:  make(map[string]string),
		codeToLink: make(map[string]shortener.Link),
	}
}

func (f *fakeStore) FindByURL(url string) (string, error) {
	if f.findByURLError != nil {
		return "", f.findByURLError
	}
	code, ok := f.urlToCode[url]
	if !ok {
		return "", shortener.ErrNotFound
	}

	return code, nil
}

func (f *fakeStore) FindByCode(code string) (shortener.Link, error) {
	link, ok := f.codeToLink[code]
	if !ok {
		return shortener.Link{}, shortener.ErrNotFound
	}

	return link, nil
}

func (f *fakeStore) Save(code string, link shortener.Link) error {
	f.codeToLink[code] = link
	f.urlToCode[link.URL] = code
	return nil
}

func newTestServer() *Server {
	store := newFakeStore()

	return &Server{
		shortener: shortener.New(store),
		base:      "http://localhost:8080",
	}
}

func newTestMux(srv *Server) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/shorten", srv.shortenHandler)
	mux.HandleFunc("GET /{code}", srv.redirectHandler)
	mux.HandleFunc("GET /api/v1/links/{code}", srv.linkHandler)

	return mux
}

// POST /api/shorten
func TestShortenHandler(t *testing.T) {
	srv := newTestServer()
	mux := newTestMux(srv)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/shorten",
		strings.NewReader(`{"url":"https://example.com"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}

	if contentType := rec.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", contentType)
	}

	if !strings.Contains(rec.Body.String(), `"code"`) {
		t.Error("response does not contain code")
	}

	if !strings.Contains(rec.Body.String(), `"short_url"`) {
		t.Error("response does not contain short_url")
	}
}

// Bad JSON
func TestShortenHandlerBadJSON(t *testing.T) {
	srv := newTestServer()
	mux := newTestMux(srv)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/shorten",
		strings.NewReader(`{"url":`),
	)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// Bad URL
func TestShortenHandlerBadURL(t *testing.T) {
	srv := newTestServer()
	mux := newTestMux(srv)

	testCases := []string{
		`{"url":""}`,
		`{"url":"http://"}`,
		`{"url":"ftp://example.com"}`,
	}

	for _, body := range testCases {
		t.Run(body, func(t *testing.T) {
			req := httptest.NewRequest(
				http.MethodPost,
				"/api/shorten",
				strings.NewReader(body),
			)

			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf(
					"status = %d, want %d",
					rec.Code,
					http.StatusBadRequest,
				)
			}
		})
	}
}

// GET /{code}
// ////// Redirect HTTP test ////////
func TestRedirectHandler(t *testing.T) {
	store := newFakeStore()

	code := "abc123"
	link := shortener.Link{
		URL: "https://example.com",
	}

	if err := store.Save(code, link); err != nil {
		t.Fatalf("Save() returned an unexpected error: %v", err)
	}

	srv := &Server{
		shortener: shortener.New(store),
		base:      "http://localhost:8080",
	}

	mux := newTestMux(srv)
	req := httptest.NewRequest(
		http.MethodGet,
		"/"+code,
		nil,
	)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}

	if location := rec.Header().Get("Location"); location != link.URL {
		t.Errorf("Location = %q, want %q", location, link.URL)
	}
}

// Unknown code
func TestRedirectHandlerUnknownCode(t *testing.T) {
	srv := newTestServer()
	mux := newTestMux(srv)

	req := httptest.NewRequest(
		http.MethodGet,
		"/doesnotexist",
		nil,
	)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// GET /api/v1/links/{code}
// ////// Metadata endpoint ////////
func TestLinkHandler(t *testing.T) {
	store := newFakeStore()

	code := "abc123"
	link := shortener.Link{
		URL:       "https://example.com",
		CreatedAt: time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
	}

	if err := store.Save(code, link); err != nil {
		t.Fatalf("Save() returned an unexpected error: %v", err)
	}

	srv := &Server{
		shortener: shortener.New(store),
		base:      "http://localhost:8080",
	}
	mux := newTestMux(srv)
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/links/"+code,
		nil,
	)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()

	if !strings.Contains(body, `"url":"https://example.com"`) {
		t.Errorf("response missing URL: %s", body)
	}

	if !strings.Contains(body, `"created_at":"2026-01-15T12:00:00Z"`) {
		t.Errorf("response missing created_at: %s", body)
	}
}

func TestLinkHandlerUnknownCode(t *testing.T) {
	srv := newTestServer()
	mux := newTestMux(srv)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/links/doesnotexist",
		nil,
	)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestInternalFailure(t *testing.T) {
	store := &fakeStore{
		findByURLError: errors.New("database unavailable"),
	}

	s := shortener.New(store)
	srv := Server{
		shortener: s,
		base:      "http://localhost:8080",
	}
	body := strings.NewReader(`{"url":"https://example.com"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.shortenHandler(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rr.Code)
	}
}

// ////// Store idempotency through HTTP ////////
func TestShortenHandlerIdempotency(t *testing.T) {
	srv := newTestServer()
	mux := newTestMux(srv)

	body := `{"url":"https://example.com"}`
	var firstCode string

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/shorten",
			strings.NewReader(body),
		)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
		}
		response := rec.Body.String()
		const prefix = `{"code":"`
		start := strings.Index(response, prefix)
		if start == -1 {
			t.Fatalf("response does not contain code: %s", response)
		}

		start += len(prefix)

		end := strings.Index(response[start:], `"`)
		if end == -1 {
			t.Fatalf("could not parse code: %s", response)
		}

		code := response[start : start+end]

		if i == 0 {
			firstCode = code
		} else if code != firstCode {
			t.Fatalf(
				"same URL returned different codes: %q != %q",
				firstCode,
				code,
			)
		}
	}
}
