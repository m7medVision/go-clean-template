# Feature flags

A feature can ship dark and be turned on or off without a deploy. The backend decides every flag;
the web client asks it for the caller's flags and never evaluates one itself.

Flags are read through the [OpenFeature](https://openfeature.dev) Go SDK, in `pkg/flags`, and
nowhere else. No vendor SDK appears outside it.

## Providers

| `FEATURE_FLAGS_FILE` | What answers a flag |
| --- | --- |
| set (`config/flags.yaml` in `.env.example`, `/config/flags.yaml` in compose) | [GO Feature Flag](https://gofeatureflag.org), in process, on that file. The app polls the file every 10 seconds: an edit takes effect within that time, with no restart |
| unset (production, as the template ships) | no provider: every flag answers its default in code |

A production provider (a GO Feature Flag relay proxy, flagd, a vendor) plugs in at `flags.New` in
`pkg/flags`: build its OpenFeature provider and pass it to `NewWithProvider`. Nothing else changes.
Ask first: it is a new service to run.

## The flags

| Flag | Default | Owner | What it turns off | Remove when |
| --- | --- | --- | --- | --- |
| `task-attributes` | on | the task domain | `PATCH /v1/tasks/{id}/attributes` and its GraphQL and gRPC counterparts answer not found; task reads leave `attributes` empty (GraphQL: an empty object) without reading MongoDB; the web hides the priority and labels columns and the dialog's attribute fields | attributes need no off switch |

## Adding a flag

1. A function in `internal/entity/flag.go` returning its `Flag` (key and default), listed in
   `Flags()`, with its owner and removal condition in its comment; a field in `FlagValues`.
2. Its entry in `config/flags.yaml` (GO Feature Flag's format: `variations` and a `defaultRule`).
3. The check, in the handler, never in a controller:
   `h.Flags.Bool(ctx, f.Key, f.Default, userID)`. The user ID is the evaluation context, so a
   provider can target users. A command whose feature is off answers `entity.ErrFeatureDisabled`.
4. Its value in `GetFlags` (`internal/usecase/flags/query`), then `make openapi`, `make web-api`,
   and `FlagValues` in `graphql/schema/flags_types.graphqls` with `make graphql`.
5. Its row in the table above.

Handlers are tested with `pkg/flags/flagstest`, which answers fixed values.

## Removing a flag

Every flag is removed once its rollout is done: delete its function, its `FlagValues` field and its
schema field (GraphQL: mark it `@deprecated` first while a client still reads it), its checks, its
entry in `config/flags.yaml` and its row above. The code keeps the path the flag was on.

## Observing evaluations

Two OpenFeature hooks, registered once in `pkg/flags`, so handlers carry no flag logging or tracing:

- the traces hook from OpenFeature's contrib package adds a `feature_flag.evaluation` event (key,
  variant, provider) to the message's span, so a trace tells "flag off" from "task missing";
- a log hook writes one `Warn` ("flag evaluation failed, default used", with the flag key and the
  error) when the provider fails and the default is used. `Warn`, not `Error`: the request still
  succeeds, and `Error` would reach GlitchTip. A successful evaluation is not logged.

There are no flag metrics yet.

## Never

Permissions as flags (roles are in the token); a vendor SDK outside `pkg/flags`; the browser
deciding a flag.
