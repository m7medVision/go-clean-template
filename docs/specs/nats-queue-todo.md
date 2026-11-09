# Tasks: nats-queue

- [x] `pkg/broker` and its chain test; live check against `nats:2.15.2-alpine`.
- [x] `notify.Publisher`, mail controller on a router, `broker.Permanent`.
- [x] Config, `RunWorker`, modules wiring and their tests.
- [x] Remove `pkg/worker`, the queue repo and SQL; drop migration; `make sqlc`.
- [x] Compose, Makefile, `.env.example`, CI.
- [x] Integration tests; docs; the poison TODO in `pkg/broker`.
- [x] CI green on PR #56.
