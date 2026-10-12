package shortener_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"url-shortener/internal/shortener"
)

type fakeStore struct {
	mu sync.RWMutex

	urlToCode  map[string]string
	codeToLink map[string]shortener.Link

	findByURLError  error
	findByCodeError error
	saveError       error
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		urlToCode:  make(map[string]string),
		codeToLink: make(map[string]shortener.Link),
	}
}

func (f *fakeStore) FindByURL(url string) (string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

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
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.findByCodeError != nil {
		return shortener.Link{}, f.findByCodeError
	}
	link, ok := f.codeToLink[code]
	if !ok {
		return shortener.Link{}, shortener.ErrNotFound
	}

	return link, nil
}

func (f *fakeStore) Save(code string, link shortener.Link) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.saveError != nil {
		return f.saveError
	}
	f.codeToLink[code] = link
	f.urlToCode[link.URL] = code

	return nil
}

var _ shortener.Store = (*fakeStore)(nil)

func TestShorten(t *testing.T) {
	store := newFakeStore()
	s := shortener.New(store)
	code, err := s.Shorten("https://example.com")
	if err != nil {
		t.Fatalf("Shorten() returned an unexpected error: %v", err)
	}
	if code == "" {
		t.Error("Shorten() returned an empty code")
	}
}

func TestShortenSameURL(t *testing.T) {
	store := newFakeStore()
	s := shortener.New(store)

	code1, err := s.Shorten("https://example.com")
	if err != nil {
		t.Fatalf("first Shorten() returned an unexpected error: %v", err)
	}
	code2, err := s.Shorten("https://example.com")
	if err != nil {
		t.Fatalf("second Shorten() returned an unexpected error: %v", err)
	}
	if code1 != code2 {
		t.Errorf("same URL returned different codes: %q != %q", code1, code2)
	}
}

func TestShortenSavesLink(t *testing.T) {
	store := newFakeStore()
	s := shortener.New(store)

	code, err := s.Shorten("https://example.com")
	if err != nil {
		t.Fatalf("Shorten() returned an unexpected error: %v", err)
	}
	link, err := store.FindByCode(code)
	if err != nil {
		t.Fatalf("FindByCode() returned an unexpected error: %v", err)
	}
	if link.URL != "https://example.com" {
		t.Errorf("stored URL = %q, want %q", link.URL, "https://example.com")
	}
	if link.CreatedAt.IsZero() {
		t.Error("stored link has zero CreatedAt")
	}
}

func TestShortenEmptyURL(t *testing.T) {
	store := newFakeStore()
	s := shortener.New(store)

	_, err := s.Shorten("")

	if !errors.Is(err, shortener.ErrURLRequired) {
		t.Fatalf("Shorten() error = %v, want ErrURLRequired", err)
	}
}

func TestShortenInvalidURL(t *testing.T) {
	store := newFakeStore()
	s := shortener.New(store)

	invalidURLs := []string{
		"http:/example.com",
		"http://",
	}

	for _, url := range invalidURLs {
		_, err := s.Shorten(url)

		if !errors.Is(err, shortener.ErrInvalidURL) {
			t.Fatalf("Shorten(%q) error = %v, want ErrInvalidURL", url, err)
		}
	}
}

func TestShortenUnsupportedScheme(t *testing.T) {
	store := newFakeStore()
	s := shortener.New(store)

	urls := []string{
		"ftp://example.com",
		"postgres://admin@1234",
	}

	for _, url := range urls {
		_, err := s.Shorten(url)

		if !errors.Is(err, shortener.ErrUnsupportedScheme) {
			t.Fatalf(
				"Shorten(%q) error = %v, want ErrUnsupportedScheme",
				url,
				err,
			)
		}
	}
}

func TestShortenFindByURLError(t *testing.T) {
	store := newFakeStore()
	s := shortener.New(store)

	expectedErr := errors.New("store unavailable")
	store.findByURLError = expectedErr

	_, err := s.Shorten("https://example.com")

	if !errors.Is(err, expectedErr) {
		t.Fatalf("Shorten() error = %v, want %v", err, expectedErr)
	}
}

func TestShortenSaveError(t *testing.T) {
	store := newFakeStore()
	s := shortener.New(store)

	expectedErr := errors.New("store unavailable")
	store.saveError = expectedErr

	_, err := s.Shorten("https://example.com")

	if !errors.Is(err, expectedErr) {
		t.Fatalf("Shorten() error = %v, want %v", err, expectedErr)
	}
}

func TestRedirect(t *testing.T) {
	store := newFakeStore()
	s := shortener.New(store)

	expectedLink := shortener.Link{
		URL:       "https://example.com",
		CreatedAt: time.Now(),
	}

	if err := store.Save("abc123", expectedLink); err != nil {
		t.Fatalf("Save() returned an unexpected error: %v", err)
	}

	link, err := s.Redirect("abc123")
	if err != nil {
		t.Fatalf("Redirect() returned an unexpected error: %v", err)
	}

	if link.URL != expectedLink.URL {
		t.Errorf("Redirect() URL = %q, want %q", link.URL, expectedLink.URL)
	}

	if !link.CreatedAt.Equal(expectedLink.CreatedAt) {
		t.Errorf(
			"Redirect() CreatedAt = %v, want %v",
			link.CreatedAt,
			expectedLink.CreatedAt,
		)
	}
}

func TestRedirectNotFound(t *testing.T) {
	store := newFakeStore()
	s := shortener.New(store)

	_, err := s.Redirect("doesnotexist")

	if !errors.Is(err, shortener.ErrNotFound) {
		t.Fatalf("Redirect() error = %v, want ErrNotFound", err)
	}
}

func TestRedirectFindByCodeError(t *testing.T) {
	store := newFakeStore()
	s := shortener.New(store)

	expectedErr := errors.New("store unavailable")
	store.findByCodeError = expectedErr

	_, err := s.Redirect("abc123")

	if !errors.Is(err, expectedErr) {
		t.Fatalf("Redirect() error = %v, want %v", err, expectedErr)
	}
}

// ////// Concurrent duplicate shorten ////////
// Multiple concurrent requests for the same URL must still return the same code.
func TestConcurrentDuplicateShorten(t *testing.T) {
	store := newFakeStore()
	s := shortener.New(store)

	const goroutines = 1000

	codes := make(chan string, goroutines)
	errs := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			code, err := s.Shorten("https://example.com")

			codes <- code
			errs <- err
		}()
	}

	var expectedCode string

	for i := 0; i < goroutines; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("Shorten() returned an error: %v", err)
		}

		code := <-codes

		if expectedCode == "" {
			expectedCode = code
			continue
		}

		if code != expectedCode {
			t.Fatalf(
				"concurrent Shorten() returned different codes: %q != %q",
				expectedCode,
				code,
			)
		}
	}
}
