# Spec: product analytics, PostHog first

Module id: `posthog` · third of four (see [README.md](README.md#observability-and-messaging)) ·
Depends on `nats-queue`, `task-attributes` (both merged) · Status: **spec revised 2026-11-09, awaiting
review**

## Objective

Product sees what users do, in PostHog first and in any other analytics vendor added later, without
the template storing analytics or leaking PII. Optional: with no vendor configured, nothing is
captured and nothing else changes. Done means:

- seven typed events reach PostHog, enough for an activation funnel
  (`user_registered` → `task_created` → `task_status_changed` to done);
- the web client sends page views, identifies the user after sign-in, and records sessions with
  every input and all task text masked;
- no event carries an email, a name, task text or an amount, and the types make that impossible.

## Design (decided)

1. **PostHog stores; we only send.** No analytics in Postgres or Mongo.
2. **A typed catalogue** in its own package, `internal/entity/analytics/` (as
   `internal/entity/mongo/task/`), one file per domain: one struct per server event with typed
   properties, `analytics.Events()`, a docs table, a test that every mapped domain event is listed. No
   `map[string]any`, no free-form `Capture`. The web client takes its names from one file,
   `web/src/lib/analytics-events.ts`.
3. **Launch events:**

   | Event | Source | Properties |
   | --- | --- | --- |
   | `user_registered` | server: `UserRegistered`, `UserRegisteredViaOIDC` | `method` (`password`, `oidc`) |
   | `task_created` | server: `TaskCreated` | — |
   | `task_status_changed` | server: `TaskTransitioned` | `from`, `to` |
   | `currency_converted` | server: `CurrencyConverted` | `from`, `to` (currencies, not amount) |
   | `user_logged_in` | web: after a sign-in succeeds (password, passkey, OIDC) | `method` |
   | `task_attributes_updated` | web: after the attributes mutation succeeds | `fields` (names, not values) |
   | `page_viewed` | web: every route change | `path` (the route template, `/tasks/$id`) |

   The names are ours. A vendor adapter maps them to the vendor's own (PostHog: `page_viewed` →
   `$pageview`).

4. **No outbox.** The outbox stays a future feature. Server events are only those that follow a
   stored event: an event handler per domain (`capture-*`) runs in the worker, which has the broker,
   and publishes through `repo.Analytics` (item 6). The event store is the delivery guarantee, as for the
   welcome email. Sign-in is a query and attributes live in Mongo, so neither has a stored event:
   the web client captures those two after the request succeeds.
5. **Exactly-once per vendor.** `analytics.Event` carries an `ID` derived from the stored
   event (`<stream id>-<stream version>`, the same on every replay); each adapter turns it into its vendor's dedup key (PostHog: `uuid`), so a handler
   running again after a crash, or a queue retry, is dropped by the vendor. A 4xx is
   `broker.Permanent`; give-up logs and drops.
6. **Strategy pattern, several vendors at once.**
   - `repo.AnalyticsSink` is the strategy: `Name() string` and
     `Send(ctx, *analytics.Event) error`. One adapter per vendor in
     `internal/repo/analytics/<vendor>/`, the only package importing that vendor's SDK.
     PostHog (`posthog-go`) is the only adapter in this module.
   - A queue message sends the `DeliverAnalytics` command, whose handler picks the sink by name,
     as mail sends `DeliverEmail`. Each sink sends one event and waits for the answer (not
     `posthog-go`'s background `Enqueue`), so a failure reaches the queue and is retried.
   - The worker builds the list of sinks from the vendors whose settings are present
     (`internal/app/analytics.go`); zero, one or several.
   - **Fan-out at publish, one topic per sink.** `repo.Analytics.Capture` publishes one message
     per configured sink to `analytics-<sink>` with the ID `<sink>-<event id>`; each topic has its
     own handler bound to its sink. So a vendor that is down retries and poisons alone and never
     makes another vendor receive an event twice.
   - Adding a vendor is a new adapter, its settings, and one line in `internal/app/analytics.go`.
     It receives events from the moment it is configured; it gets no history.
   - Adapters are tested against one shared contract suite (`internal/repo/analytics/sinktest`),
     as the event stores share `eventstoretest`.
7. **No history replay.** Each `capture-*` handler implements `repo.StartsAtHead`, so the
   projection runner starts its checkpoint at the store's head the first time it runs, so enabling analytics never sends past events. `*Imported` events are never
   mapped. The handlers cannot be rebuilt (they are event handlers).
8. **The distinct ID is the user ID** on both sides (server: the event's `Metadata.UserID`), so server and web events join. The web client
   calls `identify(userID)` after sign-in and `reset()` on sign-out.
9. **Optional by presence, per vendor:** each adapter has its own settings (`POSTHOG_API_KEY`,
   `POSTHOG_HOST`); a vendor is on when its settings are present, and there is no on/off flag. With
   no vendor, the `capture-*` handlers and the `analytics-*` topics are not registered. The web
   client has one vendor: `VITE_POSTHOG_KEY` / `VITE_POSTHOG_HOST`; without them `posthog-js` is
   not loaded. `web/src/lib/analytics.ts` is the only file that imports it (`make web-check`
   enforces it), so switching the web vendor is one file. Session replay is PostHog's feature,
   not part of the facade.
10. **No consent banner and no GDPR delete** in this module.

## Project structure

`+` a new file, `~` a change.

| File | What |
| --- | --- |
| **Entity** | |
| `+ internal/entity/analytics/analytics.go` | `Event`, `Properties`, `Events()`, `EventID()`, `Encode`/`Decode`, `ErrUnknownEvent`, `ErrUnknownSink`, `QueuePrefix` |
| `+ internal/entity/analytics/user.go` | `UserRegistered`, `SignUpMethod` |
| `+ internal/entity/analytics/task.go` | `TaskCreated`, `TaskStatusChanged` |
| `+ internal/entity/analytics/exchange.go` | `CurrencyConverted` |
| `+ internal/entity/analytics/analytics_test.go` | unique names, round trip, no free string |
| **Ports** | |
| `+ internal/repo/analytics.go` | `Analytics`, `AnalyticsSink` (+ `go:generate`) |
| `+ internal/repo/repomock/analytics.go` | `make mock` |
| `~ internal/repo/contracts.go` | `StartsAtHead` |
| **Adapters** | |
| `+ internal/repo/analytics/analytics.go` + `_test.go` | the fan-out publisher |
| `+ internal/repo/analytics/posthog/posthog.go` + `_test.go` | the PostHog sink, the only `posthog-go` import |
| `+ internal/repo/analytics/sinktest/sinktest.go` | the suite every sink passes |
| **Projection runner** | |
| `~ internal/repo/persistent/projection/runner.go` + `_test.go` | a `StartsAtHead` subscriber with no checkpoint starts at the head |
| **Event handlers** | |
| `+ internal/usecase/user/event/capture_user.go` + `_test.go` | `UserRegistered`, `UserRegisteredViaOIDC` |
| `+ internal/usecase/task/event/capture_task.go` + `_test.go` | `TaskCreated`, `TaskTransitioned` |
| `+ internal/usecase/exchange/event/capture_exchange.go` + `_test.go` | `CurrencyConverted` |
| **Delivery** | |
| `+ internal/usecase/analytics/register.go` | `Deps{Sinks}`, `Register` |
| `+ internal/usecase/analytics/command/deliver_analytics.go` + `_test.go` | picks the sink by name |
| `+ internal/controller/worker/v1/analytics.go` + `_test.go` | one route per sink |
| **Wiring** | |
| `+ internal/app/analytics.go` + `_test.go` | `analyticsModule`, `analyticsSinks` |
| `~ internal/app/{user,task,exchange}.go` | `eventHandlers`: the `capture-*` handler when sinks exist |
| `~ internal/app/modules.go` | `deps`: `analytics`, `sinks` |
| `~ internal/app/worker.go` | builds the sinks and the publisher |
| **Config** | |
| `~ config/config.go` + `_test.go` | `Analytics{PostHog}` |
| `~ .env.example`, `docker-compose.yml` (worker) | `POSTHOG_API_KEY`, `POSTHOG_HOST` |
| `~ go.mod`, `go.sum` | `github.com/posthog/posthog-go` |
| **Web** | |
| `+ web/src/lib/analytics.ts` | the only `posthog-js` import: `init`, `capture`, `identify`, `reset`, `pageview` |
| `+ web/src/lib/analytics-events.ts` | `user_logged_in`, `task_attributes_updated`, `page_viewed` |
| `+ web/src/lib/analytics.test.ts` | |
| `~ web/src/main.tsx` | init when `VITE_POSTHOG_KEY` is set |
| `~ web/src/router.tsx` | page view with the route template |
| `~ web/src/auth/` | `identify` after sign-in, `reset` on sign-out, `user_logged_in` |
| `~ web/src/tasks/api.ts` | `task_attributes_updated` on the mutation's success |
| `~ web/vite.config.ts` | lint: `posthog-js` only in `src/lib/analytics.ts` |
| `~ web/.env.example`, `web/package.json`, `web/bun.lock` | `VITE_POSTHOG_*`, `posthog-js` |
| **Docs** | |
| `+ docs/analytics.md` | the catalogue, the strategy, adding a sink |
| `~ docs/configuration.md`, `docs/worker.md`, `docs/cqrs.md`, `docs/web.md`, `docs/architecture.md` | settings, topics, `capture-*` and `StartsAtHead`, the facade, the packages |

## Code shape

Only the parts that carry the design; the rest follows the neighbouring code.

`internal/entity/analytics/analytics.go`

```go
type Properties interface{ Name() string }

type Event struct {
	ID, DistinctID string
	At             time.Time
	Properties     Properties
}

func Events() []Properties { return slices.Concat(userEvents(), taskEvents(), exchangeEvents()) }

func EventID(e *entity.RecordedEvent) string {
	return fmt.Sprintf("%s-%d", e.StreamID, e.StreamVersion)
}
```

`internal/entity/analytics/task.go` (`user.go` and `exchange.go` have the same shape)

```go
type TaskStatusChanged struct {
	From entity.TaskStatus `json:"from"`
	To   entity.TaskStatus `json:"to"`
}

func (TaskStatusChanged) Name() string { return "task_status_changed" }

func taskEvents() []Properties { return []Properties{TaskCreated{}, TaskStatusChanged{}} }
```

`internal/repo/analytics.go`

```go
Analytics interface {
	Capture(ctx context.Context, e *analytics.Event) error
}

AnalyticsSink interface {
	Name() string
	Send(ctx context.Context, e *analytics.Event) error
}
```

`internal/usecase/task/event/capture_task.go`

```go
switch ev := e.Event.(type) {
case entity.TaskCreated:
	props = analytics.TaskCreated{}
case entity.TaskTransitioned:
	props = analytics.TaskStatusChanged{From: ev.From, To: ev.To}
default:
	return fmt.Errorf("CaptureTask - %s: %w", e.Event.EventType(), entity.ErrUnhandledEvent)
}

err := h.Analytics.Capture(ctx, &analytics.Event{
	ID:         analytics.EventID(e),
	DistinctID: e.Metadata.UserID,
	At:         e.RecordedAt,
	Properties: props,
})
```

`internal/repo/analytics/analytics.go`

```go
for _, sink := range p.sinks {
	if err = p.pub.Publish(ctx, analytics.QueuePrefix+sink, sink+"-"+e.ID, body); err != nil {
		return fmt.Errorf("Analytics - Capture - p.pub.Publish %s: %w", sink, err)
	}
}
```

`internal/usecase/analytics/command/deliver_analytics.go`

```go
sink, ok := h.Sinks[cmd.Sink]
if !ok {
	return entity.Written{}, fmt.Errorf("DeliverAnalytics - %s: %w", cmd.Sink, analytics.ErrUnknownSink)
}

if err := sink.Send(ctx, cmd.Event); err != nil {
	return entity.Written{}, fmt.Errorf("DeliverAnalytics - sink.Send %s: %w", cmd.Sink, err)
}
```

`internal/controller/worker/v1/analytics.go`

```go
for _, sink := range sinks {
	c := &AnalyticsController{cmds: cmds, sink: sink, l: l}
	r.Handle(analytics.QueuePrefix+sink, c.Handle)
}
// a body that does not decode, or ErrUnknownSink → broker.Permanent; anything else is retried
```

`internal/repo/analytics/posthog/posthog.go`

```go
ph.Capture{
	Uuid:       uuid.NewSHA1(uuid.NameSpaceOID, []byte(e.ID)).String(),
	DistinctId: e.DistinctID,
	Event:      e.Properties.Name(),
	Timestamp:  e.At,
	Properties: props,
}
// sent one at a time and awaited; a 4xx → broker.Permanent
```

`internal/repo/persistent/projection/runner.go`

```go
if _, ok := s.(repo.StartsAtHead); ok && !found {
	checkpoint = head
}
```

`internal/app/analytics.go`

```go
func analyticsSinks(cfg config.Analytics) (map[string]repo.AnalyticsSink, error) {
	sinks := map[string]repo.AnalyticsSink{}

	if cfg.PostHog.APIKey != "" {
		s, err := posthog.New(cfg.PostHog.APIKey, cfg.PostHog.Host)
		if err != nil {
			return nil, fmt.Errorf("analytics - posthog.New: %w", err)
		}

		sinks[s.Name()] = s
	}

	return sinks, nil
}
```

`internal/app/task.go`

```go
eventHandlers: func(d *deps) []repo.Subscriber {
	if d.analytics == nil {
		return nil
	}

	return []repo.Subscriber{taskevent.CaptureTask{Analytics: d.analytics}}
},
```

`web/src/lib/analytics.ts`

```ts
posthog.init(key, {
  api_host: host,
  session_recording: { maskAllInputs: true, maskTextSelector: "[data-task-text]" },
});

export const pageview = (path: string) => posthog.capture("$pageview", { path });
```

Package names: `entity/analytics`, `repo/analytics` and `usecase/analytics` are all `analytics`; a
file importing two keeps `analytics` for the entity and aliases the others (`analyticsrepo`,
`analyticscmd`), as `internal/app` aliases the repositories today.

## Testing strategy

- **Entity:** the catalogue test (every mapped event listed, no `*Imported` mapped); no event type
  has a string field outside an allowed list.
- **Event handlers:** table-driven against the mock `repo.Analytics`: the right properties, ID and
  distinct ID for each domain event.
- **`DeliverAnalytics`:** the named sink is called; an unknown one is `ErrUnknownSink`.
- **Projection runner:** a new `StartsAtHead` subscriber starts at the head; an existing one keeps its checkpoint; others start at 0.
- **Publisher:** two fake sinks: one message per sink, IDs `<sink>-<event id>`; none with no sinks.
- **Worker handler:** a failing sink is retried alone; the other sink's topic is untouched.
- **Sinks:** `sinktest` against a fake PostHog server: one request per event, the dedup key from
  `ID`, the name mapping, a 4xx is permanent.
- **Web:** `analytics.test.ts`: no key, no load; page view with the route template on navigation;
  `user_logged_in` and `task_attributes_updated` after success only; masking options set (the
  replay itself is not tested).
- **Integration:** none (decided 2026-11-08). No fake PostHog in the stack.

## Boundaries

- **Always:** vendor SDKs only in their adapter (web: in `analytics.ts`); user ID only; masked replay; the catalogue updated with every new event.
- **Ask first:** a new vendor adapter; a new event; any person property; surveys, experiments, group analytics.
- **Never:** PII in a property; sending from the app process; analytics stored by us; an outbox
  (a future feature, not this module).

## Success criteria

1. With the keys set, creating and finishing a task produces the funnel's three events for one
   person in PostHog, and page views for the same person.
2. Enabling the keys on a database with history sends none of it.
3. With no vendor configured, no `capture-*` checkpoints, no analytics traffic, nothing else changes.
4. With two sinks configured and one down, the other receives every event once.
5. Masking is on for every input and task text.
6. Checks green; the catalogue test fails when an event is mapped but not listed.
