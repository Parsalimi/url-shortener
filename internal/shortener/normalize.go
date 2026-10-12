package shortener

import (
	"errors"
	"net/url"
	"strings"
)

var (
	ErrURLRequired       = errors.New("url is required")
	ErrInvalidURL        = errors.New("invalid URL")
	ErrUnsupportedScheme = errors.New("unsupported URL scheme")
)

func normalizeURL(rawURL string) (string, error) {
	if rawURL == "" {
		return "", ErrURLRequired
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return "", ErrInvalidURL
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return "", ErrUnsupportedScheme
	}

	if u.Hostname() == "" {
		return "", ErrInvalidURL
	}

	u.Scheme = strings.ToLower(u.Scheme)
	hostname := strings.ToLower(u.Hostname())
	port := u.Port()

	// Remove default ports
	switch u.Scheme {
	case "http":
		if port == "80" {
			port = ""
		}
	case "https":
		if port == "443" {
			port = ""
		}
	}

	// Rebuild Host with the normalized hostname and port.
	u.Host = hostname
	if port != "" {
		u.Host += ":" + port
	}

	return u.String(), nil
}
