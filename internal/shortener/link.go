package shortener

import "time"

type Link struct {
	URL       string
	CreatedAt time.Time
}
