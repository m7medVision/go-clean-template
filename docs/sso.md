# SSO and roles

SSO sign-in through an OpenID Connect provider sits next to the app's own sign-ins. In development
the provider is [Keycloak](https://www.keycloak.org/), started by `make dev` and the compose files with
a checked-in realm. Any provider that implements OpenID Connect discovery works the same way: Okta,
Microsoft Entra ID, Auth0, Zitadel. Point `OIDC_ISSUER` at it and set the role mapping. No code
changes.

The app stays the one that issues tokens. A finished SSO sign-in returns the same JWT that password
login returns, with the caller's roles in a `roles` claim. Everything after that token,
on every transport, does not know how the user signed in.

The code, layer by layer:

| Layer | Where |
| --- | --- |
| Entity | [`internal/entity/role.go`](../internal/entity/role.go) (`Role`, `Principal`), sentinels in [`errors.go`](../internal/entity/errors.go) |
| Use case | [`internal/usecase/oidc`](../internal/usecase/oidc/oidc.go): the only package that imports [go-oidc](https://github.com/coreos/go-oidc) and `golang.org/x/oauth2` |
| Repository | [`internal/repo/persistent/identity`](../internal/repo/persistent/identity/identity.go), queries in [`queries/identity.sql`](../internal/repo/persistent/queries/identity.sql), table [`user_identities.hcl`](../internal/app/schema/user_identities.hcl) |
| Ceremony token | [`pkg/jwt/scoped.go`](../pkg/jwt/scoped.go) |
| REST | [`internal/controller/restapi/v1/oidc.go`](../internal/controller/restapi/v1/oidc.go) |
| Web | [`web/src/sso/`](../web/src/sso), with the button in [`use-sso-sign-in.ts`](../web/src/sso/use-sso-sign-in.ts) that [`routes/login.tsx`](../web/src/routes/login.tsx) offers, and the callback in [`routes/sso.callback.tsx`](../web/src/routes/sso.callback.tsx) |
| Dev provider | [`keycloak/realm.json`](../keycloak/realm.json), the `keycloak` service in [`docker-compose.yml`](../docker-compose.yml) |

## The sign-in

Authorization code with PKCE, with a confidential client. The browser holds the redirects, and the
backend only answers JSON.
It has the same shape as a passkey ceremony: <!-- scaffold:passkeys -->

1. `POST /v1/auth/oidc/begin` returns the provider's authorize URL and a `ceremony_token`. The token
   is a short-lived signed payload (`jwt.Scoped`, 10 minutes) holding the `state`, the `nonce` and
   the PKCE verifier. Nothing is written to the database.
2. The web client keeps the token in `sessionStorage` and sends the browser to the URL. The user signs
   in at the provider, which redirects to `APP_BASE_URL/sso/callback?code=…&state=…`.
3. The callback route posts `{code, state}` to `POST /v1/auth/oidc/finish`, with the token in the
   `X-Ceremony-Token` header. The use case checks the state against the token, exchanges the code with
   the client secret and the verifier, and verifies the ID token: signature, issuer, audience, expiry
   and nonce. It then finds or creates the user and returns `{token}`.

A code works once at the provider, so replaying a finished callback is refused. The redirect URI is
derived from `APP_BASE_URL`, and it must be one of the client's redirect URIs at the provider.

The provider is discovered on the first sign-in, not at startup. The app starts while the provider
is still booting or down, and `GET /v1/auth/oidc` never calls it. A failed discovery is retried on
the next sign-in and answers `502` until then.

## Users and identities

An identity at the provider is `(issuer, subject)`, and `user_identities` links it to a local user.
The first SSO sign-in of an identity creates the user, with the provider's `preferred_username`
(falling back to the email), its `email` and no password. Password login refuses a user without a
password.

Accounts are never linked by email. When the email or username is already taken by another
account, the sign-in is refused with a `409`: the provider's email claim is not proof that its holder
owns the local account (with Entra ID it is not even guaranteed verified). An ID token without an
email is refused (`401`); request the `email` scope from the provider.

## Roles

The app has two roles, `admin` and `user`. How the token carries them, and where they are checked, is
in [architecture.md](architecture.md#roles).

- Every other sign-in grants only `user`. Admin comes only from the provider.
- An SSO sign-in reads the provider's role names from the claim at `OIDC_ROLES_CLAIM` (a dotted path),
  maps each through `OIDC_ROLE_MAP`, and drops names that do not map to an app role. A sign-in that
  maps to nothing gets `user`.
- Roles are fixed when the token is issued. Removing someone from the admin group takes effect at
  their next sign-in, at most `JWT_TOKEN_EXPIRY` (15 minutes by default) later.

The one role-checked operation today is the admin task list (`Task.ListAll`): `GET /v1/admin/tasks`,
`query allTasks` and `TaskService/ListAllTasks`, shown in the web client as **All tasks**.

## Errors

| Case | Sentinel | REST |
| --- | --- | --- |
| `OIDC_ISSUER` is empty | `ErrOIDCDisabled` | `404` |
| The provider cannot be reached | `ErrOIDCUnavailable` | `502` |
| Bad, expired or replayed state, token, code or ID token; no email | `ErrOIDCCeremonyInvalid` | `401` |
| The email or username belongs to another account | `ErrAccountConflict` | `409` |
| A caller without `admin` lists every user's tasks | `ErrAdminRequired` | `403` (`FORBIDDEN`, `PERMISSION_DENIED`) |

## Keycloak in development

`make dev`, `make compose-up` and the integration stack start Keycloak 26 in dev mode, importing
[`keycloak/realm.json`](../keycloak/realm.json) on every start. The import is the whole state, so a
recreated container comes back the same:

| | |
| --- | --- |
| Realm | `app`, issuer `http://keycloak.lvh.me:8180/realms/app` |
| Client | `go-clean-template`, secret `go-clean-template-dev-secret`, redirect URIs for `http://localhost`, Vite (`http://localhost:3000`) and Caddy (`http://app.lvh.me`) |
| Users | `alice` / `alice` (realm roles `app-admin`, `app-user`); `bob` / `bob` (`app-user`) |
| Admin console | http://keycloak.lvh.me:8180, `admin` / `admin` |

The issuer is the same URL everywhere. `keycloak.lvh.me` resolves to 127.0.0.1 on the host, and the
container has that name as a network alias, listening on 8180 inside and out. The browser, the app on
the host (`make dev`), the app container (`make compose-up-all`) and the integration tests therefore
all see the same `iss`. `lvh.me` is public DNS. Offline, or behind a resolver that blocks answers
pointing at 127.0.0.1, add `127.0.0.1 keycloak.lvh.me` to `/etc/hosts`.

In `make compose-up-all` the web client is served on `http://app.lvh.me`, but the compose `app`
service sets `APP_BASE_URL=http://localhost`, which the links the app builds and the integration tests rely on.
The provider then sends the browser back to an origin Caddy does not serve. Until that value moves to
`http://app.lvh.me`, use SSO under `make dev`.

The realm puts the realm roles in the ID token (`realm_access.roles`, through a protocol mapper on
the client), so `OIDC_ROLES_CLAIM` keeps its default.

## Another provider

Register a confidential web client with the redirect URI `https://<your app>/sso/callback`, scopes
`openid email profile`, and a claim that carries the user's roles or groups. Then set the `OIDC_*`
variables ([configuration.md](configuration.md#oidc)):

| Provider | `OIDC_ISSUER` | `OIDC_ROLES_CLAIM` | Notes |
| --- | --- | --- | --- |
| Keycloak | `https://<host>/realms/<realm>` | `realm_access.roles` | Realm roles reach the ID token only through a protocol mapper with *Add to ID token* on, as in the dev realm. |
| Microsoft Entra ID | `https://login.microsoftonline.com/<tenant-id>/v2.0` | `roles` | Define App Roles on the app registration and assign users or groups to them. Prefer App Roles over the `groups` claim: past about 200 groups the token leaves them out ("groups overage"). |
| Okta | `https://<org>.okta.com/oauth2/default` | `groups` | Add a `groups` claim to the authorization server, filtered to the groups you map. The org authorization server does not add it by default. |

`OIDC_ROLE_MAP` maps the provider's names to `admin` or `user`, for example
`OIDC_ROLE_MAP=Tasks.Admin:admin` on Entra ID.
