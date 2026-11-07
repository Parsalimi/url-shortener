# URL Shortener

A small HTTP URL-shortening service written in Go.

The project covers:

* HTTP handlers and REST API
* URL normalization and idempotency
* Domain errors and error handling
* Store abstraction and dependency injection
* Concurrent access and race detection
* Benchmarks and CPU profiling
* PostgreSQL persistence with GORM
* Persistence and restart testing

## Requirements

* Go 1.22+
* Docker and Docker Compose
* PostgreSQL (provided through Docker Compose)

## Running the Application

### 1. Start PostgreSQL

```bash
docker compose up -d
docker compose ps
```

The `shortener` database is created automatically by the PostgreSQL container.

### 2. Create the Test Database

Integration tests use a separate database named `shortener_test`.

The test database must be created manually because PostgreSQL's `POSTGRES_DB` setting is only applied when the database volume is initialized.

Connect to PostgreSQL:

```bash
docker compose exec postgres psql -U shortener -d postgres
```

Create the database:

```sql
CREATE DATABASE shortener_test OWNER shortener;
```

Exit `psql`:

```sql
\q
```

The database only needs to be created once.

To verify:

```bash
docker compose exec postgres psql -U shortener -d postgres -c "\l"
```

You should see both `shortener` and `shortener_test`.

The application's tables are created automatically by GORM migrations when the server starts.

## Configuration

The application supports two Store implementations:

```text
memory
postgres
```

Select the Store using the `STORE` environment variable.

Example `.env`:

```env
STORE=postgres
DATABASE_URL=postgres://shortener:shortener@localhost:5432/shortener
```

For the in-memory Store:

```env
STORE=memory
```

`DATABASE_URL` is required when using `STORE=postgres`.

## Start the Server

With PostgreSQL:

```bash
STORE=postgres \
DATABASE_URL=postgres://shortener:shortener@localhost:5432/shortener \
go run ./cmd/server -addr :8080 -base http://localhost:8080
```

If the environment is already configured:

```bash
go run ./cmd/server -addr :8080 -base http://localhost:8080
```

Default values:

```text
addr = :8080
base = http://localhost:8080
```

The `-base` value is used to construct the `short_url` returned by the API.

## API

### Create a Short URL

```http
POST /api/shorten
Content-Type: application/json
```

Request:

```json
{
  "url": "https://example.com"
}
```

Response:

```json
{
  "code": "Ab3xY7",
  "short_url": "http://localhost:8080/Ab3xY7"
}
```

### Redirect

```http
GET /{code}
```

Returns `302 Found` with the target URL in the `Location` header.

### Link Metadata

```http
GET /api/v1/links/{code}
```

Returns the original URL and creation timestamp.

Only `http` and `https` URLs are accepted. The service does not fetch submitted URLs.

## Testing

Run the test suite:

```bash
go test ./...
```

Run with the race detector:

```bash
go test -race ./...
```

Run static analysis:

```bash
go vet ./...
```

Generate coverage:

```bash
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

## Benchmarks

Run benchmarks:

```bash
go test -bench=. -benchtime=1000000x -benchmem ./internal/shortener/
```

Profile CPU usage:

```bash
go test -bench=BenchmarkShorten -cpuprofile=cpu.out ./internal/shortener/
go tool pprof cpu.out
```

The benchmark shows that short-link creation is significantly more expensive than redirect lookup, with random code generation and memory allocation/garbage collection being notable CPU costs.

## Design Decisions

Architectural and implementation decisions are documented in [`DECISIONS.md`](DECISIONS.md).
