package memory_test

import (
	"errors"
	"testing"
	"time"

	"url-shortener/internal/memory"
	"url-shortener/internal/shortener"
)

func TestInMemoryStore_SaveAndFindByURL(t *testing.T) {
	store := memory.NewInMemoryStore()

	link := shortener.Link{
		URL:       "https://example.com",
		CreatedAt: time.Now(),
	}

	err := store.Save("abc123", link)
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	code, err := store.FindByURL(link.URL)
	if err != nil {
		t.Fatalf("FindByURL() returned error: %v", err)
	}

	if code != "abc123" {
		t.Fatalf("FindByURL() returned code %q, want %q", code, "abc123")
	}
}

func TestInMemoryStore_SaveAndFindByCode(t *testing.T) {
	store := memory.NewInMemoryStore()

	link := shortener.Link{
		URL:       "https://example.com",
		CreatedAt: time.Now(),
	}

	err := store.Save("abc123", link)
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	got, err := store.FindByCode("abc123")
	if err != nil {
		t.Fatalf("FindByCode() returned error: %v", err)
	}

	if got.URL != link.URL {
		t.Fatalf("FindByCode() returned URL %q, want %q", got.URL, link.URL)
	}

	if !got.CreatedAt.Equal(link.CreatedAt) {
		t.Fatalf(
			"FindByCode() returned CreatedAt %v, want %v",
			got.CreatedAt,
			link.CreatedAt,
		)
	}
}

func TestInMemoryStore_FindByURL_NotFound(t *testing.T) {
	store := memory.NewInMemoryStore()

	_, err := store.FindByURL("https://does-not-exist.example")
	if !errors.Is(err, shortener.ErrNotFound) {
		t.Fatalf("FindByURL() error = %v, want %v", err, shortener.ErrNotFound)
	}
}

func TestInMemoryStore_FindByCode_NotFound(t *testing.T) {
	store := memory.NewInMemoryStore()

	_, err := store.FindByCode("doesnotexist")
	if !errors.Is(err, shortener.ErrNotFound) {
		t.Fatalf("FindByCode() error = %v, want %v", err, shortener.ErrNotFound)
	}
}

func TestInMemoryStore_SaveMultipleLinks(t *testing.T) {
	store := memory.NewInMemoryStore()

	links := []struct {
		code string
		link shortener.Link
	}{
		{
			code: "abc123",
			link: shortener.Link{
				URL:       "https://example.com",
				CreatedAt: time.Now(),
			},
		},
		{
			code: "xyz789",
			link: shortener.Link{
				URL:       "https://google.com",
				CreatedAt: time.Now(),
			},
		},
	}

	for _, tc := range links {
		if err := store.Save(tc.code, tc.link); err != nil {
			t.Fatalf("Save(%q) returned error: %v", tc.code, err)
		}
	}

	for _, tc := range links {
		code, err := store.FindByURL(tc.link.URL)
		if err != nil {
			t.Fatalf("FindByURL(%q) returned error: %v", tc.link.URL, err)
		}

		if code != tc.code {
			t.Fatalf(
				"FindByURL(%q) returned code %q, want %q",
				tc.link.URL,
				code,
				tc.code,
			)
		}

		got, err := store.FindByCode(tc.code)
		if err != nil {
			t.Fatalf("FindByCode(%q) returned error: %v", tc.code, err)
		}

		if got.URL != tc.link.URL {
			t.Fatalf(
				"FindByCode(%q) returned URL %q, want %q",
				tc.code,
				got.URL,
				tc.link.URL,
			)
		}
	}
}
