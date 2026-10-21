# Passkeys

Passkeys (WebAuthn) sit next to passwords. A signed-in user adds passkeys from the profile screen;
anyone signs in with one, from a button or from the browser's autofill on the email field, and gets
the same JWT that password login returns. Everything after the token is unchanged, so gRPC and
GraphQL accept it as they always have.

The ceremonies are REST only: they are driven by the browser, and the auth forms already live on
REST. The endpoints are listed in [api.md](api.md#passkeys).

The code, layer by layer:

| Layer | Where |
| --- | --- |
| Entity | [`internal/entity/passkey.go`](../internal/entity/passkey.go), sentinels in [`errors.go`](../internal/entity/errors.go) |
| Use case | [`internal/usecase/passkey`](../internal/usecase/passkey/passkey.go): the only package that imports [go-webauthn](https://github.com/go-webauthn/webauthn) |
| Repository | [`internal/repo/persistent/passkey`](../internal/repo/persistent/passkey/passkey.go), queries in [`queries/passkey.sql`](../internal/repo/persistent/queries/passkey.sql) |
| Ceremony token | [`pkg/jwt/scoped.go`](../pkg/jwt/scoped.go) |
| REST | [`internal/controller/restapi/v1/passkey.go`](../internal/controller/restapi/v1/passkey.go) |
| Web | [`web/src/passkey/`](../web/src/passkey), and the login screen in [`web/src/routes/login.tsx`](../web/src/routes/login.tsx) |

go-webauthn stays inside the use case, the way `user` keeps bcrypt to itself. Controllers pass the
browser's JSON in as bytes and hand the options JSON back out; no WebAuthn type reaches `entity` or a
controller.

## Ceremonies

Each ceremony is two requests. `begin` answers `{options, ceremony_token}`: `options` is the WebAuthn
options JSON for the browser, and `ceremony_token` is the ceremony's state, signed. `finish` takes
the browser's `PublicKeyCredential.toJSON()` as the body and the token in the `X-Ceremony-Token`
request header. It is a header, not a query parameter, because the request logger and the panic
recovery log the request URI, and a query string also ends up in proxy access logs.

The finish body is the one REST input that is not decoded into fields. Its shape belongs to the
WebAuthn spec and go-webauthn parses and checks it, so the controller keeps the bytes as they arrived
in `request.PasskeyCredential`, a `json.RawMessage` whose schema is an open object. Huma's `RawBody`
would keep the bytes too, but it documents the body as a binary string, and the web client's
generated types would then have no JSON body to send. The begin `options` are the same in reverse
(`response.PasskeyOptions`).

### Adding a passkey

Signed in, from the Passkeys card on `/profile`, and signed in recently.

1. `POST /v1/passkeys/register/begin` first checks the sign-in is recent (below), then loads the
   user and their passkeys and returns creation options. The user's existing passkeys are listed in
   `excludeCredentials`, so an authenticator that already holds one says so instead of making a
   second. The options ask for a discoverable credential (resident key required), user
   verification required and attestation `none`.
2. The browser runs `navigator.credentials.create` with those options.
3. `POST /v1/passkeys/register/finish?name=…`, with the token in `X-Ceremony-Token`, verifies the token, parses the
   credential, and has go-webauthn check it against the ceremony. The ceremony must belong to the
   caller: a token begun by another user fails. The passkey is stored under `name`, trimmed, or
   `Passkey` when it is blank, and the answer is `201` with the passkey.

Registration needs no burn (below). The credential ID is the table's primary key, so finishing the
same registration twice is a `409`, and only the signed-in user's own ceremony can finish at all.

#### A recent sign-in

A passkey outlives the session that adds it: it keeps signing in after the JWT expires and after a
password change. Without a check, anyone holding a stolen JWT could add their own passkey and keep
the account. So `begin` requires that the caller signed in recently: the auth middleware
(`middleware.AuthTime`) passes the token's `iat` to the use case, and a token older than
`WEBAUTHN_REAUTH_MAX_AGE` (5 minutes by default), or one without an `iat`, is
`entity.ErrPasskeyReauthRequired`. The rule lives in the use case (`BeginRegistration`), not the
controller.

The controller answers it with a `401` problem whose `detail` is exactly `reauthentication
required`. That string is the contract clients match on; it does not change. The token is still
valid for everything else, so the web client does not sign the user out on it: the Add dialog asks
for the password, signs in again through `POST /v1/auth/login` with the user's email, stores the new
token and starts the ceremony again. A passkey login also counts as a sign-in: its JWT is as fresh
as a password one.

#### User verification

Both ceremonies require user verification (`userVerification: "required"`): the authenticator must
check a PIN or biometric, not just a touch. go-webauthn carries the requirement in the ceremony
state and rejects a credential or assertion whose authenticator data lacks the UV flag, as
`ErrPasskeyCeremonyInvalid`. A passkey is a single factor that stands in for the password, so
presence alone is not enough.

### Signing in

Public, from `/login`.

1. `POST /v1/auth/passkey/begin` returns request options with no `allowCredentials`: the login is
   discoverable, so the authenticator offers whichever passkeys it holds for this site. Nothing is
   written.
2. The browser runs `navigator.credentials.get`, either for the "Sign in with a passkey" button or
   with `mediation: "conditional"`, which puts the passkeys in the autofill of the email field
   (`autocomplete="username webauthn"`).
3. `POST /v1/auth/passkey/finish`, with the token in `X-Ceremony-Token`, then runs, in this order:
   1. verify the ceremony token;
   2. parse the assertion;
   3. look the owner up by the assertion's user handle (the user's UUID as 16 bytes), and have
      go-webauthn check the signature against that user's stored passkeys;
   4. reject a sign count that did not advance (see below);
   5. **burn the challenge**;
   6. store the credential's new state and stamp `last_used_at`;
   7. issue the auth JWT and answer `{token}`, the same body password login returns.

The burn comes after the signature check and before anything is written or issued. Only a signature
from a registered passkey can consume a challenge, so someone holding a captured token and a forged
body cannot burn a ceremony its owner is still completing. A replayed assertion still fails, because
the insert that burns the challenge is the check.

A sign count that went backwards or stood still is treated as a possible cloned authenticator and
rejected. The check applies only when the counters are non-zero: synced passkeys (iCloud Keychain,
Google Password Manager) report `0` every time, and those always pass.

## Sign on begin, burn on finish

The state a ceremony carries from `begin` to `finish` (the challenge, the user it was begun for, the
expiry) has to live somewhere. There are three places to put it, and this template uses the third.

**Store on begin.** `begin` writes a row, `finish` reads and deletes it. Every challenge works once.
But the login page starts a conditional (autofill) ceremony as soon as it loads, so this is a
database write for every visit to `/login`, by anyone, signed in or not. That is a cheap way to make
the server write, and it needs a cleanup job and, soon, rate limiting.

**Stateless.** `begin` signs the state into a token and stores nothing; `finish` checks the token.
No writes at all, but nothing stops the same token and assertion from being sent again: a login
that someone captures (a logged request, a proxy) works again until the token expires. The
authenticator's signature counter would catch some replays, but synced passkeys report `0`, so it
cannot be relied on.

**Sign on begin, burn on finish.** `begin` is stateless, as above, so loading the login page costs
nothing. `finish`, once the assertion verifies, inserts a SHA-256 hash of the challenge into
`webauthn_used_challenges`:

```sql
INSERT INTO webauthn_used_challenges (challenge_hash, expires_at)
VALUES ($1, $2)
ON CONFLICT (challenge_hash) DO NOTHING
RETURNING challenge_hash;
```

No row back means the challenge was used before: `entity.ErrPasskeyChallengeUsed`, a `409`. The
database writes only for logins that verified, and each challenge works once. The row only has to
outlive the token, so `expires_at` is the ceremony's expiry, and the same finish path deletes the
expired rows first (`DeleteExpiredChallenges`). There is no cleanup job.

### The ceremony token

The token is an HS256 JWT ([`jwt.Scoped`](../pkg/jwt/scoped.go)) whose payload is go-webauthn's
`SessionData` as JSON. go-webauthn's `CreateCredential` and `ValidatePasskeyLogin` take that
`SessionData` by value, which is what makes a rebuilt session as good as a stored one.

- **Its key is derived, not configured.** It is `HKDF-SHA256(JWT_SECRET, info "webauthn-ceremony")`,
  so there is no extra secret to set or rotate. Rotating `JWT_SECRET` ends in-flight ceremonies along
  with every session.
- **It can never pass for an auth token, or the reverse.** The key differs from the one auth tokens
  are signed with, and the audience is the purpose, `webauthn-ceremony`, which `Verify` requires.
  `pkg/jwt`'s tests check both directions.
- **It lives `WEBAUTHN_CEREMONY_TIMEOUT`** (5 minutes by default). The same value is the `timeout` in
  the options the browser gets, and go-webauthn enforces it again on finish.

A token that is forged, expired, from another purpose, or whose payload does not decode is
`entity.ErrPasskeyCeremonyInvalid`.

## Data model

Two tables, declared in [`internal/app/schema/`](../internal/app/schema) like the rest.

`webauthn_credentials` ([HCL](../internal/app/schema/webauthn_credentials.hcl)), one row per passkey:

| Column | Type | |
| --- | --- | --- |
| `id` | `bytea`, primary key | the credential ID; unpadded base64url in JSON and in `/v1/passkeys/{id}` |
| `user_id` | `uuid` | the owner; foreign key to `users`, `ON DELETE CASCADE`, indexed |
| `name` | `text` | the user's label, at most 64 characters |
| `credential` | `jsonb` | go-webauthn's whole `Credential`: public key, sign count, flags, AAGUID, transports |
| `created_at` | `timestamptz` | |
| `last_used_at` | `timestamptz`, nullable | stamped by every sign-in |

Only what is queried or shown is a column. The rest of the credential lives in `jsonb`, so a new
field in the library needs no schema change. The `credential` column never leaves the server:
`entity.Passkey` tags it `json:"-"`, and the API returns `id`, `name`, `created_at` and
`last_used_at`.

`webauthn_used_challenges` ([HCL](../internal/app/schema/webauthn_used_challenges.hcl)), one row per
burned login challenge:

| Column | Type | |
| --- | --- | --- |
| `challenge_hash` | `bytea`, primary key | SHA-256 of the challenge |
| `expires_at` | `timestamptz`, indexed | the ceremony's expiry; deleted after it |

The users table is unchanged. The WebAuthn user handle is the user's UUID.

## Errors

Every failure of the ceremony itself is one sentinel, `entity.ErrPasskeyCeremonyInvalid`: a bad,
expired or foreign token, a body that does not parse, a signature that does not verify, an unknown
user handle or credential (a deleted passkey included), a token begun for another user, a sign count
that did not advance, or an authenticator that did not verify the user. The app log has the
reason; the caller gets a generic message.

| Sentinel | `register/finish` | `auth/passkey/finish` | `PATCH`, `DELETE /v1/passkeys/{id}` |
| --- | --- | --- | --- |
| `ErrPasskeyCeremonyInvalid` | `400` | `401` | |
| `ErrPasskeyExists` | `409` | | |
| `ErrPasskeyChallengeUsed` | | `409` | |
| `ErrPasskeyNotFound` | | | `404` |

- On sign-in an invalid ceremony is a `401`, like a wrong password. On registration the caller is
  already signed in, so it is a `400`.
- Another user's passkey is a `404`, not a `403`, so passkey IDs do not leak. So is an ID that is not
  base64url.
- A missing `X-Ceremony-Token` header, or a name over 64 characters, fails Huma's validation with
  `422`. A token sent in the old `ceremony_token` query parameter is not read, so it is missing too.
- `register/begin` answers `404` if the signed-in user no longer exists, and `401` with the detail
  `reauthentication required` for `ErrPasskeyReauthRequired` (a sign-in that is not recent).

## Relying party setup

A passkey is bound to its RP ID, and a browser runs a ceremony only on an origin whose host is the RP
ID or a subdomain of it, in a secure context (HTTPS, or `http://localhost`). go-webauthn also checks
that the origin in the browser's signed client data is one of `WEBAUTHN_RP_ORIGINS`. Both settings
are described in [configuration.md](configuration.md#webauthn); the defaults fit local development.

### `make dev`

The web client runs on the Vite dev server at <http://localhost:3000>, which proxies `/v1` to the app,
so the origin the browser reports is `http://localhost:3000` and the RP ID is `localhost`. The
defaults cover this.

Open the client as `localhost`, not `127.0.0.1`: an RP ID cannot be an IP address, and a passkey made
for `localhost` is not offered on another host.

### Compose

The compose `app` service sets the same values as `.env.example`: RP ID `localhost` and the origins
`http://localhost`, `http://localhost:3000` and `http://localhost:8080`. The integration tests reach
the app as host `app`, but the virtual authenticator declares `http://localhost:8080` as its origin,
which is what the server checks.

The full stack behind Caddy serves the client at `http://app.lvh.me`. That host is not under
`localhost`, and plain HTTP on it is not a secure context, so browsers do not offer passkeys there.
To try passkeys in the browser, use `make dev`. To use them behind Caddy, serve the site over HTTPS
and set the RP values to its host, as in production.

### Production

- Serve the client over HTTPS. Browsers refuse WebAuthn on plain HTTP anywhere but `localhost`.
- Set `WEBAUTHN_RP_ID` to the registrable domain, such as `example.com`. Passkeys made for it then
  work on `example.com` and every subdomain, so the client can move from `app.example.com` to
  `www.example.com` without losing them.
- Set `WEBAUTHN_RP_ORIGINS` to the exact origins the client is served from, scheme and port
  included: `https://app.example.com`. Drop the `localhost` defaults.
- Choose the RP ID before anyone registers. Changing it later orphans every passkey already made:
  the browser will not offer them under the new ID.
- Set `WEBAUTHN_RP_NAME` to the name users should see in the passkey prompt and their password
  manager.

## Testing

[virtualwebauthn](https://github.com/descope/virtualwebauthn) is a software authenticator: it reads
the options JSON, makes an EC2 key, and writes the attestation and assertion JSON a browser would.
Both test suites use it with the real go-webauthn, so the ceremonies are checked end to end, not
mocked.

- **Unit**, in [`internal/usecase/passkey_test.go`](../internal/usecase/passkey_test.go), with gomock
  repositories: registration and login succeed; `begin` refuses a sign-in older than the max age,
  or one without an issue time, before reading anything; a tampered, expired or foreign token fails;
  so does an attestation or assertion without the user-verified flag, an assertion for another
  ceremony's challenge, an unknown credential or user handle, a credential
  signed by another key, and a sign-count regression, none of them reaching the burn; a sign count
  of `0` passes; a second finish is a replay; the login JWT parses to the right user, and the burn
  happens before it is issued.
- **Integration**, in [`integration-test/passkey_test.go`](../integration-test/passkey_test.go), over
  REST: `TestHTTPPasskeyLoginV1` registers a user with a password, adds a passkey, lists it, signs
  in with it, calls `/v1/user/profile` with that JWT, replays the finish body for a `409`, and checks
  a ceremony token sent in the query string is not read (`422`). The stale-sign-in refusal is covered
  by the unit and handler tests, since the suite's tokens are always fresh.
  `TestHTTPPasskeyManageV1` renames and deletes the passkey, checks another user gets `404`, and
  checks the deleted passkey no longer signs in (`401`).

Run them with `make test` and `make compose-up-integration-test`
([development.md](development.md#tests)).

## Not in v1

- **Passkey at sign-up, or accounts with no password.** Passwords stay first-class, so deleting the
  last passkey is always allowed: every account still has its password.
- **gRPC and GraphQL ceremonies.** The ceremonies are browser-driven; those transports accept the
  JWT a passkey login returns.
- **Attestation verification and AAGUID allowlists.** That is enterprise policy; attestation `none`
  is right for a general template.
- **Emails** (a new-passkey alert, a recovery link). A natural follow-up on the existing mail queue.
- **Step-up re-authentication with a passkey.** Adding a passkey re-asks for the password; other
  sensitive actions, and re-authenticating with an existing passkey, are not covered.
- **A generic "auth methods" abstraction.** Two concrete methods read better than one speculative
  framework.
- **Rate limiting.** Sign on begin removed the page-load write that made it urgent; it belongs in
  general middleware, not in the passkey code.
