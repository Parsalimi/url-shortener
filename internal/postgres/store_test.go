package postgres_test

import (
	"errors"
	"testing"
	"time"
	"url-shortener/internal/postgres"
	"url-shortener/internal/shortener"
)

const testDSN = "postgres://shortener:shortener@localhost:5432/shortener_test"

func TestPersistenceAcrossNewApplication_Idempotency(t *testing.T) {
	db, err := postgres.Open(testDSN)
	if err != nil {
		t.Fatal(err)
	}

	if err := postgres.Migrate(db); err != nil {
		t.Fatal(err)
	}

	if err := db.Exec("TRUNCATE TABLE link_models").Error; err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		db.Exec("TRUNCATE TABLE link_models")
	})

	// APPLICATION #1
	store1 := postgres.NewPostgresStore(db)
	s1 := shortener.New(store1)

	code1, err := s1.Shorten("https://www.systemgroup.net/")
	if err != nil {
		t.Fatal(err)
	}

	link1, err := s1.Redirect(code1)
	if err != nil {
		t.Fatal(err)
	}

	// APPLICATION #1 DIES

	// APPLICATION #2
	store2 := postgres.NewPostgresStore(db)
	s2 := shortener.New(store2)

	code2, err := s2.Shorten("https://www.systemgroup.net/")
	if err != nil {
		t.Fatal(err)
	}

	link2, err := s2.Redirect(code1)
	if err != nil {
		t.Fatal(err)
	}

	if code1 != code2 {
		t.Fatal("idempotency across restart isn't working")
	}

	// APPLICATION #2 DIES

	// APPLICATION #3
	store3 := postgres.NewPostgresStore(db)
	s3 := shortener.New(store3)

	link3, err := s3.Redirect(code1)
	if err != nil {
		t.Fatal(err)
	}

	if link3.URL != "https://www.systemgroup.net/" {
		t.Fatal("persistence across restart isn't working")
	}

	if !link1.CreatedAt.Equal(link3.CreatedAt) {
		t.Fatal("persistence across restart isn't working")
	}

	if !link2.CreatedAt.Equal(link1.CreatedAt) {
		t.Fatalf("expected CreatedAt %v, got %v", link1.CreatedAt, link2.CreatedAt)
	}
}

func TestMigrate(t *testing.T) {
	db, err := postgres.Open(testDSN)
	if err != nil {
		t.Fatal(err)
	}

	if err := db.Migrator().DropTable(&postgres.LinkModel{}); err != nil {
		t.Fatal(err)
	}

	if err := postgres.Migrate(db); err != nil {
		t.Fatal(err)
	}

	if !db.Migrator().HasTable(&postgres.LinkModel{}) {
		t.Fatal("expected LinkModel table to exist after migration")
	}
}

func TestOpen_InvalidDSN(t *testing.T) {
	_, err := postgres.Open("invalid-dsn")

	if err == nil {
		t.Fatal("expected Open to return an error")
	}
}

func TestFindByURL_DatabaseError(t *testing.T) {
	db, err := postgres.Open(testDSN)
	if err != nil {
		t.Fatal(err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}

	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}

	store := postgres.NewPostgresStore(db)

	_, err = store.FindByURL("https://example.com")

	if err == nil {
		t.Fatal("expected database error")
	}

	if errors.Is(err, shortener.ErrNotFound) {
		t.Fatal("expected database error, got ErrNotFound")
	}
}

func TestFindByCode_DatabaseError(t *testing.T) {
	db, err := postgres.Open(testDSN)
	if err != nil {
		t.Fatal(err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}

	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}

	store := postgres.NewPostgresStore(db)

	_, err = store.FindByCode("does-not-exist")

	if err == nil {
		t.Fatal("expected database error")
	}

	if errors.Is(err, shortener.ErrNotFound) {
		t.Fatal("expected database error, got ErrNotFound")
	}
}

func TestSave_DatabaseError(t *testing.T) {
	db, err := postgres.Open(testDSN)
	if err != nil {
		t.Fatal(err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}

	store := postgres.NewPostgresStore(db)
	err = store.Save("abc123", shortener.Link{
		URL:       "https://example.com",
		CreatedAt: time.Now(),
	})
	if err == nil {
		t.Fatal("expected database error")
	}
}
