# API

The three domains and how each transport exposes them. Every transport calls the same use cases, so
behaviour and errors match across transports.

| Transport | Local | Behind Caddy (`make compose-up-all`) |
| --- | --- | --- |
| REST | `http://127.0.0.1:8080/v1/...` | `http://app.lvh.me/v1/...` |
| GraphQL | `POST http://127.0.0.1:8080/graphql` | `POST http://app.lvh.me/graphql` | <!-- scaffold:graphql -->
| gRPC | `127.0.0.1:8081` | `grpc.lvh.me:80` (h2c) | <!-- scaffold:grpc -->

Every operation except register and the sign-ins needs `Authorization: Bearer <token>`, with the
token from a sign-in.

## User authentication

Registration, login, and JWT-based authorization.

| Operation | REST |
| --- | --- |
| Register | `POST /v1/auth/register` |
| Login | `POST /v1/auth/login` |
| Get profile | `GET /v1/user/profile` |

- Passwords are hashed with bcrypt; JWTs expire after `JWT_TOKEN_EXPIRY`.
- Registration queues a welcome email that the worker sends (see [worker.md](worker.md)). <!-- scaffold:welcome -->

<!-- scaffold:sso:begin -->
## SSO

Sign-in through an OpenID Connect provider (Keycloak in development), for the same JWT password login
returns, carrying the roles the provider grants. REST only. The flow, the role mapping and provider
setup are in [sso.md](sso.md).

| Method | Path | Auth | Purpose |
| --- | --- | --- | --- |
| `GET` | `/v1/auth/oidc` | none | `{enabled}`: whether `OIDC_ISSUER` is set |
| `POST` | `/v1/auth/oidc/begin` | none | `{url, ceremony_token}`: send the browser to `url`; `404` when SSO is off, `502` when the provider is down |
| `POST` | `/v1/auth/oidc/finish` | none | Body `{code, state}` from the callback, header `X-Ceremony-Token`; returns `{token}`. `401` for a bad or replayed sign-in, `409` when the email or username belongs to another account |

<!-- scaffold:sso:end -->
<!-- scaffold:passkeys:begin -->
## Passkeys

WebAuthn passkeys alongside passwords: add them while signed in, then sign in with one for the same
JWT password login returns. REST only; the ceremonies, the replay protection and the relying-party
setup are in [passkeys.md](passkeys.md).

| Method | Path | Auth | Purpose |
| --- | --- | --- | --- |
| `POST` | `/v1/passkeys/register/begin` | Bearer, recent | Start adding a passkey: creation options and a `ceremony_token`; `401` `reauthentication required` if the sign-in is older than `WEBAUTHN_REAUTH_MAX_AGE` |
| `POST` | `/v1/passkeys/register/finish` | Bearer | Verify the new credential and store it (`201`); header `X-Ceremony-Token`, optional query `name` |
| `GET` | `/v1/passkeys` | Bearer | The caller's passkeys, oldest first |
| `PATCH` | `/v1/passkeys/{id}` | Bearer | Rename a passkey; body `{"name": "…"}` |
| `DELETE` | `/v1/passkeys/{id}` | Bearer | Delete a passkey (`204`); it can no longer sign in |
| `POST` | `/v1/auth/passkey/begin` | none | Start a sign-in: request options and a `ceremony_token` |
| `POST` | `/v1/auth/passkey/finish` | none | Verify the assertion and return `{token}`; header `X-Ceremony-Token` |

- Both `finish` bodies are the browser's `PublicKeyCredential.toJSON()`, sent as it is. The
  `ceremony_token` from `begin` goes in the `X-Ceremony-Token` request header, not the query
  string, so it stays out of access logs.
- Adding a passkey needs a recent sign-in: `register/begin` checks the auth token's `iat`. A token
  older than `WEBAUTHN_REAUTH_MAX_AGE` (5 minutes by default) gets a `401` problem whose `detail` is
  exactly `reauthentication required`. The token is still valid for everything else; sign in again
  and retry. See [passkeys.md](passkeys.md#adding-a-passkey).
- `{id}` is the credential ID as unpadded base64url. Another user's passkey answers `404`.
- An invalid ceremony is `400` on registration and `401` on sign-in; finishing a sign-in twice is
  `409`. The full mapping is in [passkeys.md](passkeys.md#errors).

<!-- scaffold:passkeys:end -->
## Task management

CRUD with a status state machine. Tasks are scoped to the authenticated user.

| Operation | REST |
| --- | --- |
| Create | `POST /v1/tasks` |
| List | `GET /v1/tasks` |
| List every user's (admin) | `GET /v1/admin/tasks` |
| Get | `GET /v1/tasks/{id}` |
| Update | `PUT /v1/tasks/{id}` |
| Transition | `PATCH /v1/tasks/{id}/status` |
| Delete | `DELETE /v1/tasks/{id}` |

- Status transitions: `todo` → `in_progress` → `done`, and `in_progress` → `todo`.
- Lists page with `limit`/`offset` and take an optional `status` filter.
- The admin list needs the `admin` role in the caller's token, or it answers `403`. A password
  sign-in grants only `user`; see [architecture.md](architecture.md#roles).

## Currency exchange

Converts an amount between currencies at the latest daily rate from the
[ExchangeRate-API open access endpoint](https://www.exchangerate-api.com/docs/free) (keyless, about
160 currencies, including the Gulf currencies), and keeps each user's history. It is the template's
example of an outbound HTTP API behind a repository interface.

| Operation | REST |
| --- | --- |
| Convert | `POST /v1/exchange/convert` |
| History | `GET /v1/exchange/history` |

- Request: `{"from": "OMR", "to": "CNY", "amount": "100.10"}`. Codes are case-insensitive.
- Amounts and rates are decimal strings, computed with exact decimals (no `float64`). The result is
  rounded half-even to 2 places.
- Rates update once a day and are not live market rates; the response carries the rate's `rate_date`.
  The provider's terms require a link back to it wherever the rates are shown (the web page has one).
- An unknown currency, or `from` equal to `to`, answers 400. History pages
  with `limit`/`offset` and returns a `total`.

## REST

Routes are [chi](https://github.com/go-chi/chi) wrapped by [Huma](https://github.com/danielgtaylor/huma),
which builds the OpenAPI document from the registered input and output structs. There are no
annotation comments to maintain. With `DOCS_ENABLED=true` it serves:

- `/docs`: the API reference UI
- `/openapi.json`, `/openapi.yaml`, `/schemas/{schema}`: the document and its schemas

The same document is checked in at [`openapi/v1.json`](openapi/v1.json) (`make openapi`), so contract
changes show up in review and clients can generate types without a running server.

- **Errors** are RFC 9457 `application/problem+json`: `type`, `title`, `status`, `detail`, and
  `errors` for validation failures. Read `detail` for the message. Validation errors name the failing
  `location` but never echo the submitted value.
- **Validation** failures answer `422`. That includes query parameters: on `GET /v1/tasks`, `limit`
  must be at least 1, `offset` at least 0, and `status` one of `todo`, `in_progress`, `done`.
  Unknown fields in a request body are ignored.
- **Metrics** at `/metrics` (`METRICS_ENABLED`): `http_requests_total` and
  `http_request_duration_seconds`, labeled by `method`, `route` (the chi pattern, or `unmatched`) and
  `code`.
- `/healthz` answers when the app is up.

<!-- scaffold:graphql:begin -->
## GraphQL

[gqlgen](https://github.com/99designs/gqlgen) on the REST port, covering tasks and the exchange
history. The schemas are [task.graphqls](../internal/controller/graphql/schema/task.graphqls) and
[exchange.graphqls](../internal/controller/graphql/schema/exchange.graphqls).

| Operation | GraphQL |
| --- | --- |
| Create a task | `mutation createTask` |
| List tasks | `query tasks` |
| List every user's tasks (admin) | `query allTasks` |
| Get a task | `query task` |
| Update a task | `mutation updateTask` |
| Transition a task | `mutation transitionTask` |
| Delete a task | `mutation deleteTask` |
| Exchange history | `query exchangeHistory` |

- With `DOCS_ENABLED=true`, a playground is at `/graphql/playground`. Put
  `{"Authorization": "Bearer <token>"}` in its headers pane.
- Resolvers check the bearer token themselves, so introspection works without one.
- Errors carry `extensions.code`: `UNAUTHENTICATED`, `BAD_USER_INPUT`, `NOT_FOUND`, `FORBIDDEN`
  (the admin list without the `admin` role), `INTERNAL`. An expired token is `UNAUTHENTICATED` in a
  200 response.

<!-- scaffold:graphql:end -->
<!-- scaffold:grpc:begin -->
## gRPC

Services are defined in [auth.proto](proto/v1/auth.proto), [task.proto](proto/v1/task.proto) and
[exchange.proto](proto/v1/exchange.proto); `make proto-v1` regenerates the Go code beside them. Server
reflection is on, so `grpcurl -plaintext 127.0.0.1:8081 list` works.

| Operation | Method |
| --- | --- |
| Register | `AuthService/Register` |
| Login | `AuthService/Login` |
| Get profile | `AuthService/GetProfile` |
| Create a task | `TaskService/CreateTask` |
| List tasks | `TaskService/ListTasks` |
| List every user's tasks (admin) | `TaskService/ListAllTasks` |
| Get a task | `TaskService/GetTask` |
| Update a task | `TaskService/UpdateTask` |
| Transition a task | `TaskService/TransitionTask` |
| Delete a task | `TaskService/DeleteTask` |
| Convert | `ExchangeService/Convert` |
| Exchange history | `ExchangeService/GetHistory` |

- The token goes in the `authorization` metadata as it is, with no `Bearer` prefix; without a valid
  one a call answers `Unauthenticated`.
- Errors map to status codes: invalid input (an unknown currency too) is `InvalidArgument`, a missing
  user or task `NotFound`, a taken username or email `AlreadyExists`, another user's task or the admin
  list without the `admin` role `PermissionDenied`, and anything else `Internal`.
<!-- scaffold:grpc:end -->

## Versioning

Each transport keeps its handlers in a `v1/` package. A v2 is a sibling package with the same layout,
registered next to v1; v1 stays untouched.

REST, in [`internal/controller/restapi/router.go`](../internal/controller/restapi/router.go). The v2
operations go on the same Huma API under `/v2/...` paths, so they share one OpenAPI document:

```go
api := humachi.New(r, apiConfig)

v1.NewRoutes(api, e, u, tk, l)
v2.NewRoutes(api, e, u, tk, l)
```

<!-- scaffold:grpc:begin -->
gRPC, in [`internal/controller/grpc/router.go`](../internal/controller/grpc/router.go), with the new
protos in `docs/proto/v2`:

```go
{
	v1.NewAuthRoutes(app, u, l)
	v1.NewTaskRoutes(app, tk, l)
	v1.NewExchangeRoutes(app, e, l)
}

{
	v2.NewAuthRoutes(app, u, l)
	v2.NewTaskRoutes(app, tk, l)
	v2.NewExchangeRoutes(app, e, l)
}

reflection.Register(app)
```
<!-- scaffold:grpc:end -->
