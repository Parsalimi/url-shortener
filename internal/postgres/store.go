package postgres

import (
	"errors"
	"fmt"
	"time"
	"url-shortener/internal/shortener"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type LinkModel struct {
	ID        uint      `gorm:"primaryKey"`
	Code      string    `gorm:"uniqueIndex;not null"`
	URL       string    `gorm:"uniqueIndex;not null"`
	CreatedAt time.Time `gorm:"not null"`
}

func Open(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	return db, nil
}

type PostgresStore struct {
	db *gorm.DB
}

var _ shortener.Store = (*PostgresStore)(nil)

func NewPostgresStore(db *gorm.DB) *PostgresStore {
	return &PostgresStore{
		db: db,
	}
}

func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&LinkModel{})
}

func (p *PostgresStore) FindByURL(url string) (string, error) {
	var model LinkModel

	result := p.db.
		Where("url = ?", url).
		First(&model)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return "", shortener.ErrNotFound
	}

	if result.Error != nil {
		return "", fmt.Errorf("find link by URL: %w", result.Error)
	}

	return model.Code, nil
}

func (p *PostgresStore) FindByCode(code string) (shortener.Link, error) {
	var model LinkModel

	result := p.db.Where("code = ?", code).First(&model)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return shortener.Link{}, shortener.ErrNotFound
	}

	if result.Error != nil {
		return shortener.Link{}, fmt.Errorf("find link by code: %w", result.Error)
	}

	return shortener.Link{URL: model.URL, CreatedAt: model.CreatedAt}, nil
}

func (p *PostgresStore) Save(code string, link shortener.Link) error {
	model := LinkModel{Code: code, URL: link.URL, CreatedAt: link.CreatedAt}

	result := p.db.Create(&model)
	if result.Error != nil {
		return fmt.Errorf("save link: %w", result.Error)
	}

	return nil
}
