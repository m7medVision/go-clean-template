# OIDC Sign-in with Keycloak

## Problem Statement

How might we let the template sign users in through an enterprise identity provider (Keycloak by
default, Okta or Entra ID by configuration) alongside the existing password and passkey login, with
roles from the IdP enforced the same way across REST, gRPC and GraphQL, and with `make dev` working
on the first try?

## Recommended Direction

**Direction A: the backend brokers the login and issues the app's own JWT.** One provider per
deployment; Keycloak ships with the template.

**PR 1: roles.** The codebase has no role system today, only ownership (`ErrTaskForbidden`). Add
`entity.Role` (`admin`, `user`), a `Principal{UserID, Roles}` that every transport's auth middleware
places in the context, a `roles` claim in the app JWT, and one real rule enforced **in the use
case**: an admin can list every user's tasks. Local (password and passkey) users always get `user`.

**PR 2: OIDC.** The backend runs authorization code + PKCE (`coreos/go-oidc/v3`,
`golang.org/x/oauth2`), verifies the ID token, finds or creates the user through a
`user_identities (issuer, subject) → user_id` table, maps the configured roles claim to
`entity.Role`, and issues the same app JWT that password and passkey login issue. Nothing past the
middleware knows how the user signed in. OIDC is on when `OIDC_ISSUER` is set; there is no on/off
flag.

The dev experience is part of the feature: Keycloak in compose (`start-dev --import-realm`,
embedded storage) with a checked-in realm (`alice` = admin, `bob` = user), a confidential client
for the app, a test-only direct-grant client for integration tests, and the issuer-URL split solved
by pinning `KC_HOSTNAME` to the public URL while the app fetches discovery over the compose network.

### Why not direction B (SPA holds Keycloak tokens, backend is a resource server)

Faster role revocation, but two token types to accept across three transports, gRPC and GraphQL
clients needing Keycloak tokens, and a harder dev issuer problem. A short app-JWT lifetime covers
most of the revocation gap.

## Key Assumptions to Validate

- [ ] The issuer split works: spike `KC_HOSTNAME=http://localhost:8180` with discovery over
      `http://keycloak:8080/realms/app` (go-oidc `InsecureIssuerURLContext`), from inside and
      outside compose, before writing anything else.
- [ ] Two roles show RBAC clearly: the "admin lists all tasks" rule across all three transports
      reads as an example, not a framework.
- [ ] A 15-minute default for the existing `JWT_TOKEN_EXPIRY` does not break the web app: check how
      `web/src/auth/token.ts` handles expiry.
- [ ] Claim mapping covers the big three: unit-test the mapper against recorded Keycloak
      (`realm_access.roles`), Entra ID (`roles`, App Roles) and Okta (`groups`) ID-token payloads in
      `testdata/`.

## MVP Scope

**In:**

- `entity.Role`, `Principal`, `roles` JWT claim, role-aware auth on REST, gRPC and GraphQL, one
  admin rule in the task use case.
- `user_identities` table (HCL), repo and use case for OIDC login, `GET /v1/auth/oidc/login` and
  `GET /v1/auth/oidc/callback`.
- Config: `OIDC_ISSUER` (empty = OIDC off), `OIDC_DISCOVERY_URL`, `OIDC_CLIENT_ID`,
  `OIDC_CLIENT_SECRET`, `OIDC_REDIRECT_URL`, `OIDC_ROLES_CLAIM`, `OIDC_ROLE_MAP`.
- Keycloak service in compose and `make dev`, realm export in `deploy/keycloak/`.
- "Sign in with SSO" button in `web/`, shown when OIDC is configured.
- Integration test: direct-grant token for `alice`, run the login exchange, assert the admin rule.
- Docs: `docs/auth.md` (flow, roles, Okta and Entra ID setup notes), the configuration table, the
  development service table.

**Out:** everything below.

## Not Doing (and Why)

- **SSO-only mode / a flag to disable local login:** no new toggle env vars. Local login always
  stays on.
- **Multiple providers at once:** an enterprise has one directory. Internals take an issuer, so a
  list later is additive.
- **Auto-linking accounts by email:** the nOAuth takeover class (Entra's `email` is not guaranteed
  verified). The first SSO login creates a new user; linking to an existing account happens only
  while signed in.
- **Direction B (SPA-held IdP tokens):** see above.
- **Refresh tokens and back-channel logout:** a 15-minute app JWT and re-login are enough for a
  template.
- **Fine-grained permissions, ReBAC, OpenFGA:** teaches a framework, not clean architecture.
- **SCIM provisioning, multi-tenant realms:** product features, not template features.
- **Keycloak on the app's Postgres:** the stack runs exactly one database. Embedded storage plus
  realm import is enough.
- **Okta and Entra ID configs tested in CI:** they cannot run in compose; documented only.

## Open Questions

- Should an admin be able to promote local users (which puts a roles column in the database), or do
  local users stay fixed at `user`?
- Is host port 8180 free for Keycloak?
- Can `OIDC_DISCOVERY_URL` be dropped, by having the app container reach Keycloak at the public
  hostname (compose `extra_hosts`), so the config is one variable shorter?

## As built

Built as planned (direction A), with these changes. The reference is [docs/sso.md](../sso.md).

- **The web client owns the redirect.** The backend answers JSON only (`begin` → `{url,
  ceremony_token}`, `finish` → `{token}`), shaped like a passkey ceremony. The redirect URI is
  `APP_BASE_URL/sso/callback`, so there is no `OIDC_REDIRECT_URL`.
- **One issuer URL, `keycloak.lvh.me:8180`,** reaches Keycloak from the browser, the host and the
  containers, so there is no `OIDC_DISCOVERY_URL` and no `OIDC_ENABLED`: SSO is on when `OIDC_ISSUER`
  is set.
- **The integration test drives Keycloak's login form** over HTTP, with no direct-grant client.
- **A bad sign-in is `401`** (as for passkey login), not `400`; an unreachable provider is `502`.
- **`JWT_TOKEN_EXPIRY` defaults to 15m** to bound how long a revoked role lingers.
- **Open:** in `make compose-up-all` the compose `APP_BASE_URL` (`http://localhost`) is not the origin
  Caddy serves (`http://app.lvh.me`), so SSO works under `make dev` only until that value moves.
