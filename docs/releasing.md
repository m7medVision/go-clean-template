# Releases

Nobody picks a version or pushes a tag. Every commit to master whose CI passes is released when
its commits call for it, and the version is named from those commits.

## How a version is named

`make release-version` reads the commits since the last `v*` tag and applies these rules.
Commit subjects follow [Conventional Commits](https://www.conventionalcommits.org/):
`type(scope)!: text`. Merge commits are skipped, and the commits of a merged PR count.

| Commits since the last tag | Below 1.0 | From 1.0 |
| --- | --- | --- |
| a breaking change: `feat!:`, `fix(api)!:`, or a `BREAKING CHANGE:` line in the body | minor (`v0.4.2` → `v0.5.0`) | major (`v1.4.2` → `v2.0.0`) |
| a `feat` | minor | minor |
| a `fix` or `perf` | patch (`v0.4.2` → `v0.4.3`) | patch |
| only `docs`, `ci`, `chore`, `test`, `build`, `refactor`, or no type | no release | no release |

- The highest bump wins: a `fix` and a `feat` together make a minor.
- With no tag yet, the first release is `v0.1.0`.
- Dependabot's `chore(deps):` merges release nothing.

`make release-notes` prints the notes for the same commits. They are grouped under Breaking
changes, Features, Fixes and Performance, with the scope in bold and the short hash.

Both targets run locally: `make release-version` prints what master would be released as, or
nothing, with the reason on stderr.

## What happens on a merge

1. CI runs on the master push. Its `scaffold` job checks every preset with `make pre-commit`, then
   keeps each checked project as an artifact: `<preset>.tar.gz`, holding only the files the
   project's `.gitignore` allows.
2. When CI succeeds, `.github/workflows/release.yml` checks out the commit CI checked and runs
   `make release-version`.
   - Nothing printed means no release, and the run ends there.
   - It also prints nothing when a newer tag already exists that this commit is not part of: a
     later merge was released first, and its release includes this commit.
3. Otherwise it creates the GitHub release with `gh release create`: the tag on that commit, the
   notes, and the preset tarballs.

Releases run one at a time, in order. To see what master would be released as without publishing
anything, run the workflow by hand (`gh workflow run release.yml`): it writes the version and notes
to the run's summary.

## Tags are never moved or deleted

The Go module proxy and checksum database keep every version they have seen, forever. If a tag
moves, `go run …/cmd/create@vX` breaks for anyone who fetched it before. If something is wrong
with a release, fix it and let the next merge release a new version. A ruleset on `refs/tags/v*`
blocks updating and deleting tags.

## What a release gives people

- `go run github.com/m7medvision/go-clean-template/cmd/create@v0.4.0` (or `@latest`) runs the
  scaffold CLI at that version. It downloads the template at the same version and generates from
  it, so the CLI and the template always match. See [scaffold.md](scaffold.md).
  For a few minutes after a release, the Go module proxy may not list the new tag yet, so
  `@latest` still resolves to the version before it (or, before the first release, to a
  pseudo-version of master). Name the version (`@v0.4.0`) to get it at once.
- `api.tar.gz`, `api-grpc.tar.gz`, `fullstack.tar.gz` and `everything.tar.gz` on the release page
  are those presets, generated and checked. They keep the template's module path; use the CLI
  with `--module` for your own.
