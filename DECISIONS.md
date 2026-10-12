# Design decisions

## Dependencies

* `github.com/joho/godotenv` — loads configuration from a `.env` file during local development. I did not use the standard library because Go's standard library does not provide `.env` file parsing/loading; environment variables themselves are available through `os`, but `.env` support requires parsing an external file format.

## Part 1

* **URL normalization:** URLs are normalized inside `Shorten` before idempotency lookup. Scheme and hostname are lowercased, default ports are removed, and the root path `/` is treated as equivalent to an empty path. Query strings, fragments, non-default ports, and path case are preserved. Only `http` and `https` are accepted.

* **Idempotency:** The Store maintains indexes for normalized URL → code and code → `Link`. An existing normalized URL is found before generating a new code, ensuring the same URL always receives the same code.

* ***Code generation:** Codes use 6-character cryptographically secure random Base62 strings. Random codes were chosen over sequential IDs to avoid predictable codes and simple enumeration.

* **Collision handling:** Generated codes are checked against the Store before being saved. If a collision occurs, a new code is generated and checked again.

* **Shortener abstraction:** URL-shortening logic is encapsulated in a `Shortener` type with a `New()` constructor. This keeps application logic independent from the HTTP layer and allows the storage implementation to change without changing handlers.

* **Synchronization:** The in-memory Store uses `sync.RWMutex` because reads are expected to dominate writes. The shorten check → generate → save workflow is protected so concurrent requests for the same URL cannot create different codes.

* **HTTP routing:** Go's standard `http.NewServeMux` is used for routing and method matching, avoiding an additional routing dependency.

* **HTTP/domain boundary:** URL validation and normalization belong to the domain layer, while HTTP handlers translate domain errors into HTTP responses. The domain layer does not depend on HTTP status codes.

* **Public base URL:** The configured `base` URL belongs to the HTTP layer. The Shortener returns a code, and the HTTP layer combines it with `base` to construct the public `short_url`.

* **Package layout:** `cmd/server` contains application composition and HTTP setup. `internal/shortener` contains domain logic and the Store abstraction, while `internal/memory` and `internal/postgres` contain concrete Store implementations. `main` constructs the concrete Store and wires the application together.

## Part 2

* **Store interface location:** The `Store` interface is defined in `internal/shortener` because `Shortener` is its actual consumer. `main` only constructs the concrete Store and injects it into `Shortener`. This keeps persistence details out of the HTTP layer.

* **Store interface methods:** The interface contains only the operations required by `Shortener`:

  * `FindByURL` — find an existing code and preserve idempotency.
  * `FindByCode` — retrieve a link and check code collisions.
  * `Save` — persist a new link.

* **Store implementations:** The in-memory Store was refactored to implement the interface. A fake Store is used for unit tests, and PostgreSQL provides the persistent implementation in Part 4.

* **Domain errors:** Expected errors are represented by sentinel errors such as `ErrNotFound`, `ErrInvalidURL`, `ErrURLRequired`, and `ErrUnsupportedScheme`. Errors are wrapped with `%w` so their original identity is preserved.

* **HTTP error mapping:** The HTTP layer uses `errors.Is` to translate domain errors into HTTP responses:

  * URL validation errors → `400 Bad Request`
  * `ErrNotFound` → `404 Not Found`
  * unexpected errors → `500 Internal Server Error`

  Internal errors return a generic error message rather than exposing implementation details.

* **JSON:** The standard `encoding/json` package is used for request and response bodies. The metadata endpoint returns the original URL and `CreatedAt` timestamp in RFC3339 format.

* **Idempotency:** `Shortener.Shorten` searches the Store using the normalized URL before generating a code. Therefore, the same URL returns the same code regardless of the underlying Store implementation.

## Part 3

* **Locking choice:** The in-memory Store uses `sync.RWMutex` because the expected workload is read-heavy. I benchmarked `Mutex` and `RWMutex` instead of choosing based only on theory. `RWMutex` had slightly higher sequential read cost but performed significantly better for concurrent reads:

  * `Redirect`: 54.95 ns/op
  * `RedirectParallel`: 79.73 ns/op with `RWMutex` vs 136.3 ns/op with `Mutex`

  Therefore, read operations use `RLock`/`RUnlock`, while writes use `Lock`/`Unlock`.

* **Server timeouts:** Explicit HTTP server timeouts are configured to protect against slow clients:

  * `ReadHeaderTimeout`: 5s
  * `ReadTimeout`: 10s
  * `WriteTimeout`: 10s
  * `IdleTimeout`: 60s

  The values are intentionally conservative because the service handles small requests and responses.

* **Eviction / memory cap:** No eviction policy or maximum link count was implemented.

### Performance

Benchmarked with `go test -bench=. -benchtime=1000000x -benchmem ./internal/shortener/` on a 13th Gen Intel(R) Core(TM) i7-13620H.

* `BenchmarkShorten`: **3754 ns/op, 21 allocs/op**
* `BenchmarkRedirect`: **54.95 ns/op, 0 allocs/op**
* `BenchmarkRedirectParallel`: **79.73 ns/op, 0 allocs/op**

The redirect path is significantly cheaper than creating a short link, while concurrent reads benefit from `RWMutex`.

* **Profiling:** CPU profiling of `BenchmarkShorten` showed memory allocation/GC and cryptographically secure random code generation as notable costs. `randomSixCharBase62` accounted for 16.33% cumulative CPU time, with `crypto/rand.Int` accounting for 15.57%. This confirmed that code generation is a significant part of the shortening cost.

## Part 4

* **Storage choice:** PostgreSQL was chosen as the durable Store implementation, accessed through GORM. PostgreSQL provides durable transactional storage while GORM keeps the persistence implementation behind the existing `Store` interface.

* **Database environment:** PostgreSQL is run in a Docker container using Docker Compose rather than being installed directly on the host. This makes the database environment easier to reproduce across development machines, simplifies setup and cleanup, and avoids depending on a host-specific PostgreSQL installation. Docker Compose also provides a consistent way to configure the database, credentials, port mapping, and persistent storage. Application data is kept in a Docker volume so that restarting or recreating the container does not automatically discard the database contents. The Go application itself runs separately and connects to PostgreSQL through the configured `DATABASE_URL`.


* **Schema / model:** The PostgreSQL Store uses a `LinkModel` with:

  * `ID` — auto-incrementing primary key
  * `Code` — unique, non-null short code
  * `URL` — unique, non-null normalized URL
  * `CreatedAt` — non-null creation timestamp

  Both `Code` and `URL` have unique database indexes. These constraints provide database-level guarantees against duplicate codes and duplicate normalized URLs.

* **Migrations:** GORM's `AutoMigrate` is used to create and update the required schema. Migration runs during application startup before the PostgreSQL Store is used.

* **Startup / persistence:** When `STORE=postgres` is selected, the application opens the configured PostgreSQL database, runs the migration, and constructs a `PostgresStore`. Links are therefore loaded from persistent storage through the Store rather than reconstructed in memory.

* **Persist before success:** The application performs the database `Create` operation before returning the generated link from `Shorten`. The HTTP layer therefore cannot return `201 Created` until the persistence operation has succeeded. If the database operation fails, the error is propagated instead of returning a successful response.

  * PostgreSQL provides transactional durability for the committed insert. No separate in-memory write is treated as successful independently of the database.

* **`created_at` storage:** `CreatedAt` is represented as a Go `time.Time` and persisted through GORM. The API serializes it as RFC3339 in the metadata response, preserving the original creation timestamp across application restarts.

* **Idempotency across restarts:** The normalized URL is persisted together with its generated code. After a restart, a new `PostgresStore` can find the existing URL and return the same code instead of generating a new one. Therefore the Part 1 idempotency guarantee also holds across restarts.

* **Restart simulation:** Persistence is tested by creating one Store/Shortener instance, saving a link, discarding that instance, and creating a new Store/Shortener instance against the same PostgreSQL database. The second instance successfully retrieves the persisted link and returns the same code for the same URL. This simulates an application restart without requiring an actual process restart.

* **Store selection:** The application selects the storage implementation through the `STORE` environment variable. `STORE=memory` uses the in-memory implementation, while `STORE=postgres` uses PostgreSQL and requires `DATABASE_URL`.

* **Environment configuration:** `godotenv` is used to load configuration values from a local `.env` file during development. This keeps database credentials and other environment-specific settings out of the source code while still using environment variables as the application's configuration mechanism.

* **Testing the persistent Store:** PostgreSQL integration tests use a dedicated test database. Tests cover persistence across new Store instances, idempotency after restart, migration, successful persistence, missing records, and database-error paths. The persistent implementation is also exercised under `go test -race ./...`.

## Part 5

### Architecture

Part 5 changes the deployment model from a single application instance to a horizontally scalable architecture. The key architectural requirement is that application instances must be **stateless** with respect to authoritative link data, so any instance can handle any request.

```text
                                                    Internet
                                                        │
                                                        ▼
                                          ┌─────────────────────────┐
                                          │      Load Balancer      │
                                          │                         │
                                          │ Routing + Health Checks │
                                          └────────────┬────────────┘
                                                       │
                                        ┌──────────────┼──────────────┐
                                        │              │              │
                                        ▼              ▼              ▼
                                 ┌────────────┐ ┌────────────┐ ┌────────────┐
                                 │ Go App #1  │ │ Go App #2  │ │ Go App #N  │
                                 │            │ │            │ │            │
                                 │ HTTP       │ │ HTTP       │ │ HTTP       │
                                 │ Shortener  │ │ Shortener  │ │ Shortener  │
                                 │ Store      │ │ Store      │ │ Store      │
                                 └─────┬──────┘ └─────┬──────┘ └─────┬──────┘
                                       │              │              │
                                       └──────────────┼──────────────┘
                                                      │
                                                      ▼
                                           ┌─────────────────────┐
                                           │     PostgreSQL      │
                                           │                     │
                                           │ Authoritative Store │
                                           │  Links + Metadata   │
                                           └─────────────────────┘
```

#### Request flow

A client sends a request to the public endpoint rather than directly to an application instance. The load balancer selects one healthy application replica and forwards the request to it.

The selected Go application handles the HTTP request and delegates URL-shortening or lookup logic to `Shortener`. `Shortener` depends only on the `Store` interface, so it does not need to know whether the request is being handled by the first replica or the tenth.

For persistent deployment, the concrete Store is `PostgresStore`. Therefore, all application replicas read and write the same PostgreSQL database.

For example, a redirect request can be handled by any replica:

```text
Client
  │
  │ GET /abc123
  ▼
Load Balancer
  │
  ├──► App #1 ──┐
  │             │
  │             │
  │             │
  │             │
  └──► App #2 ──┤
                │
                ▼
           PostgreSQL
```

The important property is that `abc123` is not stored as authoritative state inside App #1 or App #2. Both instances can retrieve it from the shared Store.

#### Stateless application layer

The application layer is considered stateless because no replica owns persistent link state that another replica would need in order to process a request.

The authoritative state is:

```text
normalized URL → code
code → Link
```

and this state is persisted in PostgreSQL.

An individual application process may still have transient in-memory state, such as configuration, connection pools, or temporary request data. “Stateless” here means that such local state is not the source of truth for the application's persistent data.

Because replicas share the same Store, sticky sessions are unnecessary. A request can move from App #1 to App #2 and still access the same link data.

#### Application responsibilities

Each Go replica contains the same application layers as the current implementation:

```text
HTTP Handler
     │
     ▼
 Shortener
     │
     ▼
 Store interface
     │
     ▼
PostgresStore
     │
     ▼
 PostgreSQL
```

The load balancer is responsible only for traffic distribution and instance health. It does not implement URL-shortening logic or maintain link mappings.

`Shortener` remains responsible for domain behavior such as URL normalization, validation, code generation, and idempotency.

`PostgresStore` is responsible for persistence and database interaction.

This separation means horizontal scaling can happen without duplicating business logic or introducing a second application responsible for coordination.

#### Cross-replica consistency

The mutexes used by the in-memory implementation only synchronize goroutines inside a single process. They cannot coordinate App #1 with App #2.

In the multi-replica architecture, PostgreSQL therefore becomes the coordination point for persistent state.

Database constraints such as the unique constraints on `code` and normalized `url` provide cross-instance protection against duplicate records. The application must also handle database constraint conflicts correctly when concurrent requests from different replicas race to create the same logical link.

This is an important difference from the single-instance architecture:

```text
Single instance:
    goroutines
        │
        └── process-local synchronization

Multiple instances:
    App #1 ──┐
    App #2 ──┼──► PostgreSQL constraints / transactions
      .      .
      .      .
      .      .
    App #N ──┘
```

The application-level lock is therefore not considered a distributed lock. PostgreSQL is the authoritative synchronization boundary for shared persistent state.

#### Load balancing and health checks

The load balancer distributes requests among application replicas and performs health/readiness checks.

An instance that is unavailable or not ready to serve traffic should be removed from the load-balancing pool. This allows other replicas to continue serving requests while the failed instance is restarted or replaced.

The application therefore does not need to know which replica will receive a particular request.

#### Horizontal scaling

When request volume increases, additional identical Go replicas can be started:

```text
Before:

Load Balancer
      │
      ▼
    App #1
      │
      ▼
 PostgreSQL


After:

             ┌──► App #1 ──┐
             ├──► App #2 ──┤
Load Balancer┼──► App #3 ──┼──► PostgreSQL
             .      .      .
             .      .      .
             .      .      .
             └──► App #N ──┘
```

Adding replicas increases application-layer capacity without moving or copying link data between instances.

The main scaling limitation therefore moves from application CPU/concurrency to shared dependencies, particularly PostgreSQL, network capacity, and database connection capacity.

#### Deployment model

Docker can package the Go application as an identical deployable unit. Kubernetes can then manage multiple instances of that container.

A possible deployment is:

```text
                                                 Load Balancer
                                                       │
                                               Kubernetes Service
                                                       │
                                          ┌────────────┼────────────┐
                                          ▼            ▼            ▼
                                        Pod #1       Pod #2       Pod #N
                                        Go App       Go App       Go App
                                          │            │            │
                                          └────────────┼────────────┘
                                                       ▼
                                                   PostgreSQL
```

A Kubernetes `Deployment` can maintain the desired number of application replicas, while a `Service` provides stable routing to available Pods.

An HPA can increase or decrease the number of replicas based on configured resource or application metrics.

Kubernetes manages the application replicas; it does not automatically solve PostgreSQL scaling. The database remains a shared dependency that must be scaled or optimized independently when it becomes the bottleneck.

#### Observability and autoscaling

At multiple replicas, observing only one application's local behavior is insufficient. Metrics should be collected across instances, including:

* request rate
* request latency
* HTTP error rate
* CPU and memory usage
* database connection usage

A monitoring system such as Prometheus can collect metrics and Grafana can visualize them. Suitable metrics can also be exposed to Kubernetes autoscaling mechanisms.

This makes scaling a feedback loop:

```text
Traffic
   │
   ▼
Applications
   │
   ▼
Metrics
   │
   ▼
Monitoring / HPA
   │
   ▼
More or fewer replicas
```

#### Architectural tradeoffs

The main benefit of this architecture is that application capacity can scale horizontally while link state remains centralized and consistent in PostgreSQL.

The main tradeoff is that PostgreSQL becomes a shared dependency and therefore a potential bottleneck or single point of failure if it is deployed as only one database instance.

As traffic grows further, additional mechanisms can be introduced independently:

* read replicas for read-heavy workloads
* caching/CDN for frequently accessed redirects
* connection-pool tuning
* database partitioning or sharding
* write-path rate limiting or buffering

The central architectural decision is therefore to **scale the application horizontally while keeping persistent link state outside the application replicas**. This allows any healthy replica to serve any request and keeps the existing `Shortener → Store` abstraction reusable as the deployment grows.

#### CDN / Edge Caching

* **Purpose:** A CDN is placed in front of the load balancer to serve frequently requested redirects from edge locations. Cache hits avoid application and database work, reducing redirect latency and origin load.

* **Cache policy:** Successful redirect responses (`302 Found`) are cacheable for an initial TTL of 300 seconds. The origin communicates the intended freshness using `Cache-Control` headers, and the CDN is configured to honor the policy. Link-creation requests are not cached.

* **Cache key:** The cache key includes the public hostname and short-code path. Query parameters are included only if they affect the response, preventing unrelated requests from sharing an incorrect cached redirect.

* **Consistency and invalidation:** Link destinations are currently immutable through the API, which makes time-based caching suitable. If destination updates are introduced, cached responses may remain stale until expiry unless the CDN is explicitly invalidated.

* **Tradeoffs:** Edge caching reduces redirect latency and load on Go replicas and PostgreSQL, but cached responses can be stale and cache misses still reach the origin. The CDN is an optimization layer, not the authoritative source of link data; PostgreSQL remains the source of truth.

#### Write-path scaling: Rate limiting

* **Strategy:** Rate limiting is used to control incoming link-creation traffic and protect the application and PostgreSQL from excessive write requests. A queue or pre-generated code pool is not introduced because rate limiting is simpler and does not require asynchronous processing or additional code-allocation logic.

* **Scope and policy:** The limit applies to `POST /api/shorten`, with an initial policy of 10 requests per minute per client. Requests exceeding the limit receive `429 Too Many Requests`, with `Retry-After` provided when available. Redirect requests are not subject to this creation limit.

* **Placement:** In a multi-replica deployment, rate limiting must be enforced at a shared gateway or through shared rate-limit state. A process-local counter would apply separate limits to individual replicas and would not enforce a consistent per-client limit across the service.

* **Client identity:** The limiter uses a client identifier derived from a trusted source, such as the connecting IP address or a trusted proxy's forwarded-client information. Forwarded headers are trusted only when supplied by configured proxies.

* **Tradeoffs:** Rate limiting protects downstream resources by rejecting excess requests, but it does not increase database write capacity or buffer requests for later processing. The limit must be tuned to balance abuse protection with legitimate client traffic.

## Part 6
## URL and Domain Policy

The service accepts destination URLs using only the `http` and `https` schemes. URL validation is limited to syntactic validation and scheme checking; accepted URLs are normalized according to the application's URL normalization rules before storage and are later used as redirect destinations.

The service does not maintain a domain allowlist or blocklist, resolve destination domains for security checks, fetch destination URLs on the server, or use malware scanning, phishing detection, or external URL-reputation services. Consequently, it does not determine whether a destination is malicious or trustworthy and should not be considered a malicious-URL detection system.

Not fetching destination URLs during link creation avoids introducing SSRF risks through server-side requests made as part of the shortening operation. However, this does not guarantee that the destination itself is safe for users who follow the redirect.

If domain blocking is introduced in the future, it should be implemented as an explicit policy layer before persistence, with clearly defined rules for blocked domains and subdomains. A production implementation could use a maintained blocklist or an external URL-reputation service; this is outside the scope of the current project.
