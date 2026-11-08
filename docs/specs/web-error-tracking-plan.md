# Implementation plan: web error tracking

Spec: [web-error-tracking.md](web-error-tracking.md) · Tasks: [web-error-tracking-todo.md](web-error-tracking-todo.md) · Map: [web.md](web.md)

On `feat/web-error-tracking` (off `feat/web-errors`), two commits: the client and its tests; the docs
and config. Built without a GlitchTip: a local envelope-capture server stood in for it in the
browser check. The release step (hidden source maps, upload, removing them from `dist/`) is **not
built**: it needs GlitchTip's release API, which the backend rollout has not added, and generating
maps into `dist/` without removing them would serve them.
