# Releases

Nobody picks a version or pushes a tag. Every commit to master whose CI passes is released when
its commits call for it, and the version is named from those commits.

## How a version is named

[svu](https://github.com/caarlos0/svu) (`go tool svu`, pinned in `go.mod`) names the version from the
commits since the last `v*` tag; `make release-version` runs it. Commit subjects follow
[Conventional Commits](https://www.conventionalcommits.org/): `type(scope)!: text`. Merge commits
change nothing, and the commits of a merged PR count.

| Commits since the last tag | Below 1.0 | From 1.0 |
| --- | --- | --- |
| a breaking change: `feat!:`, `fix(api)!:`, or a `BREAKING CHANGE:` line in the body | minor (`v0.4.2` → `v0.5.0`) | major (`v1.4.2` → `v2.0.0`) |
| a `feat` | minor | minor |
| a `fix` | patch (`v0.4.2` → `v0.4.3`) | patch |
| only `perf`, `docs`, `ci`, `chore`, `test`, `build`, `refactor`, or no type | no release | no release |

- The highest bump wins: a `fix` and a `feat` together make a minor.
- `perf` alone releases nothing; it is listed in the notes of the next release.
- Dependabot's `chore(deps):` merges release nothing.

`make release-version` prints what master would be released as, or nothing, with the reason on
stderr. It also prints nothing when a `v*` tag exists that HEAD does not contain: a later merge
was released first, and its release includes this commit.

## How the notes are written

[GoReleaser](https://goreleaser.com/) writes the notes and publishes the release, configured in
`.goreleaser.yaml`. It builds no binaries. The notes list the commits since the last tag under
Breaking changes, Features, Fixes and Performance, each commit once, under the first heading that
matches its subject. Other types are left out. GoReleaser only reads subjects, so a commit whose
only breaking marker is a `BREAKING CHANGE:` footer is listed under its type, or not at all.

## What happens on a merge

1. CI runs on the master push.
2. When CI succeeds, `.github/workflows/release.yml` checks out the commit CI checked and runs
   `make release-version`. Nothing printed means no release, and the run ends there.
3. Otherwise it tags that commit in its checkout and runs GoReleaser. The tag is never pushed:
   GoReleaser creates the GitHub release with the notes, and GitHub creates the tag on that commit
   (`release.target_commitish`).

Releases run one at a time, in order. To see what master would be released as without publishing
anything, run the workflow by hand (`gh workflow run release.yml`): GoReleaser runs with
`--skip=publish`, and the run's summary shows the version and notes.

## Tags are never moved or deleted

The Go module proxy and checksum database keep every version they have seen, forever. If a tag
moves, the module at that version no longer matches its checksum for anyone who fetched it
before. If something is wrong with a release, fix it and let the next merge release a new version.
A ruleset on `refs/tags/v*` blocks updating and deleting tags.
