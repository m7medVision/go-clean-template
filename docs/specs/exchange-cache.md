# Spec: the exchange rates cached in Valkey

Module id: `exchange-cache` · second of three in one PR (see [README.md](README.md#valkey)) ·
Depends on `valkey` · Status: **spec approved 2026-11-08; plan in [exchange-cache-plan.md](exchange-cache-plan.md), tasks in
[exchange-cache-todo.md](exchange-cache-todo.md)**

Paths are checked against `feat/valkey` at `b071664` on 2026-11-08.

## Objective

`ConvertCurrency` asks ExchangeRate-API for a base currency's rates once per provider update,
not once per conversion. The provider publishes new rates once a day and says when the next update
is due (`time_next_update_unix`). Until then, every conversion from that base reads the rates from
Valkey. When it is done:

- the first conversion from a base fetches `GET /v6/latest/{base}` and stores the response body in
  Valkey under `exchange:latest:{BASE}`, expiring at the provider's next update;
- every later conversion from that base, from any app instance, reads the body from Valkey and
  makes no outbound request until the key expires;
- the cache is invisible to callers: same rates, same `rate_date`, same errors, same statuses;
- a Valkey that is down, slow or holding a body that doesn't decode counts as a miss: the
  conversion goes to the provider, as it does today;
- `repo.ExchangeWebAPI`, the `ConvertCurrency` handler and every transport stay unchanged. The
  cache lives inside the adapter that talks to the provider.

**Who it is for:**
- a developer reading the template, who sees an outbound API cached behind the port it already
  had, with the use case none the wiser;
- an operator, whose app stops calling a rate-limited third party on every conversion.

## Tech stack

Nothing new: `valkey-go` v1.0.79, from the `valkey` module. `SET key value PXAT <ms>` sets the
value and its absolute expiry in one command. Valkey has no persistence, so a restart empties the
cache, and the next conversion from each base refills it.

## Configuration

None. The cache needs no setting of its own:
- its lifetime is the provider's;
- the app requires `VALKEY_URL` already;
- caching can't be switched off (no on/off env vars).

## Design

```
ConvertCurrency ──Rate(ctx, from, to)──▶ webapi.ExchangeWebAPI ──Cache (webapi's port)──▶ valkey.Cache ──▶ Valkey
                                                  │
                                                  └── on a miss: GET {EXCHANGE_SERVICE_URL}/v6/latest/{from}
```

- **The port belongs to webapi.** `webapi.Cache` is declared in `internal/repo/webapi`, beside its
  only consumer, as `notify.Queue` is:
  ```go
  type Cache interface {
  	Get(ctx context.Context, key string) ([]byte, bool, error)
  	Set(ctx context.Context, key string, value []byte, expiresAt time.Time) error
  }
  ```
  `pkg/valkey.Cache` satisfies it structurally: bytes by key, with an absolute expiry, and nothing
  about exchange rates. `pkg/` still imports nothing from `internal/`.
- **What is stored is the provider's body, byte for byte.** A cached body goes through the same
  decode as a fresh one. Rates stay `decimal.Decimal` text from the provider to the conversion and
  never pass through a float or a re-encoding.
- **What may be stored:** a body that came back with HTTP 200, decoded, said `result: success` and
  gave a `time_next_update_unix` later than now.
- **What is never stored:**
  - an error body (`unsupported-code`, `rate-limited`, any other);
  - a non-200 answer;
  - a body that doesn't decode;
  - a body whose next update has already passed. The recorded fixture the integration stub serves
    is one of these: its next update was 2026-10-20.
- **The key** is `exchange:latest:` plus the base. The base arrives already normalised to three
  upper-case letters by `entity.NewConversion`, so no caller-supplied text reaches the key
  unchecked.
- **A hit decides everything a miss would.** A target missing from the cached rates is
  `ErrUnsupportedCurrency`, without a request. An unknown base was never cached, so it still goes
  to the provider and gets `unsupported-code`.
- **One deadline for the whole `Rate`.** Today `webapi.Timeout` (5 s) bounds the request, and the
  app's HTTP write deadline (`internal/app/servers.go`) counts on it. From now on it bounds
  everything `Rate` does: the cache read, the request and the cache write. This copies
  `sso.Timeout`:
  - `context.WithTimeoutCause(ctx, Timeout, errTimedOut)`;
  - `unavailable` tells that timeout (the provider was too slow: `ErrRatesUnavailable`) apart from
    the caller going away (not the provider's failure).

  So the write deadline stays exact, with no change to `servers.go`.
- **Each cache call is bounded too**, by `_cacheTimeout` (500 ms). valkey-go's default
  `ConnWriteTimeout` is 10 s, so without a bound a Valkey that hangs, rather than refusing, would
  spend the provider's whole budget. With it, a hung Valkey costs at most 500 ms of the 5 s.
- **Failing open is silent in the logs.** The cache's error is dropped, but it isn't hidden:
  valkeyotel records it on the command's span (error status), under the conversion's trace.
  Logging it would put one line per conversion in the log for as long as Valkey is down, and the
  adapter has no logger today.
- **The body is read through a 1 MiB limit.** It is now read into memory and stored, and the
  provider's real body is about 3 KiB. A body cut off at the limit doesn't decode, so it is
  `ErrRatesUnavailable` and is never stored.
- **Wiring:** `deps` gains the client, set only by the app; the worker's `deps` leaves it nil, as
  it does `store` and `tokens`. `internal/app/exchange.go` becomes
  `webapi.New(d.cfg.Exchange.ServiceURL, valkey.NewCache(d.valkey))`.

## Project structure

| File | Change |
| --- | --- |
| `pkg/valkey/cache.go` | new: `Cache`, `NewCache(client)`, `Get` (a missing key is `false`, no error), `Set` (`SET … PXAT`) |
| `internal/repo/webapi/exchange_openerapi.go` | the `Cache` port; `New(baseURL, cache)`; `Rate` under one `Timeout` with `errTimedOut`; the cache read before the request and the write after it; `time_next_update_unix` decoded; the body read through the 1 MiB limit |
| `internal/repo/webapi/exchange_openerapi_test.go` | the existing cases pass with a cache that always misses; new cases for hits, misses, what is never stored and a failing cache |
| `internal/app/modules.go` | `deps.valkey`, only the app's |
| `internal/app/app.go` | the client from `valkey.New` goes into `deps` |
| `internal/app/exchange.go` | `webapi.New(…, valkey.NewCache(d.valkey))` |
| `integration-test/valkey_test.go` | `valkey.Cache` against the stack's Valkey: a missing key, a round trip, and the expiry Valkey reports (`PEXPIRETIME`) equal to the one set |
| `integration-test/exchange_test.go` | a conversion served from a cached body the test seeds (see Testing) |
| `docs/request-flow.md` | the diagram gains Valkey; the wiring quote; "6. The outbound API" gains the cache; the test table |
| `docs/architecture.md` | the `pkg/valkey` row (`Cache`) and the `internal/repo/webapi` row (cached until the next update) |
| `CLAUDE.md` | the `internal/repo/webapi/` line says the rates are cached in Valkey |

## Code style

The repo's conventions, plus no comments unless one is important. The only one planned is on the
dropped `Set` error: dropping an error needs its reason stated. The heart of the change:

```go
func (w *ExchangeWebAPI) latest(ctx context.Context, base string) (latestResponse, error) {
	key := _cacheKeyPrefix + base

	if body, ok := w.cached(ctx, key); ok {
		return body, nil
	}

	raw, body, err := w.fetch(ctx, base)
	if err != nil {
		return latestResponse{}, err
	}

	if next := time.Unix(body.NextUpdate, 0); body.Result == _resultSuccess && next.After(time.Now()) {
		w.store(ctx, key, raw, next)
	}

	return body, nil
}

func (w *ExchangeWebAPI) cached(ctx context.Context, key string) (latestResponse, bool) {
	ctx, cancel := context.WithTimeout(ctx, _cacheTimeout)
	defer cancel()

	raw, ok, err := w.cache.Get(ctx, key)
	if err != nil || !ok {
		return latestResponse{}, false
	}

	var body latestResponse
	if json.Unmarshal(raw, &body) != nil {
		return latestResponse{}, false
	}

	return body, true
}
```

`fetch` is today's `latest` with the body read once into `raw`; `store` is `cached`'s mirror
around `Set`.

## Testing strategy

- **Unit, `internal/repo/webapi` (`make test`, no Docker).** A map-backed fake `Cache` in the test
  file, and the `httptest` server counting its requests. Table-driven:
  - a miss fetches once and stores the body under `exchange:latest:OMR`, expiring at its
    `time_next_update_unix` (the body is built with a next update an hour ahead);
  - a hit makes no request and returns the cached rate and date;
  - on a hit, a target missing from the cached rates is `ErrUnsupportedCurrency`, with no request;
  - the recorded fixture, whose next update has passed, is returned and not stored;
  - error bodies, non-200 answers and undecodable bodies are not stored;
  - a cache whose `Get` fails is a miss, and one whose `Set` fails still returns the rate;
  - a cached body that doesn't decode is a miss, and the fresh body replaces it;
  - every existing case still passes, unchanged except for the cache argument.
- **Integration, `pkg/valkey.Cache`** (`integration-test/valkey_test.go`, under `_test:`):
  - `Get` of a missing key is `false` with no error;
  - `Set` then `Get` returns the bytes;
  - `PEXPIRETIME` reports the `expiresAt` that was set, to the millisecond.
- **Integration, end to end** (`integration-test/exchange_test.go`):
  - The test seeds `exchange:latest:USD` with a body whose `CNY` rate differs from the stub's and
    whose next update is an hour ahead.
  - It converts USD to CNY and expects the seeded rate. The stub would have answered 17.406483: it
    serves the OMR fixture for every base but XXX. So the app read Valkey before the provider.
  - It deletes the key afterwards. No other test converts from USD.
  - The suite already knows `VALKEY_URL`. The key format becomes something the suite relies on,
    and `docs/request-flow.md` documents it.
- **By hand, once:** two conversions from OMR within a minute against the real provider. In
  Jaeger, the first trace has the `GET` to open.er-api.com and the second has only the Valkey
  `GET`.

## Boundaries

- **Always:** G1 (`make format`, `make linter-golangci`, `make test`) before each commit; the
  `docs/request-flow.md` quotes kept true to the code; `repo.ExchangeWebAPI` and the use case left
  unchanged.
- **Ask first:**
  - caching anything besides a successful provider body (an unknown base, for example);
  - a cap on the expiry shorter than the provider's;
  - an in-process layer in front of Valkey (valkey-go's client-side caching, `DoCache`);
  - stampede protection (singleflight);
  - a log line for a failing cache.
- **Never:**
  - the cache in the use case, the aggregate or a controller;
  - a rate passed through a float or re-encoded before it is stored;
  - an on/off switch for the cache;
  - a stale rate served past the provider's next update.

## Success criteria

1. A second conversion from the same base, before the provider's next update, makes no outbound
   request (unit test; Jaeger by hand).
2. A conversion read from the cache returns the same rate, date and errors as one read from the
   provider (unit tests; the integration suite's existing exchange tests unchanged and green).
3. With Valkey stopped while the app runs, conversions still answer 200 at the provider's rate
   (unit tests for a failing cache; by hand with `docker compose stop valkey`).
4. Error answers, and bodies whose next update has passed, are never stored (unit tests).
5. The seeded integration test gets the cached rate, not the stub's; the suite and CI are green.
6. `repo.ExchangeWebAPI`, `ConvertCurrency`, the transports and `servers.go` are unchanged; G1
   passes at every commit; the docs in the table say what changed.

## Release

One series of `perf(exchange)` commits, plus `test` and `docs`. A caller sees no new behaviour,
so this module releases nothing of its own. The PR's release is the `feat(valkey)!` commit's.

## Decided by default (approved with the spec, 2026-11-08)

1. **Failing open writes no log line.** The error is on the trace (valkeyotel). The other choice is
   a logger passed into `webapi.New` and a warning per failed cache call.
2. **`_cacheTimeout` is 500 ms** per cache call, inside the 5 s `Timeout` for the whole `Rate`.
3. **The expiry is the provider's, uncapped.** A bogus far-future `time_next_update_unix` would
   keep a rate until Valkey restarts or the key is deleted. The conversion's `rate_date` still
   shows which day's rate it used.
4. **Not in this module:**
   - caching an unknown base;
   - stampede protection: at each expiry, the first concurrent conversions per base may each call
     the provider;
   - an in-process cache;
   - Prometheus metrics for hits and misses.
