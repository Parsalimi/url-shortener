# Checklist

## Part 1
| Done | Pts | Requirement |
|:----:|:---:|-------------|
| ✅ | 4 | `POST /api/shorten` returns **201** with `code` and `short_url` |
| ✅ | 3 | **Idempotency:** second `POST` with the **same URL** returns the **same** `code` (test required) |
| ✅ | 4 | `GET /{code}` returns **302** with correct `Location` |
| ✅ | 3 | Unknown code → **404**; bad/missing URL → **400** |
| ✅ | 3 | URL validation and no server-side fetch of long URL |
| ✅ | 2 | Codes 6–8 chars for new URLs; collision strategy for **new** codes only |
| ✅ | 2 | `-base` flag used for `short_url` |
| ✅ | 2 | `httptest`: shorten + redirect; table tests for bad URL and unknown code |
| ✅ | 2 | Concurrent test (include concurrent duplicate shorten for same URL); **`go test -race ./...`** passes |

## Part 2
| Done | Pts | Requirement |
|:----:|:---:|-------------|
| ✅ | 5 | Metadata route **200** / **404** with correct JSON |
| ✅ | 4 | `ErrNotFound`, `ErrInvalidURL` from store/domain |
| ✅ | 4 | `%w` + `errors.Is` in HTTP mapping |
| ✅ | 5 | `Store` interface + fake used in tests |
| ✅ | 4 | Tests for metadata route and error mapping |
| ✅ | 3 | Test or note in README: idempotency still works via `Store` / HTTP after Part 2 changes |

## Part 3
| Done | Pts | Requirement |
|:----:|:---:|-------------|
| ✅ | 5 | Server timeouts configured |
| ✅ | 5 | `Mutex` vs `RWMutex` matches behavior and `DECISIONS.md` |
| ✅ | 5 | Benchmarks for shorten and redirect |
| ✅ | 5 | README: benchmark line + profiling insight |
| ✅ | 5 | Tests and `-race` still green |

## Part 4
| Done | Pts | Requirement |
|:----:|:---:|-------------|
| ✅ | 6 | Persistent `Store` (GORM+DB or file-backed) |
| ✅ | 5 | Startup load |
| ✅ | 5 | Create persisted before response |
| ✅ | 4 | Restart test (temp DB or temp files) |
| ✅ | 3 | Config selects memory vs persistent store |
| ✅ | 2 | `-race` clean with persistent store |

## Part 5
| Done | Pts | Requirement |
|:----:|:---:|-------------|
| ✅ | 4 | `DECISIONS.md`: LB → N apps → shared store |
| ✅ | 3 | CDN / edge caching for redirects |
| 📄 | 3 | Write-path scaling (rate limit, queue, or code pool) |
| [ ] | 3 | Sharding / partitioning strategy |
| [ ] | 4 | **Bonus code:** cache layer, load-test script + README numbers, etc. |

## Part 6
| Done | Pts | Requirement |
|:----:|:---:|-------------|
| [ ] | 3 | Graceful shutdown working + documented |
| [ ] | 3 | Rate limit on create |
| ✅ | 2 | Domain policy in `DECISIONS.md` |
| [ ] | 2 | What you log vs never log |
| [ ] | 4 | **Bonus:** pprof/metrics behind flag, or structured logging |