# Configuration

Every setting is an environment variable, parsed by [`config/config.go`](../config/config.go) with
[caarlos0/env](https://github.com/caarlos0/env). There are no config files. Start from
[`.env.example`](../.env.example): the `Makefile` loads `.env` (falling back to `.env.example`) and
exports it into every target, and [docker-compose.yml](../docker-compose.yml) passes the same
variables to the containers.

Keep `.env` values free of spaces and quotes: both `make` and the shell read that file.

## App

The Default column is the value in `config/`, the only place a default is written: the packages the
settings reach take them as arguments. "required" means both binaries refuse to start without it;
"required by the app" means `cmd/app` refuses to start without it, and `cmd/worker` does not read it.

| Env var | Default | |
| --- | --- | --- |
| `APP_NAME` | required | |
| `APP_VERSION` | required | |
| `APP_BASE_URL` | `http://localhost` | the public origin users reach the app at, and the base of the links the app builds; a trailing slash is dropped. The web client's routes those links name (`/login`, `/sso/callback`) are built in one place, `config.App` |
| `HTTP_PORT` | `8080` | the HTTP API, its docs and metrics |
| `WEB_PORT` | `3000` | the Vite dev server's port (`make dev`, `make web-dev`); the app does not read it |
| `VITE_SENTRY_DSN` | empty | the web client, at build time (`make web-dev`, `make web-build`): where it sends unexpected errors; empty starts no SDK and sends nothing. The Go binaries do not read it |
| `VITE_POSTHOG_KEY` | empty | the web client, at build time: the PostHog project key it sends [analytics](analytics.md) with; empty loads nothing and sends nothing |
| `VITE_POSTHOG_HOST` | `https://eu.i.posthog.com` | the web client, at build time: the PostHog host |
| `VITE_RELEASE` | empty | the web client, at build time: the release name on reported errors, and the release the source maps belong to |
| `SENTRY_URL`, `SENTRY_AUTH_TOKEN`, `SENTRY_ORG`, `SENTRY_PROJECT` | empty | `make web-build` only: with `SENTRY_AUTH_TOKEN` set, the build emits source maps, uploads them to this GlitchTip organization and project, and deletes them from `dist/`; without it no maps are built. The DSN's public key is the 32-character hex form GlitchTip shows, not a dashed UUID |
| `GRPC_PORT` | `8081` | the gRPC transport |
| `TRUSTED_PROXY` | empty (no proxy) | the one address whose `X-Forwarded-For` the app reads, on HTTP and gRPC: Caddy's. An address that doesn't parse stops the app at startup. See [The client address](#the-client-address) |
| `LOG_LEVEL` | required | `debug`, `info`, `warn`, `error` |
| `PG_URL` | required | the app's database. The app applies the pending goose migrations to it at startup (with the `migrate` tag) |
| `PG_POOL_MAX` | required | |
| `JWT_SECRET` | required by the app | change it outside development |
| `JWT_TOKEN_EXPIRY` | `15m` | the auth token's lifetime. Roles are fixed when it is issued, so this is also how long a revoked role lingers ([architecture.md](architecture.md#roles)) |
| `METRICS_ENABLED` | `true` | serves Prometheus at `/metrics`: HTTP and gRPC request counts and latencies, commands and queries, projection lag ([api.md](api.md)). It decides only whether `/metrics` is served: the collectors count either way |
| `DOCS_ENABLED` | `false` | the API reference UIs and documents ([api.md](api.md)): REST's `/docs`, its OpenAPI document and schemas, and GraphQL's playground and introspection, and gRPC server reflection |

Two variables are read by tooling, not by the app:

- `NATS_PORT` (default `4222`) is the host port the compose `nats` service publishes, and `NATS_URL`
  in `.env.example` is built from it.
- `PG_PORT` (default `5432`) is the host port the compose `db` service publishes, and `PG_URL` in
  `.env.example` is built from it. Set it when something else on the host already uses 5432.

### The client address

The request logs of both servers and the [sign-in limit](#sign-in-limit) use the client's
address. [`clientip.Resolve`](../pkg/clientip/clientip.go) holds the rule:

- With `TRUSTED_PROXY` empty, the client is the address of the connection.
- On a connection from `TRUSTED_PROXY`, the client is the rightmost entry of `X-Forwarded-For` (on
  gRPC, the `x-forwarded-for` metadata): the one the proxy wrote. Anything to its left came from the
  client. If that entry doesn't parse, the client is the proxy's own address.
- On a connection from any other address, the header is ignored, and the connection's address is
  the client.

Only Caddy is supported as the proxy, as one address. Stock Caddy (no `trusted_proxies`) discards the
`X-Forwarded-For` a client sends and writes the address it received the connection from, so the app
gets one honest value. `X-Real-IP`, `Forwarded`, CDN headers and chi's `RealIP` are not used.

In compose, `app_network` is pinned to `172.30.0.0/24`, with the containers' own addresses taken
from `172.30.0.128/25`. Caddy is `172.30.0.10`, and the app's `TRUSTED_PROXY` is that address. Only
that one address is trusted, not the subnet: the app's ports are also published on the host, and a
connection from the host can arrive from the network's gateway, inside the subnet. If the subnet
collides with another Docker network on the host, change it in
[docker-compose.yml](../docker-compose.yml), and Caddy's `ipv4_address` and `TRUSTED_PROXY` with it.

## Sign-in limit

Read only by the app. Password sign-in and sign-up have a budget per client address and per account,
counted in Valkey ([api.md](api.md#user-authentication), [cqrs.md](cqrs.md#the-sign-in-limit)).
None can be turned off: a value of zero or less stops the app at startup.

| Env var | Default | |
| --- | --- | --- |
| `AUTH_LIMIT_CLIENT_ATTEMPTS` | `20` | attempts per client address per window, counted separately for sign-in and sign-up. Every attempt counts, a malformed one too. The integration stack and CI set `1000`, because the suite signs up and in many times from one address |
| `AUTH_LIMIT_ACCOUNT_FAILURES` | `5` | wrong passwords per account per window. Once they are spent, the next sign-in is refused even with the right password; a successful sign-in clears the count |
| `AUTH_LIMIT_WINDOW` | `15m` | the window both budgets use: a fixed window, starting with the first attempt |

## Valkey

| Env var | Default | |
| --- | --- | --- |
| `VALKEY_URL` | required by the app | `valkey://host:port/db`, or `valkeys://` for TLS; a password goes in the URL (`valkey://:secret@host:6379/0`). The app connects at startup and stops when Valkey cannot be reached: unlike Postgres, it does not wait for a server that is still starting |
| `VALKEY_PORT` | `6379` | the host port the compose `valkey` service publishes, and `VALKEY_URL` in `.env.example` is built from it, as `PG_URL` is from `PG_PORT`. Set it when something else on the host already uses 6379. The app does not read it |

## MongoDB

| Env var | Default | |
| --- | --- | --- |
| `MONGO_URL` | required by the app | `mongodb://user:password@host:port/database?authSource=admin`; the path names the database the app uses. The app connects and pings at startup and stops when MongoDB cannot be reached. The worker does not read it |
| `MONGO_PORT` | `27017` | the host port the compose `mongo` service publishes, and `MONGO_URL` in `.env.example` is built from it, as `PG_URL` is from `PG_PORT`. The app does not read it |

## Feature flags

| Env var | Default | |
| --- | --- | --- |
| `FEATURE_FLAGS_FILE` | empty | read only by the app: the [GO Feature Flag](feature-flags.md) file it polls (`config/flags.yaml` in `.env.example`, `/config/flags.yaml` in compose). Empty, every flag answers its default in code |

## Exchange

| Env var | Default | |
| --- | --- | --- |
| `EXCHANGE_SERVICE_URL` | `https://open.er-api.com` | base URL of the rate provider; the integration tests point it at a stub |

## WebAuthn

Passkeys. See [passkeys.md](passkeys.md#relying-party-setup) for the values to use in development,
compose and production.

| Env var | Default | |
| --- | --- | --- |
| `WEBAUTHN_RP_ID` | `localhost` | the relying party ID: the host passkeys are bound to. In production, the registrable domain (`example.com`); changing it orphans existing passkeys |
| `WEBAUTHN_RP_NAME` | `Go Clean Template` | the name the browser's passkey prompt shows; `.env.example` uses `go-clean-template` because of the no-spaces rule |
| `WEBAUTHN_RP_ORIGINS` | `http://localhost,http://localhost:3000,http://localhost:8080` | comma-separated origins a ceremony may come from, scheme and port included: Caddy, a dev server on port 3000 and the app's own port |
| `WEBAUTHN_CEREMONY_TIMEOUT` | `5m` | how long a ceremony can take: the browser prompt's timeout and the lifetime of the ceremony token |
| `WEBAUTHN_REAUTH_MAX_AGE` | `5m` | how recently the caller must have signed in (the age of their auth token) to add a passkey; an older sign-in gets a `401` and must sign in again ([passkeys.md](passkeys.md#adding-a-passkey)) |

The ceremony token's signing key is derived from `JWT_SECRET`, so there is no extra secret to set.

## OIDC

SSO sign-in ([sso.md](sso.md)). It is on when `OIDC_ISSUER` is set; `.env.example` and the compose
files point it at the dev Keycloak. The provider redirects to `APP_BASE_URL/sso/callback`, which must
be a redirect URI of the client.

| Env var | Default | |
| --- | --- | --- |
| `OIDC_ISSUER` | empty (SSO off) | the provider's issuer URL, as its discovery document states it |
| `OIDC_CLIENT_ID` | | the confidential client registered at the provider |
| `OIDC_CLIENT_SECRET` | | that client's secret |
| `OIDC_ROLES_CLAIM` | `realm_access.roles` | dotted path to the ID-token claim with the provider's role names: `roles` on Entra ID, `groups` on Okta |
| `KEYCLOAK_PORT` | `8180` | the development Keycloak's port in compose; `.env.example` builds `OIDC_ISSUER` from it. The app does not read it |
| `OIDC_ROLE_MAP` | empty | `provider-name:app-role` pairs, comma-separated, no spaces (`app-admin:admin,app-user:user`). Unmapped names grant nothing; a sign-in that maps to no role gets `user`. A pair naming any other app role stops the app at startup |

The state token's signing key is derived from `JWT_SECRET`, so there is no extra secret to set.

## Worker

Read only by `cmd/worker`; only `NATS_URL` is required, and only by it. See [worker.md](worker.md) for how they behave.

| Env var | Default | |
| --- | --- | --- |
| `NATS_URL` | required by the worker | NATS with JetStream: the worker's queue. The app never reads it. `RunWorker` refuses to start without it |
| `WORKER_MAX_RETRIES` | `5` | retries of a failing handler before its message moves to the `poison` topic |
| `WORKER_RETRY_INITIAL` | `1s` | wait before the first retry; it doubles each time |
| `WORKER_RETRY_MAX` | `30s` | longest wait between retries |
| `WORKER_ACK_WAIT` | `5m` | how long JetStream waits for an acknowledgement before redelivering; longer than a handler and its retries |
| `WORKER_SHUTDOWN_TIMEOUT` | `10s` | grace period for in-flight handlers |

## CQRS

The app reads the first two; the worker, which runs the projections, the last two. None is
required. See [cqrs.md](cqrs.md) for how they behave.

| Env var | Default | |
| --- | --- | --- |
| `CQRS_CONSISTENCY_TIMEOUT` | `2s` | how long a write waits for its projections; past it the write answers 503 (saved, read again shortly). The HTTP write deadline follows it: this, plus the 5s timeout of the rate API and of the SSO provider (a conversion or a finished sign-in waits for both), plus 3s |
| `EVENTSTORE_SNAPSHOT_EVERY` | `100` | snapshot a stream each time it crosses a multiple of this many events; `0` never snapshots |
| `PROJECTION_POLL_INTERVAL` | `1s` | how often an idle projection reads the store when no notification arrives |
| `PROJECTION_BATCH_SIZE` | `100` | events a projection reads at once |

### Email

Also read only by the worker, which sends the email.

| Env var | Default | |
| --- | --- | --- |
| `SMTP_HOST` | `localhost` | SMTP relay (`mailpit` in compose) |
| `SMTP_PORT` | `1025` | also the port compose publishes Mailpit's SMTP on |
| `MAILPIT_UI_PORT` | `8025` | the port compose publishes Mailpit's inbox UI and API on; the app does not read it |
| `SMTP_USERNAME` | empty | empty turns auth off; when set, PLAIN auth, sent only over TLS or to localhost |
| `SMTP_PASSWORD` | empty | |
| `MAIL_FROM` | `Go Clean Template <noreply@example.com>` | sender address, parsed at startup: anything but exactly one address stops the binary. `.env.example` uses a bare address because of the no-spaces rule |

### Analytics

Read only by the worker, which sends the server's [analytics](analytics.md) events. Each vendor is on
when its key is set; with none, nothing is captured.

| Env var | Default | |
| --- | --- | --- |
| `POSTHOG_API_KEY` | empty | the PostHog project key; set turns the PostHog sink on |
| `POSTHOG_HOST` | `https://eu.i.posthog.com` | the PostHog host the sink sends to |

## Tracing

Distributed tracing uses [OpenTelemetry](https://opentelemetry.io/). Spans are exported over OTLP/gRPC
to a collector: [Jaeger](https://www.jaegertracing.io/) in the docker stack, with its UI at
http://127.0.0.1:16686.

- **One trace across transports.** W3C `traceparent` and `baggage` propagate through every transport.
  The HTTP transports use the
  [otelhttp](https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp)
  middleware.
  gRPC uses the [otelgrpc](https://github.com/open-telemetry/opentelemetry-go-contrib) stats handler.
  The outbound calls, to the exchange-rate API and to the SSO provider, use an `otelhttp` transport,
  so they join the same trace (all but the SSO key fetch, which go-oidc runs in the background).
- **Every layer has spans.** A trace shows the request, then a span per command or query (the
  buses' trace step), then one per SQL statement (otelpgx), per outbound HTTP call (otelhttp) and per Valkey command
  (valkeyotel) and per MongoDB command (`pkg/mongo`'s command monitor).
- **No-op when disabled.** With `TRACING_ENABLED=false`, a no-op provider is installed, so the
  instrumentation, HTTP's included, runs unconditionally with no overhead. An incoming `traceparent`
  still propagates, so every transport records the caller's trace ID as its events' correlation ID.
- **The APIs are traced, whatever their version.** Every HTTP request but the health check
  (`/healthz`) and the metrics scrape (`/metrics`) is traced: REST operations under any version,
  GraphQL and the docs. So is every gRPC call.

| Env var | Default | |
| --- | --- | --- |
| `TRACING_ENABLED` | `false` | `.env.example` turns it on |
| `TRACING_OTLP_ENDPOINT` | `localhost:4317` | OTLP/gRPC collector |
| `TRACING_OTLP_INSECURE` | `true` | no TLS to the collector |
| `TRACING_SAMPLE_RATE` | `0.1` | parent-based ratio. Use `1.0` locally (`.env.example` does), lower it in production |

## Error tracking

Unexpected errors go to GlitchTip, or any Sentry-protocol server, when a DSN is set; see
[error-tracking.md](error-tracking.md). Both binaries read these, each with its own project's DSN:
compose sets the app's from `SENTRY_APP_DSN` and the worker's from `SENTRY_WORKER_DSN`.

| Env var | Default | |
| --- | --- | --- |
| `SENTRY_DSN` | empty: off | the project's DSN |
| `SENTRY_ENVIRONMENT` | `development` | the `environment` of every event |
