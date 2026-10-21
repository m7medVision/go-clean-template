# Passkeys for go-clean-template

> **Shipped.** This idea is implemented; how it works is documented in
> [docs/passkeys.md](../passkeys.md). Two details changed on the way: sign-in is
> `/v1/auth/passkey/{begin,finish}` rather than `/v1/passkeys/login/...`, and the
> profile shows passkeys as a plain REST list rather than a data table. The open
> questions below were settled as: key derived from `JWT_SECRET` with HKDF;
> deleting the last passkey is allowed, since every account has a password.

## Problem Statement

How might we give developers who clone this template passkey sign-in that is
production-shaped (replay-safe, tested, documented) and built in the same layered
style as the rest of the codebase, without taking password login away?

## Recommended Direction

A new `passkey` domain alongside `user`. A signed-in user adds passkeys from the
profile screen. Anyone can sign in with a discoverable passkey, offered through
browser autofill (conditional UI), and receives the same JWT that password login
issues. Everything after the token is unchanged.

Ceremony state uses **sign on begin, burn on finish**. `begin` returns the
go-webauthn `SessionData` as a short-lived signed token: no database write, so
loading the login page for autofill costs nothing. `finish` verifies the token,
then inserts a hash of the challenge into `webauthn_used_challenges`. A
primary-key conflict means a replay and maps to `entity.ErrPasskeyChallengeUsed`.
Each challenge works once without one write per page load. The trade-off is
written up in `docs/` because it is the main thing this feature teaches.

It is REST only: ceremonies are browser-driven, and auth forms already live on
REST. gRPC and GraphQL keep accepting the JWT as they do today.

## Key Assumptions to Validate

- [ ] go-webauthn validates against a `SessionData` we rebuilt from a token,
      not one it stored. Spike: `BeginDiscoverableLogin` → JSON → sign → verify
      → `ValidateDiscoverableLogin` with a body parsed by
      `protocol.ParseCredentialRequestResponseBody`. Avoid the `*http.Request`
      helpers so this fits Huma. Fallback: direction A (store on begin).
- [ ] Ceremony tokens can never act as auth tokens. Test: a ceremony token
      sent as a Bearer token is rejected, and the reverse also fails.
      Use a distinct signing key or audience.
- [ ] A small Go ES256 software authenticator can drive both ceremonies in
      `integration-test/` (attestation "none", packed `authData`). Prototype
      it before the transport code. If it balloons, the "Integration
      tested" criterion is at risk.
- [ ] `WEBAUTHN_RP_ID` and `WEBAUTHN_RP_ORIGINS` (a list) work for `make dev`
      (Vite origin), compose and integration tests. Test: register and sign
      in once in each environment.

## MVP Scope

In:

- `internal/entity/passkey.go` (credential type, sentinels in `errors.go`)
- `internal/app/schema/webauthn_credentials.hcl`: credential ID, user_id,
  public key, sign count, AAGUID, backup flags, transports, name,
  created_at, last_used_at
- `internal/app/schema/webauthn_used_challenges.hcl`: challenge hash
  primary key, expires_at; expired rows deleted when `finish` inserts
- `repo.Passkey` and `usecase.Passkey` contracts, with implementations
  and traced decorators
- REST: `POST /v1/passkeys/register/{begin,finish}` (authed),
  `POST /v1/passkeys/login/{begin,finish}` (public; finish returns
  `response.Token`), `GET /v1/passkeys`, `PATCH /v1/passkeys/{id}`
  (rename), `DELETE /v1/passkeys/{id}`
- Web: "Sign in with a passkey" plus `autocomplete="username webauthn"`
  conditional UI on `login.tsx`; a passkey list (shadcn data table) and
  an add button on the profile screen; native `PublicKeyCredential`
  JSON helpers, no client library unless the browser APIs fall short
- Config: `WEBAUTHN_RP_ID`, `WEBAUTHN_RP_NAME`, `WEBAUTHN_RP_ORIGINS`,
  and the ceremony token TTL
- Tests: use-case unit tests (gomock), `integration-test/passkey_test.go`
  with the software authenticator, including a replay test
- Docs: `docs/api.md`, `docs/configuration.md`, a new `docs/passkeys.md`
  explaining sign on begin, burn on finish; README "What you get"

## Not Doing (and Why)

- **Passkey at sign-up / passwordless accounts.** It doubles the
  account-creation edge cases, and passwords stay first-class for v1.
- **Stateless-only ceremonies.** Replayable within the token TTL, and
  synced passkeys often report signCount 0. A template teaches what it does.
- **gRPC / GraphQL ceremonies.** Browser-driven flows; those transports
  already accept the JWT.
- **Attestation verification / AAGUID allowlists.** Enterprise policy;
  "none" is right for a general template.
- **Emails (new-passkey alert, recovery link).** A good v2 using the
  existing mail queue; it keeps v1 focused on the ceremonies.
- **Step-up re-auth for sensitive actions.** A v2 idea that builds on the
  login ceremony.
- **A generic "auth methods" abstraction.** Two concrete methods read
  better than one speculative framework.
- **Rate limiting.** C removes the page-load write that made it urgent; it
  belongs as general middleware, not something passkey-specific.

## Open Questions

- Ceremony signing key: a new required `WEBAUTHN_SESSION_SECRET`, or derived
  from `JWT_SECRET` (HKDF with a distinct label)? Derived is less setup.
  A separate key is easier to explain and rotate.
- Can a user delete their last passkey while having no password? It can't
  happen in v1 (no passwordless accounts), but the use case should still
  enforce the rule.
- Does the passkey list table need GraphQL? Under CLAUDE.md's rule, "tables
  read through GraphQL", it would need a `passkey` GraphQL schema even though
  the ceremonies are REST-only. Either accept that, or render it as a simple
  list rather than a data table.
