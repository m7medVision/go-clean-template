# Web client

`web/` is a client-rendered React app: [TanStack Router](https://tanstack.com/router) (file-based routes
in `web/src/routes/`) and [TanStack Query](https://tanstack.com/query), built and checked with
[Vite+](https://viteplus.dev/) (`vp`: Vite, Oxlint, Oxfmt, Vitest) and bun. It imports nothing from
Go, and nothing in Go imports it.

```sh
make web-deps   # once
make web-dev    # http://localhost:3000, proxying the API to the app on HTTP_PORT
make web-check  # format, lint, type-check and test
```

## Screens

Signed-in screens (`/tasks`, `/exchange`, `/profile`) share a sidebar layout under the `_authed` route;
`/login` and `/register` use a centred card.

`/exchange` converts an amount (Omani Rial to Chinese Yuan by default) with a REST form and lists the
history in a data table. It links to the rate provider, as the provider's terms require.

The UI is built from [shadcn/ui](https://ui.shadcn.com/) components (`web/components.json`; generated
into `web/src/components/ui/`). Add one with `vp dlx shadcn@latest add <component>` from `web/`.

## Talking to the API

The client and the API share one origin (the Vite proxy in development, Caddy in the full stack), so
there is no CORS. Client routes must not use the paths the Go app owns: `/v1`, `/graphql`, `/docs`,
`/openapi.*`, `/schemas`, `/healthz`, `/metrics`.

Types are generated, never hand-written:

- **REST**: `web/src/api/schema.gen.ts` comes from [`docs/openapi/v1.json`](openapi/v1.json) via
  `make web-api`, so a REST contract change shows up as a type error here.
- **GraphQL**: operations are written with `graphql(...)` in `web/src/` and typed by `make web-graphql`
  (GraphQL Code Generator) from the gqlgen schema.

Every request, REST or GraphQL, goes through TanStack Query. Each domain keeps its query and mutation
options in `web/src/<domain>/api.ts`.

- **Tables** read over GraphQL and select only the columns they show. They are shadcn data tables on
  [TanStack Table](https://tanstack.com/table), paged on the server with `limit`/`offset`.
- **Forms** and single-record screens (auth, profile, the exchange form) use REST. Forms are
  [TanStack Form](https://tanstack.com/form) with shadcn `Field` components and a
  [Zod](https://zod.dev/) schema per domain (`web/src/components/form/`). A currency picker is
  `field.SelectField`.

## Token storage

The JWT lives in memory, mirrored to `sessionStorage` (`web/src/auth/token.ts`) so it survives a reload
but not closing the tab. Scripts can read it, so the Content-Security-Policy that Caddy sets on the
client (see [development.md](development.md#full-stack-behind-caddy)) is what protects it. GraphQL
reports an expired token as `UNAUTHENTICATED` in a 200 response; the client signs out on that just as
it does on a REST 401.
