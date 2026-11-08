# API

The three domains and how each transport exposes them. Every transport sends the same commands and queries
([cqrs.md](cqrs.md)), so behaviour and errors match across transports.

| Transport | Local | Behind Caddy (`make compose-up-all`) |
| --- | --- | --- |
| REST | `http://127.0.0.1:8080/v1/...` | `http://app.lvh.me/v1/...` |
| GraphQL | `POST http://127.0.0.1:8080/graphql` | `POST http://app.lvh.me/graphql` |
| gRPC | `127.0.0.1:8081` | `grpc.lvh.me:80` (h2c) |

Every operation except register and the sign-ins needs `Authorization: Bearer <token>`, with the
token from a sign-in. On REST that is what an operation's `security` in the OpenAPI document says,
and the server enforces exactly that: one declaration, the operation's Huma `Security`, does both.
A path no operation serves is a `404`, token or not.

## User authentication

Registration, login, and JWT-based authorization.

| Operation | REST |
| --- | --- |
| Register | `POST /v1/auth/register` |
| Login | `POST /v1/auth/login` |
| Get profile | `GET /v1/user/profile` |

- Passwords are hashed with bcrypt; JWTs expire after `JWT_TOKEN_EXPIRY`.
- Registration queues a welcome email that the worker sends (see [worker.md](worker.md)).
- Password sign-in and sign-up are limited, on every transport, with budgets per window
  (`AUTH_LIMIT_WINDOW`, 15 minutes; [configuration.md](configuration.md#sign-in-limit)):
  - each client address gets 20 attempts at each (`AUTH_LIMIT_CLIENT_ATTEMPTS`). Every attempt
    counts, a malformed one too. The address is the connection's, or behind Caddy the one Caddy
    forwards ([configuration.md](configuration.md#the-client-address));
  - each account gets 5 wrong passwords (`AUTH_LIMIT_ACCOUNT_FAILURES`). Once they are spent, the
    next sign-in is refused even with the right password. A successful sign-in clears the count.
- A refused attempt answers `429` with `Retry-After` in whole seconds (rounded up, at least 1), and
  gRPC `ResourceExhausted` with a `RetryInfo` detail. The message, `too many attempts`, does not say
  which budget was spent. The OpenAPI document lists `429` on `register` and `login`.
- Anyone who knows an email can spend its 5 failures and block password sign-in to that account
  for up to a window. Passkey and SSO sign-in are not limited, so they still work.

## SSO

Sign-in through an OpenID Connect provider (Keycloak in development), for the same JWT password login
returns, carrying the roles the provider grants. REST only. The flow, the role mapping and provider
setup are in [sso.md](sso.md).

| Method | Path | Auth | Purpose |
| --- | --- | --- | --- |
| `GET` | `/v1/auth/oidc` | none | `{enabled}`: whether `OIDC_ISSUER` is set |
| `POST` | `/v1/auth/oidc/begin` | none | `{url, ceremony_token}`: send the browser to `url`; `404` when SSO is off, `502` when the provider is down or does not answer within 5 s |
| `POST` | `/v1/auth/oidc/finish` | none | Body `{code, state}` from the callback, header `X-Ceremony-Token`; returns `{token}`. `401` for a bad or replayed sign-in (the provider refuses the code, or the ID token does not verify), `409` when the email or username belongs to another account, `502` when the provider is down or does not answer within 5 s at any step: discovery, the code exchange or fetching its keys |

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
| Update attributes | `PATCH /v1/tasks/{id}/attributes` |

- Status transitions: `todo` → `in_progress` → `done`, and `in_progress` → `todo`.
- Lists page with `limit`/`offset` (10 by default, at most 100; `entity.Page` holds the rule for
  every transport, and on gRPC an unset `limit` is the default) and take an optional `status` filter.
- The admin list needs the `admin` role in the caller's token, or it answers `403`. A password
  sign-in grants only `user`; see [architecture.md](architecture.md#roles).
- **Attributes** are typed fields kept in MongoDB, beside the event-sourced task: `priority`
  (`low`, `medium`, `high`), `estimate` (hours, 0.5–200) and `labels` (at most 10, each 1–30
  characters, no duplicates). Every task read returns them as `attributes`; a task that never had
  any returns empty ones. The update merges: a field left out stays as it is, `null` clears it, a
  value sets it (gRPC: the `update_mask` names the fields, and a named field left unset clears it;
  GraphQL: an input field left out stays). It answers the task with its attributes. Deleting a task
  deletes its attributes.

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
- An unknown currency, or `from` equal to `to`, answers 400. The amount is a decimal string, positive,
  with at most 15 digits before the point and 8 after: `entity.NewConversion` parses and checks it for
  every transport (400), and REST's pattern says the same (422). A rate provider that cannot be
  reached, times out, or answers with an error or a body that does not decode is `502` (gRPC
  `Unavailable`). History pages with `limit`/`offset` and
  returns a `total`.

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
- **Any write** can also answer `503` when it is saved but the read models have not caught up within
  `CQRS_CONSISTENCY_TIMEOUT` (read again shortly, don't write again), or `409` when the record changed
  while the request ran, after retries ([cqrs.md](cqrs.md#reads-after-writes)).
- **Any operation** whose handler is still the stub the generator wrote answers `501`
  (`entity.ErrNotImplemented`) until the handler is written ([generator.md](generator.md)).
- **Validation** failures answer `422`. That includes query parameters: on `GET /v1/tasks`, `limit`
  must be 1–100, `offset` at least 0, and `status` one of `todo`, `in_progress`, `done`.
  Unknown fields in a request body are ignored.
- **Metrics** at `/metrics` (`METRICS_ENABLED`): `http_requests_total` and
  `http_request_duration_seconds`, labeled by `method`, `route` (the chi pattern, or `unmatched`) and
  `code`; for gRPC, `grpc_requests_total` and `grpc_request_duration_seconds`, labeled by `method`
  (the full method name) and `code`.
- `/healthz` answers when the app is up.

## GraphQL

[gqlgen](https://github.com/99designs/gqlgen) on the REST port, covering tasks and the exchange
history. The schema is in [schema/](../internal/controller/graphql/schema/): each domain's types
in `<domain>_types.graphqls`, and each field in its own file, named after it
([create_task.graphqls](../internal/controller/graphql/schema/create_task.graphqls) declares
`createTask`).

| Operation | GraphQL |
| --- | --- |
| Create a task | `mutation createTask` |
| List tasks | `query tasks` |
| List every user's tasks (admin) | `query allTasks` |
| Get a task | `query task` |
| Update a task | `mutation updateTask` |
| Transition a task | `mutation transitionTask` |
| Delete a task | `mutation deleteTask` |
| Update a task's attributes | `mutation updateTaskAttributes` |
| Exchange history | `query exchangeHistory` |

- With `DOCS_ENABLED=true`, a playground is at `/graphql/playground`. Put
  `{"Authorization": "Bearer <token>"}` in its headers pane.
- Introspection (`__schema`, `__type`) is part of the docs: it answers only with `DOCS_ENABLED=true`,
  and is refused (`introspection disabled`) otherwise. Resolvers check the bearer token themselves,
  so it needs no token. The web client's types come from the schema files, not from introspection.
- Errors carry `extensions.code`: `UNAUTHENTICATED`, `BAD_USER_INPUT`, `NOT_FOUND`, `FORBIDDEN`
  (the admin list without the `admin` role), `UNAVAILABLE` and `CONFLICT` (a write's `503` and `409`
  on REST), `NOT_IMPLEMENTED` (a generated stub handler's `501`), `RATE_LIMITED` (REST's `429`; no
  GraphQL operation signs in today), `INTERNAL`. An expired token is
  `UNAUTHENTICATED` in a 200 response.

## gRPC

Services are defined in [auth.proto](proto/v1/auth.proto), [task.proto](proto/v1/task.proto) and
[exchange.proto](proto/v1/exchange.proto); `make proto-v1` regenerates the Go code beside them. With
`DOCS_ENABLED`, server reflection is on, so `grpcurl -plaintext 127.0.0.1:8081 list` works.

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
| Update a task's attributes | `TaskService/UpdateTaskAttributes` |
| Convert | `ExchangeService/Convert` |
| Exchange history | `ExchangeService/GetHistory` |

- The token goes in the `authorization` metadata as it is, with no `Bearer` prefix; without a valid
  one a call answers `Unauthenticated`. `Register` and `Login` are the methods called without one: a
  domain's module lists them by their generated `FullMethodName` constants (`grpcPublic`).
- Every call is logged and counted like an HTTP request, and a panic answers `Internal` instead of
  ending the process.
- Errors map to status codes: invalid input (an unknown currency too) is `InvalidArgument`, a missing
  user or task `NotFound`, a taken username or email `AlreadyExists`, another user's task or the admin
  list without the `admin` role `PermissionDenied`, a write the read models have not caught up with
  `Unavailable`, a write that lost to a concurrent one `Aborted`, a generated stub handler
  `Unimplemented`, a spent sign-in budget on `Register` or `Login` `ResourceExhausted` (with a
  `RetryInfo` detail), and anything else `Internal`.

## Versioning

Each transport keeps its handlers under `v1/`, one folder per domain. A v2 is a sibling `v2/` with the
same domain folders, registered next to v1 in the domain's module in `internal/app` (for tasks,
[`internal/app/task.go`](../internal/app/task.go)); v1 stays untouched.

REST: the v2 operations go on the same Huma API under `/v2/...` paths, so they share one OpenAPI
document, which `make openapi` exports from the same registration. A v2 operation needs a token when
it declares `Security`, as a v1 one does: nothing depends on the path.

```go
rest: func(api huma.API, b cqrs.Buses, l *slog.Logger) {
	resttask.NewRoutes(api, b, l)   // restapi/v1/task
	resttaskv2.NewRoutes(api, b, l) // restapi/v2/task
},
```

gRPC, with the new protos in `docs/proto/v2`:

```go
grpc: func(s grpc.ServiceRegistrar, b cqrs.Buses, l *slog.Logger) {
	pb.RegisterTaskServiceServer(s, grpctask.New(b, l))
	pbv2.RegisterTaskServiceServer(s, grpctaskv2.New(b, l))
},
```

A v2 method needs a token unless the module lists it in `grpcPublic`, as the user module lists
`AuthService` sign-up and sign-in.
