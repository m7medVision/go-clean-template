# Release system: automatic tags, scaffold from a tag

Status: idea, not built.

## Problem statement

How might we turn every meaningful merge to master into a version that a public user can generate a
project from with one command, without anyone naming the version by hand?

## Recommended direction

**Version names come from commits.** A merge counts as a change when it carries `feat`, `fix` or
`perf` commits.

- In 0.x, `feat` and breaking changes bump the minor, and `fix`/`perf` bump the patch. The first
  tag is `v0.1.0`.
- A merge with only `ci`, `docs`, `chore`, `test` or deps commits releases nothing.

`svu next` computes the version and `git-cliff` writes the notes. There's no Node release tool and
no release PR.

**A version only exists once it has passed the matrix.** The CI `scaffold` job checks all four
presets on the master push and uploads each generated project. `release.yml` then runs on
`workflow_run` (CI succeeded on master, serialized by a concurrency group). It:

1. tags the run's `head_sha`;
2. packs those exact projects as `api.tar.gz`, `api-grpc.tar.gz`, `fullstack.tar.gz` and
   `everything.tar.gz`;
3. publishes a GitHub Release with the notes and the four tarballs.

**The CLI is pinned to its tag.** `go run github.com/m7medvision/go-clean-template/cmd/create@v0.4.0`
(or `@latest`) reads its own version and downloads that tag's source archive. It then runs the same
form and generation it runs today, so the template and the CLI can't drift.

- Inside a template checkout (`make scaffold`), it keeps using the local files, as it does now.
- `--ref vX.Y.Z` overrides the version.

## Key assumptions to validate

- [ ] `go run …/cmd/create@<tag>` works from outside the repo: check with a throwaway
  `v0.0.1-rc.1` pre-release.
- [ ] The tag archive's files equal `git ls-files`: run a `diff -r` of the two generated projects as
  a test.
- [ ] `debug.ReadBuildInfo` returns the tag under `go run @v`, and `(devel)` locally: unit test plus
  the rc check.
- [ ] CI artifacts from the scaffold job are small enough to keep per run (`node_modules` and `.git`
  left out of the tarball).

## MVP scope

**In:**

- Delete the 61 upstream tags locally (`origin` has none).
- `release.yml`.
- `cliff.toml` with the 0.x bump rules.
- The CI scaffold job uploads its generated projects.
- The CLI fetches a tag, with a non-git file walk in `ListFiles` and `--ref`.
- `docs/scaffold.md` and `docs/releasing.md` cover install, tags and tarballs.
- Protected `v*` tags.
- The first release, `v0.1.0`.

**Out:** everything in the list below.

## Not doing (and why)

- **Release PRs (release-please):** releases are cut on every merge.
- **Binaries or GoReleaser:** `go run …@vX` gives the same result with nothing to build or sign.
- **Docker images:** a template's users build their own service, not this demo.
- **Notes grouped by feature:** a nice follow-up once commit scopes are consistent.
- **`--upgrade-from`:** the real long-term prize, but a project of its own.
- **Releasing on Dependabot bumps:** it would flood public users with versions that change nothing
  they use.
- **Carrying on upstream's v1.x numbers:** they belong to evrone's module path, and v2+ would force
  a `/v2` module path.

## Open questions

- Should the tarballs keep the template's module path, with docs saying to rename it, or should we
  drop them in favour of telling users to run the CLI?
- Will the tag ruleset allow the release workflow's `GITHUB_TOKEN` to create `v*` tags while still
  blocking deletes and moves for humans?
