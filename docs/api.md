# API

The three domains and how each transport exposes them: REST, gRPC, and GraphQL. All three call
the same use cases, so behaviour and errors match across transports.

| Transport | Local | Behind Caddy (`make compose-up-all`) |
| --- | --- | --- |
| REST | `http://127.0.0.1:8080/v1/...` | `http://app.lvh.me/v1/...` |
| GraphQL | `POST http://127.0.0.1:8080/graphql` | `POST http://app.lvh.me/graphql` |
| gRPC | `127.0.0.1:8081` | `grpc.lvh.me:80` (h2c) |

Every operation except register and login needs `Authorization: Bearer <token>`, with the token from
login.

## User authentication

Registration, login, and JWT-based authorization.

| Operation | REST | gRPC |
| --- | --- | --- |
| Register | `POST /v1/auth/register` | `AuthService/Register` |
| Login | `POST /v1/auth/login` | `AuthService/Login` |
| Get profile | `GET /v1/user/profile` | `AuthService/GetProfile` |

- Passwords are hashed with bcrypt; JWTs expire after `JWT_TOKEN_EXPIRY`.
- Registration queues a welcome email that the worker sends (see [worker.md](worker.md)).

## Task management

CRUD with a status state machine. Tasks are scoped to the authenticated user.

| Operation | REST | gRPC | GraphQL |
| --- | --- | --- | --- |
| Create | `POST /v1/tasks` | `TaskService/CreateTask` | `mutation createTask` |
| List | `GET /v1/tasks` | `TaskService/ListTasks` | `query tasks` |
| Get | `GET /v1/tasks/{id}` | `TaskService/GetTask` | `query task` |
| Update | `PUT /v1/tasks/{id}` | `TaskService/UpdateTask` | `mutation updateTask` |
| Transition | `PATCH /v1/tasks/{id}/status` | `TaskService/TransitionTask` | `mutation transitionTask` |
| Delete | `DELETE /v1/tasks/{id}` | `TaskService/DeleteTask` | `mutation deleteTask` |

- Status transitions: `todo` → `in_progress` → `done`, and `in_progress` → `todo`.
- Lists page with `limit`/`offset` and take an optional `status` filter.

## Currency exchange

Converts an amount between currencies at the latest daily rate from the
[ExchangeRate-API open access endpoint](https://www.exchangerate-api.com/docs/free) (keyless, about
160 currencies, including the Gulf currencies), and keeps each user's history. It is the template's
example of an outbound HTTP API behind a repository interface.

| Operation | REST | gRPC | GraphQL |
| --- | --- | --- | --- |
| Convert | `POST /v1/exchange/convert` | `ExchangeService/Convert` | |
| History | `GET /v1/exchange/history` | `ExchangeService/GetHistory` | `query exchangeHistory` |

- Request: `{"from": "OMR", "to": "CNY", "amount": "100.10"}`. Codes are case-insensitive.
- Amounts and rates are decimal strings, computed with exact decimals (no `float64`). The result is
  rounded half-even to 2 places.
- Rates update once a day and are not live market rates; the response carries the rate's `rate_date`.
  The provider's terms require a link back to it wherever the rates are shown (the web page has one).
- An unknown currency, or `from` equal to `to`, answers 400 (`InvalidArgument` on gRPC). History pages
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

## GraphQL

[gqlgen](https://github.com/99designs/gqlgen) on the REST port, covering tasks and the exchange
history. The schemas are [task.graphqls](../internal/controller/graphql/schema/task.graphqls) and
[exchange.graphqls](../internal/controller/graphql/schema/exchange.graphqls).

- With `DOCS_ENABLED=true`, a playground is at `/graphql/playground`. Put
  `{"Authorization": "Bearer <token>"}` in its headers pane.
- Resolvers check the bearer token themselves, so introspection works without one.
- Errors carry `extensions.code`: `UNAUTHENTICATED`, `BAD_USER_INPUT`, `NOT_FOUND`, `FORBIDDEN`,
  `INTERNAL`. An expired token is `UNAUTHENTICATED` in a 200 response.

## gRPC

Services are defined in [auth.proto](proto/v1/auth.proto), [task.proto](proto/v1/task.proto) and
[exchange.proto](proto/v1/exchange.proto); `make proto-v1` regenerates the Go code beside them. Server
reflection is on, so `grpcurl -plaintext 127.0.0.1:8081 list` works.

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
