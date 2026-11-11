![Go Clean Template](docs/img/logo.svg)

Go Clean Template is a starting point for Go services built on Clean Architecture, with CQRS and
event sourcing at its core. Every change is a command that records events, every read is a query
against a projection, and REST, gRPC and GraphQL all send the same commands and queries. It ships
with users, passkeys, SSO, tasks and currency exchange as working examples, a worker that runs the
projections and a queue, a React client typed from the API, and a generator that writes a new
domain or use case across every transport.

To try it, you need Docker, Go, `tmux`, `curl` and the [Vite+](https://viteplus.dev/) `vp` CLI.
Run `cp .env.example .env`, `make web-deps` and `make dev`, then open <http://localhost:3000> and
sign in with SSO as `alice` / `alice`.

The [documentation](https://m7medvision.github.io/go-clean-template/) covers the architecture, a
guide for every kind of change, and a reference for every setting and endpoint. Released under the
[MIT License](LICENSE); originally forked from
[evrone/go-clean-template](https://github.com/evrone/go-clean-template).
