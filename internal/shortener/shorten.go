package shortener

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"
)

func randomSixCharBase62() (string, error) {
	// Somehow create a function to generate random 6-digit base62 code
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	res := make([]byte, 6)

	for i := 0; i < 6; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(62))
		if err != nil {
			return "", err
		}
		res[i] = alphabet[int(n.Int64())]
	}

	return string(res), nil
}

type Shortener struct {
	mu    sync.Mutex
	store Store
}

func New(store Store) *Shortener {
	return &Shortener{store: store}
}

func (s *Shortener) Shorten(longURL string) (code string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalizedURL, err := normalizeURL(longURL)
	// normalizeURL errors:
	// 1. ErrURLRequired
	// 2. ErrUnsupportedScheme
	// 3. ErrInvalidURL
	if err != nil {
		return "", fmt.Errorf("normalize URL: %w", err)
	}

	// Check if the normalizedURL already exists.
	code, err = s.store.FindByURL(normalizedURL)
	if err == nil {
		return code, nil
	}

	if !errors.Is(err, ErrNotFound) {
		return "", fmt.Errorf("find link by URL: %w", err)
	}

	for {
		code, err = randomSixCharBase62()
		if err != nil {
			return "", fmt.Errorf("generate short code: %w", err)
		}
		_, err := s.store.FindByCode(code)
		if errors.Is(err, ErrNotFound) {
			break
		}

		if err != nil {
			return "", fmt.Errorf("find link by Code: %w", err)
		}
	}

	err = s.store.Save(code, Link{URL: normalizedURL, CreatedAt: time.Now()})
	if err != nil {
		return "", fmt.Errorf("store Save: %w", err)
	}

	return code, nil
}

func (s *Shortener) Redirect(code string) (Link, error) {
	link, err := s.store.FindByCode(code)
	if err != nil {
		return Link{}, fmt.Errorf("find link by Code: %w", err)
	}
	return link, nil
}
