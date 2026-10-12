package shortener_test

import (
	"fmt"
	"sync/atomic"
	"testing"
	"url-shortener/internal/memory"
	"url-shortener/internal/shortener"
)

func BenchmarkShorten(b *testing.B) {
	store := memory.NewInMemoryStore()
	s := shortener.New(store)

	urls := make([]string, b.N)
	for i := 0; i < b.N; i++ {
		urls[i] = fmt.Sprintf("https://example.com/%d", i)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := s.Shorten(urls[i])
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRedirect(b *testing.B) {
	store := memory.NewInMemoryStore()
	s := shortener.New(store)

	code, err := s.Shorten("https://example.com")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := s.Redirect(code)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRedirectParallel(b *testing.B) {
	store := memory.NewInMemoryStore()
	s := shortener.New(store)

	code, err := s.Shorten("https://example.com")
	if err != nil {
		b.Fatal(err)
	}

	var failed atomic.Bool

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := s.Redirect(code)
			if err != nil {
				failed.Store(true)
				return
			}
		}
	})

	b.StopTimer()

	if failed.Load() {
		b.Fatal("Resolve failed")
	}
}
