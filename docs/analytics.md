# Product analytics

Analytics is optional: with no vendor configured nothing is captured and nothing else changes. The
template stores no analytics; a vendor (PostHog today) does. No event carries an email, a name,
task text or an amount, and the types make that impossible.

## The catalogue

| Event | Sent by | Source | Properties |
| --- | --- | --- | --- |
| `user_registered` | worker | `UserRegistered`, `UserRegisteredViaOIDC` | `method` (`password`, `oidc`) |
| `task_created` | worker | `TaskCreated` | — |
| `task_status_changed` | worker | `TaskTransitioned` | `from`, `to` |
| `currency_converted` | worker | `CurrencyConverted` | `from`, `to` (currencies, not the amount) |
| `user_logged_in` | web | a sign-in that succeeded (password, passkey, OIDC) | `method` |
| `task_attributes_updated` | web | the attributes mutation succeeded | `fields` (names, not values) |
| `page_viewed` | web | every route change | `path` (the route template, `/tasks/$id`) |

The server's events are typed in [`internal/entity/analytics`](../internal/entity/analytics), one
file per domain, listed by `analytics.Events()`; the web's names are in
`web/src/lib/analytics-events.ts`. The names are ours: a sink maps them to its vendor's
(PostHog: `page_viewed` → `$pageview`). The distinct ID is the user ID on both sides, so server and
web events join.

## How a server event travels

```
domain event ─▶ capture-<domain> event handler ─▶ repo.Analytics: one message per sink
                                                    │ analytics-<sink> topic (NATS)
                                                    ▼
               worker v1 AnalyticsController ─▶ DeliverAnalytics ─▶ repo.AnalyticsSink.Send
```

- **No outbox.** Only events that follow a stored event are sent from the server: the event store is
  the delivery guarantee, as for the welcome email. Sign-in is a query and task attributes live in
  Mongo, so the web client captures those two.
- **No history.** The `capture-*` handlers implement `repo.StartsAtHead`, so the first time they
  run their checkpoints start at the store's last event: configuring a vendor on a database with
  history sends none of it. `*Imported` events are never mapped.
- **Exactly once per vendor.** An event's ID is `<stream id>-<stream version>`, the same on every
  replay; a sink turns it into its vendor's dedup key (PostHog: a `uuid` derived from it).
- **One topic per sink.** A vendor that is down retries and goes to `poison` alone; the others
  receive every event once. An event the vendor refuses (`analytics.ErrRejected`, a 4xx) goes to
  `poison` at once.

## Sinks (the strategy)

A sink is a `repo.AnalyticsSink` (`Name`, `Send`) in `internal/repo/analytics/<vendor>/`, the only
package that knows its vendor. A vendor is on when its settings are present; several can be on at
once. To add one:

1. `internal/repo/analytics/<vendor>/`: the sink, sending one event per call and waiting for the
   answer, so a failure reaches the queue; a refusal is `analytics.ErrRejected`. Its test runs
   `sinktest.Run` against a fake of the vendor.
2. `config/analytics.go`: its settings section, in `.env.example` and the compose `environment`.
3. `analyticsSinks` in `internal/app/analytics.go`: one `if` that builds it when its settings are set.

A vendor added later receives events from the moment it is configured.

## The web client

`web/src/lib/analytics.ts` is the only file that imports `posthog-js` (`make web-check` enforces
it). Without `VITE_POSTHOG_KEY` it loads nothing. It sends page views with the route template,
identifies the user by ID after sign-in and resets on sign-out. Session replay masks every input
and all text. Autocapture and PostHog's own page views are off.

## Not in this module

No consent banner, no GDPR delete, no person properties, surveys, experiments or group analytics.
