# Tasks: the exchange rates cached in Valkey

Plan: [exchange-cache-plan.md](exchange-cache-plan.md) · Spec: [exchange-cache.md](exchange-cache.md)

Gates (G1–G4) are defined in the plan. Every task gets one Conventional Commit and its boxes ticked
here, and adds no comments to new code unless one is important. Sizes: S = 1–2 files, M = 3–5.

---

### 1. A cache in `pkg/valkey`
- [x] `valkey.Cache` and `NewCache(client vk.Client) *Cache`.
- [x] `Get(ctx, key) ([]byte, bool, error)`: a missing key (`vk.IsValkeyNil`) is `nil, false, nil`;
      any other error is wrapped `valkey - Cache.Get: %w`.
- [x] `Set(ctx, key, value, expiresAt) error`: `SET key value PXAT expiresAt`, the value sent with
      `vk.BinaryString`; its error is wrapped `valkey - Cache.Set: %w`.
- [x] `integration-test/valkey_test.go`, under `_test:<uuid>`:
  - [x] `Get` of a missing key is `false` with no error;
  - [x] `Set` then `Get` returns the bytes;
  - [x] `PEXPIRETIME` equals `expiresAt.UnixMilli()`;
  - [x] the key is deleted in `t.Cleanup`.

**Files:** `pkg/valkey/cache.go`, `integration-test/valkey_test.go`.
**Verify:** G1, `go vet ./integration-test/...`. **Depends on:** none. **Size:** S

### 2. The adapter reads the cache
- [x] `webapi.Cache` (`Get`, `Set`) in `exchange_openerapi.go`; `New(baseURL string, cache Cache)`.
- [x] `latestResponse` decodes `time_next_update_unix` into `NextUpdate`.
- [x] One deadline per `Rate`:
  - [x] `context.WithTimeoutCause(ctx, w.timeout, errTimedOut)`, with `w.timeout` set to
        `Timeout`;
  - [x] `unavailable` treats `errTimedOut` as the provider's failure and a caller that went away
        as not;
  - [x] `export_test.go` adds `SetTimeout`.
- [x] `latest`: `cached` first, then `fetch`, then `store`, as in the spec's snippet. Each cache
      call runs under `_cacheTimeout` (500 ms). Key prefix `_cacheKeyPrefix = "exchange:latest:"`.
- [x] `fetch` reads the body once, through `io.LimitReader` at 1 MiB (`_maxBody`), and decodes it.
- [x] `store` runs only for `result: success` with a next update after now, and drops `Set`'s
      error. That is the one comment.
- [x] Unit tests: a map-backed fake `Cache` (it can be set to fail, or to hang until its ctx ends),
      and the `httptest` server counting its requests. Table-driven where there are cases:
  - [x] a miss fetches once and stores the body under `exchange:latest:OMR`, expiring at its next
        update (the body is built with a next update an hour ahead);
  - [x] a hit makes no request and returns the cached rate and date;
  - [x] on a hit, a target missing from the cached rates is `ErrUnsupportedCurrency`, with no
        request;
  - [x] the recorded fixture, whose next update has passed, is returned and not stored;
  - [x] error bodies, non-200 answers and undecodable bodies are not stored;
  - [x] a body over 1 MiB is `ErrRatesUnavailable` and is not stored;
  - [x] a cache whose `Get` fails, or hangs, is a miss; one whose `Set` fails still returns the
        rate;
  - [x] a cached body that doesn't decode is a miss, and the fresh body replaces it;
  - [x] a provider that never answers, under `SetTimeout(100ms)`, is `ErrRatesUnavailable`;
  - [x] the existing cases still pass, with a cache that always misses.
- [x] Wiring:
  - [x] `deps.valkey vk.Client`, which only the app sets (`app.go` passes in the client from
        `valkey.New`);
  - [x] `exchange.go` calls `webapi.New(d.cfg.Exchange.ServiceURL, valkey.NewCache(d.valkey))`.

**Files:** `internal/repo/webapi/exchange_openerapi.go`, `exchange_openerapi_test.go`,
`export_test.go`, `internal/app/modules.go`, `internal/app/app.go`, `internal/app/exchange.go`.
**Verify:** G1; G2. **Depends on:** 1. **Size:** M (six files; the three in `internal/app` change
by one line each)

### 3. The end-to-end proof
- [x] `integration-test/exchange_test.go`:
  - [x] seed `exchange:latest:USD` through `valkey.Cache` with a body whose `CNY` rate isn't the
        stub's 17.406483 and whose next update is an hour ahead;
  - [x] `POST /v1/exchange/convert` USD → CNY answers 200 with the seeded rate and its date;
  - [x] the key is deleted in `t.Cleanup`.

**Files:** `integration-test/exchange_test.go`.
**Verify:** G1, `go vet ./integration-test/...`; G3. **Depends on:** 1, 2. **Size:** S

### 4. Docs
- [x] `docs/request-flow.md`:
  - [x] the diagram gains Valkey (a hit, and the provider on a miss);
  - [x] the wiring quote is the new `webapi.New(…, valkey.NewCache(d.valkey))`;
  - [x] "6. The outbound API" gains the cache: the key, what is stored, the expiry, failing open,
        the one deadline;
  - [x] the test table lists the fake cache and the seeded integration test.
- [x] `docs/architecture.md`: the `pkg/valkey` row adds `Cache`; the `internal/repo/webapi` row
      says rates are cached until the provider's next update.
- [x] `CLAUDE.md`: the `internal/repo/webapi/` line says the rates are cached in Valkey.

**Files:** the three above. **Verify:** each page read through; the quotes match the code; links
resolve. **Depends on:** 2, 3. **Size:** M

### Checkpoint
- [ ] G3 green on the branch head.
- [ ] Spec success criteria 1–6 checked one by one; criterion 1's Jaeger check by hand, once.
- [ ] Push; G4 green on PR #52; PR body's module table updated.
- [ ] Then the `signin-limit` spec.
