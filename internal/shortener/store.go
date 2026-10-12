package shortener

import "errors"

type Store interface {
	FindByURL(url string) (string, error)
	FindByCode(code string) (Link, error)
	Save(code string, link Link) error
}

var ErrNotFound = errors.New("not found")
