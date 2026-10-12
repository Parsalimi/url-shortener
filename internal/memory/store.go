package memory

import (
	"sync"
	"url-shortener/internal/shortener"
)

type InMemoryStore struct {
	mu         sync.RWMutex
	urlToCode  map[string]string
	codeToLink map[string]shortener.Link
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{urlToCode: make(map[string]string), codeToLink: make(map[string]shortener.Link)}
}

var _ shortener.Store = (*InMemoryStore)(nil)

func (m *InMemoryStore) FindByURL(url string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	value, ok := m.urlToCode[url]
	if !ok {
		return "", shortener.ErrNotFound
	}

	return value, nil
}

func (m *InMemoryStore) FindByCode(code string) (shortener.Link, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	link, ok := m.codeToLink[code]
	if !ok {
		return shortener.Link{}, shortener.ErrNotFound
	}

	return link, nil
}

func (m *InMemoryStore) Save(code string, link shortener.Link) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.codeToLink[code] = link
	m.urlToCode[link.URL] = code

	return nil
}
