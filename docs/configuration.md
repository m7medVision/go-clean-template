# Configuration

Every setting is an environment variable, parsed by [`config/config.go`](../config/config.go) with
[caarlos0/env](https://github.com/caarlos0/env). There are no config files. Start from
[`.env.example`](../.env.example): the `Makefile` loads `.env` (falling back to `.env.example`) and
exports it into every target, and [docker-compose.yml](../docker-compose.yml) passes the same
variables to the containers.

Keep `.env` values free of spaces and quotes: both `make` and the shell read that file.

## App

The Default column is the value in `config.go`; "required" means the binary refuses to start without
it.

| Env var | Default | |
| --- | --- | --- |
| `APP_NAME` | required | |
| `APP_VERSION` | required | |
| `APP_BASE_URL` | `http://localhost` | the public origin users reach the app at, and the base of the links the app builds |
| `HTTP_PORT` | required | the HTTP API, its docs and metrics |
| `WEB_PORT` | `3000` | the Vite dev server's port (`make dev`, `make web-dev`); the app does not read it | <!-- scaffold:web -->
| `GRPC_PORT` | `8081` | the gRPC transport | <!-- scaffold:grpc -->
| `LOG_LEVEL` | required | `debug`, `info`, `warn`, `error` |
| `PG_URL` | required | the app's database. The startup schema apply makes it match the HCL schema, dropping what the schema doesn't declare |
| `PG_POOL_MAX` | required | |
| `JWT_SECRET` | required | change it outside development |
| `JWT_TOKEN_EXPIRY` | `15m` | the auth token's lifetime. Roles are fixed when it is issued, so this is also how long a revoked role lingers ([architecture.md](architecture.md#roles)) |
| `METRICS_ENABLED` | `true` | Prometheus at `/metrics` |
| `DOCS_ENABLED` | `false` | the API reference UIs and documents ([api.md](api.md)) |

Two variables are read by tooling, not by the app:

- `PG_PORT` (default `5432`) is the host port the compose `db` service publishes, and `PG_URL` in
  `.env.example` is built from it. Set it when something else on the host already uses 5432.
- `ATLAS_DEV_URL` is read only by `make sqlc`, which renders the HCL schema in a throwaway
  `docker://` Postgres container.

## Exchange

| Env var | Default | |
| --- | --- | --- |
| `EXCHANGE_SERVICE_URL` | `https://open.er-api.com` | base URL of the rate provider; the integration tests point it at a stub |

<!-- scaffold:passkeys:begin -->
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

<!-- scaffold:passkeys:end -->
<!-- scaffold:sso:begin -->
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
| `OIDC_ROLE_MAP` | empty | `provider-name:app-role` pairs, comma-separated, no spaces (`app-admin:admin,app-user:user`). Unmapped names grant nothing; a sign-in that maps to no role gets `user` |

The state token's signing key is derived from `JWT_SECRET`, so there is no extra secret to set.

<!-- scaffold:sso:end -->
<!-- scaffold:queue:begin -->
## Worker

Read only by `cmd/worker`, so none of them is required. See [worker.md](worker.md) for how they behave.

| Env var | Default | |
| --- | --- | --- |
| `WORKER_POLL_INTERVAL` | `1s` | sleep after an empty read |
| `WORKER_VISIBILITY_TIMEOUT` | `30s` | how long a read message is hidden before redelivery |
| `WORKER_BATCH_SIZE` | `10` | messages per read |
| `WORKER_SHUTDOWN_TIMEOUT` | `10s` | grace period for in-flight handlers |
<!-- scaffold:email:begin -->

### Email

Also read only by the worker, which sends the email.

| Env var | Default | |
| --- | --- | --- |
| `SMTP_HOST` | `localhost` | SMTP relay (`mailpit` in compose) |
| `SMTP_PORT` | `1025` | also the port compose publishes Mailpit's SMTP on |
| `MAILPIT_UI_PORT` | `8025` | the port compose publishes Mailpit's inbox UI and API on; the app does not read it |
| `SMTP_USERNAME` | empty | empty turns auth off; when set, PLAIN auth, sent only over TLS or to localhost |
| `SMTP_PASSWORD` | empty | |
| `MAIL_FROM` | `Go Clean Template <noreply@example.com>` | sender address; `.env.example` uses a bare address because of the no-spaces rule |
<!-- scaffold:email:end -->
<!-- scaffold:queue:end -->

## Tracing

Distributed tracing uses [OpenTelemetry](https://opentelemetry.io/). Spans are exported over OTLP/gRPC
to a collector: [Jaeger](https://www.jaegertracing.io/) in the docker stack, with its UI at
http://127.0.0.1:16686.

- **One trace across transports.** W3C `traceparent` and `baggage` propagate through every transport.
  The HTTP transports use the
  [otelhttp](https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp)
  middleware.
  gRPC uses the [otelgrpc](https://github.com/open-telemetry/opentelemetry-go-contrib) stats handler. <!-- scaffold:grpc -->
  The outbound exchange-rate call uses an `otelhttp` transport, so it joins the same trace.
- **Every layer has spans.** Use cases and repositories are wrapped in tracing decorators
  (`*/tracing.go`), so a trace shows controller → use case → repo or webapi.
- **No-op when disabled.** With `TRACING_ENABLED=false`, a no-op provider is installed, so the
  instrumentation runs unconditionally with no overhead.
- **Only the API is traced.** `/healthz`, `/metrics` and `/docs` are excluded; only the versioned
  API groups are instrumented.

| Env var | Default | |
| --- | --- | --- |
| `TRACING_ENABLED` | `false` | `.env.example` turns it on |
| `TRACING_OTLP_ENDPOINT` | `localhost:4317` | OTLP/gRPC collector |
| `TRACING_OTLP_INSECURE` | `true` | no TLS to the collector |
| `TRACING_SAMPLE_RATE` | `0.1` | parent-based ratio. Use `1.0` locally (`.env.example` does), lower it in production |
