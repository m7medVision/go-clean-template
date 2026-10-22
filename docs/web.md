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

Signed-in screens (`/tasks`, `/exchange`, `/profile`, and `/admin/tasks` for admins) share a sidebar
layout under the `_authed` route; `/login` and `/register` use a centred card. Under its password
form, `/login` offers a button for each other way of signing in: a `SignInMethod`
(`web/src/auth/sign-in-method.ts`) built by a hook in that feature's folder.
So does `/sso/callback`. <!-- scaffold:sso -->

`/admin/tasks` ("All tasks") is a read-only data table of every user's tasks over GraphQL `allTasks`.
The sidebar shows its link when the token's `roles` claim includes `admin` (`web/src/auth/roles.ts`
reads the claim without verifying it: a UI hint only, since the server checks the role). A user who
opens the URL anyway gets the server's `admin role required` in the route's error view.

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

<!-- scaffold:passkeys:begin -->
## Passkeys

The browser half of a WebAuthn ceremony is in `web/src/passkey/webauthn.ts`, on the browser's own
JSON helpers and no client library: `PublicKeyCredential.parseCreationOptionsFromJSON` and
`parseRequestOptionsFromJSON` read the server's `options`, and the credential's `toJSON()` is the
finish body, sent as it is. A browser without those helpers gets no passkey buttons
(`passkeysSupported`). A dismissed or timed-out prompt (`NotAllowedError`) and an aborted request
resolve to `null` and show nothing; only the server's rejections reach `FormError`.

- **`/profile`** has a Passkeys card: a plain list (name, created, last used) over REST, not a data
  table, with add, rename and delete. The name fields are TanStack Form + Zod
  (`web/src/passkey/schema.ts`). Adding a passkey needs a recent sign-in: when `register/begin`
  answers the `reauthentication required` problem (`isReauthRequired`, `web/src/auth/reauth.ts`),
  the Add dialog asks for the password, signs in again through `POST /v1/auth/login` with the
  profile's email, stores the new token the way `/login` does, and continues the ceremony. The API
  client does not treat that `401` as a lost session, so it does not sign the user out.
- **`/login`** has a "Sign in with a passkey" button, and offers passkeys in the email field's
  autofill (`autocomplete="username webauthn"`) through a conditional request started on mount.
  `usePasskeyLogin` keeps one ceremony in flight: the button aborts the autofill request and starts
  it again if the button flow ends without signing in, and leaving the page aborts it. Both paths
  sign in exactly as password login does.

Run it under `make dev` and open the client as `localhost`; see
[passkeys.md](passkeys.md#relying-party-setup).

<!-- scaffold:passkeys:end -->
<!-- scaffold:sso:begin -->
## SSO

When `GET /v1/auth/oidc` reports SSO as enabled, `/login` shows "Sign in with SSO". It calls `begin`
(`web/src/sso/api.ts`), keeps the `ceremony_token` and the post-login redirect in `sessionStorage`
(`web/src/sso/ceremony.ts`, taken once), and sends the browser to the provider. The provider comes back
to `/sso/callback?code=…&state=…` on a full page load. The route posts both to `finish` exactly once
(a code works once, StrictMode or not), stores the JWT as `/login` does, and replaces the history
entry with the redirect. A refusal from the provider (`?error=…`), a link without a code, or a missing
ceremony (another tab, a reload) shows a message and a way back to `/login` (`callbackProblem`,
`web/src/sso/callback.ts`). See [sso.md](sso.md).

<!-- scaffold:sso:end -->
## Token storage

The JWT lives in memory, mirrored to `sessionStorage` (`web/src/auth/token.ts`) so it survives a reload
but not closing the tab. Scripts can read it, so the Content-Security-Policy that Caddy sets on the
client (see [development.md](development.md#full-stack-behind-caddy)) is what protects it. GraphQL
reports an expired token as `UNAUTHENTICATED` in a 200 response; the client signs out on that just as
it does on a REST 401.
