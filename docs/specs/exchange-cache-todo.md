# Tasks: the exchange rates cached in Valkey

Plan: [exchange-cache-plan.md](exchange-cache-plan.md) · Spec: [exchange-cache.md](exchange-cache.md)

Gates (G1–G4) are defined in the plan. Every task gets one Conventional Commit and its boxes ticked
here, and adds no comments to new code unless one is important. Sizes: S = 1–2 files, M = 3–5.

---

### 1. A cache in `pkg/valkey`
- [ ] `valkey.Cache` and `NewCache(client vk.Client) *Cache`.
- [ ] `Get(ctx, key) ([]byte, bool, error)`: a missing key (`vk.IsValkeyNil`) is `nil, false, nil`;
      any other error is wrapped `valkey - Cache.Get: %w`.
- [ ] `Set(ctx, key, value, expiresAt) error`: `SET key value PXAT expiresAt`, the value sent with
      `vk.BinaryString`; its error is wrapped `valkey - Cache.Set: %w`.
- [ ] `integration-test/valkey_test.go`, under `_test:<uuid>`:
  - [ ] `Get` of a missing key is `false` with no error;
  - [ ] `Set` then `Get` returns the bytes;
  - [ ] `PEXPIRETIME` equals `expiresAt.UnixMilli()`;
  - [ ] the key is deleted in `t.Cleanup`.

**Files:** `pkg/valkey/cache.go`, `integration-test/valkey_test.go`.
**Verify:** G1, `go vet ./integration-test/...`. **Depends on:** none. **Size:** S

### 2. The adapter reads the cache
- [ ] `webapi.Cache` (`Get`, `Set`) in `exchange_openerapi.go`; `New(baseURL string, cache Cache)`.
- [ ] `latestResponse` decodes `time_next_update_unix` into `NextUpdate`.
- [ ] One deadline per `Rate`:
  - [ ] `context.WithTimeoutCause(ctx, w.timeout, errTimedOut)`, with `w.timeout` set to
        `Timeout`;
  - [ ] `unavailable` treats `errTimedOut` as the provider's failure and a caller that went away
        as not;
  - [ ] `export_test.go` adds `SetTimeout`.
- [ ] `latest`: `cached` first, then `fetch`, then `store`, as in the spec's snippet. Each cache
      call runs under `_cacheTimeout` (500 ms). Key prefix `_cacheKeyPrefix = "exchange:latest:"`.
- [ ] `fetch` reads the body once, through `io.LimitReader` at 1 MiB (`_maxBody`), and decodes it.
- [ ] `store` runs only for `result: success` with a next update after now, and drops `Set`'s
      error. That is the one comment.
- [ ] Unit tests: a map-backed fake `Cache` (it can be set to fail, or to hang until its ctx ends),
      and the `httptest` server counting its requests. Table-driven where there are cases:
  - [ ] a miss fetches once and stores the body under `exchange:latest:OMR`, expiring at its next
        update (the body is built with a next update an hour ahead);
  - [ ] a hit makes no request and returns the cached rate and date;
  - [ ] on a hit, a target missing from the cached rates is `ErrUnsupportedCurrency`, with no
        request;
  - [ ] the recorded fixture, whose next update has passed, is returned and not stored;
  - [ ] error bodies, non-200 answers and undecodable bodies are not stored;
  - [ ] a body over 1 MiB is `ErrRatesUnavailable` and is not stored;
  - [ ] a cache whose `Get` fails, or hangs, is a miss; one whose `Set` fails still returns the
        rate;
  - [ ] a cached body that doesn't decode is a miss, and the fresh body replaces it;
  - [ ] a provider that never answers, under `SetTimeout(100ms)`, is `ErrRatesUnavailable`;
  - [ ] the existing cases still pass, with a cache that always misses.
- [ ] Wiring:
  - [ ] `deps.valkey vk.Client`, which only the app sets (`app.go` passes in the client from
        `valkey.New`);
  - [ ] `exchange.go` calls `webapi.New(d.cfg.Exchange.ServiceURL, valkey.NewCache(d.valkey))`.

**Files:** `internal/repo/webapi/exchange_openerapi.go`, `exchange_openerapi_test.go`,
`export_test.go`, `internal/app/modules.go`, `internal/app/app.go`, `internal/app/exchange.go`.
**Verify:** G1; G2. **Depends on:** 1. **Size:** M (six files; the three in `internal/app` change
by one line each)

### 3. The end-to-end proof
- [ ] `integration-test/exchange_test.go`:
  - [ ] seed `exchange:latest:USD` through `valkey.Cache` with a body whose `CNY` rate isn't the
        stub's 17.406483 and whose next update is an hour ahead;
  - [ ] `POST /v1/exchange/convert` USD → CNY answers 200 with the seeded rate and its date;
  - [ ] the key is deleted in `t.Cleanup`.

**Files:** `integration-test/exchange_test.go`.
**Verify:** G1, `go vet ./integration-test/...`; G3. **Depends on:** 1, 2. **Size:** S

### 4. Docs
- [ ] `docs/request-flow.md`:
  - [ ] the diagram gains Valkey (a hit, and the provider on a miss);
  - [ ] the wiring quote is the new `webapi.New(…, valkey.NewCache(d.valkey))`;
  - [ ] "6. The outbound API" gains the cache: the key, what is stored, the expiry, failing open,
        the one deadline;
  - [ ] the test table lists the fake cache and the seeded integration test.
- [ ] `docs/architecture.md`: the `pkg/valkey` row adds `Cache`; the `internal/repo/webapi` row
      says rates are cached until the provider's next update.
- [ ] `CLAUDE.md`: the `internal/repo/webapi/` line says the rates are cached in Valkey.

**Files:** the three above. **Verify:** each page read through; the quotes match the code; links
resolve. **Depends on:** 2, 3. **Size:** M

### Checkpoint
- [ ] G3 green on the branch head.
- [ ] Spec success criteria 1–6 checked one by one; criterion 1's Jaeger check by hand, once.
- [ ] Push; G4 green on PR #52; PR body's module table updated.
- [ ] Then the `signin-limit` spec.
