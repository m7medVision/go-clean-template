#!/usr/bin/env bash
# Generator end to end: proves that what make new-domain writes builds, lints and passes its tests.
# It runs against the working tree as it is on disk, not HEAD, so template work is tested before it
# is committed: a throwaway git worktree gets HEAD, then the tracked changes and the untracked
# files under tools/gen, committed there so the generator's rule R3 (no uncommitted changes) holds.
# The generator and the gates run in that worktree, which is removed on exit, pass or fail. The
# working checkout is never touched; nothing starts the app or touches a database.
#
# It needs what the regenerators need: protoc, Docker (make sqlc's throwaway dev database), atlas,
# the vp CLI and the tools make bin-deps installs.
#
# Usage: scripts/gen-e2e.sh. Run it through `make gen-e2e`.
set -euo pipefail

repo=$(git -C "$(dirname "$0")/.." rev-parse --show-toplevel)
tmp=$(mktemp -d)
wt="$tmp/worktree"

cleanup() {
	status=$?
	cd /
	git -C "$repo" worktree remove --force "$wt" >/dev/null 2>&1 || true
	git -C "$repo" worktree prune || true
	rm -rf "$tmp"
	if [ "$status" -eq 0 ]; then
		echo "gen-e2e: passed"
	else
		echo "gen-e2e: failed (exit $status)" >&2
	fi
	exit "$status"
}
trap cleanup EXIT

echo "gen-e2e: worktree $wt"
git -C "$repo" worktree add --quiet --detach "$wt" HEAD

# The tracked changes, staged or not, and the untracked files under tools/gen that git does not
# ignore. Other untracked files stay behind.
git -C "$repo" diff --binary HEAD >"$tmp/tracked.patch"
if [ -s "$tmp/tracked.patch" ]; then
	git -C "$wt" apply --binary "$tmp/tracked.patch"
fi
git -C "$repo" ls-files -z --others --exclude-standard -- tools/gen |
	tar -C "$repo" --null -T - -cf - | tar -C "$wt" -xf -

git -C "$wt" add --all
git -C "$wt" -c user.name=gen-e2e -c user.email=gen-e2e@localhost -c commit.gpgsign=false \
	commit --quiet --allow-empty --no-verify -m "gen-e2e: the working tree"

cd "$wt"
make web-deps
make new-domain name=widget
echo "gen-e2e: what generating changed:"
git status --short
# make openapi ran among the regenerators: the REST operations must be in the exported document,
# which make web-api then typed for the web client.
for op in create-widget list-widgets get-widget update-widget delete-widget; do
	grep -q "\"operationId\": \"$op\"" docs/openapi/v1.json ||
		{ echo "gen-e2e: docs/openapi/v1.json has no $op operation" >&2; exit 1; }
done
# make proto-v1 ran too: the gRPC controller builds on the Go code it generates from widget.proto.
for f in docs/proto/v1/widget.pb.go docs/proto/v1/widget_grpc.pb.go; do
	[ -f "$f" ] || { echo "gen-e2e: make proto-v1 wrote no $f" >&2; exit 1; }
done
# So did make graphql: the resolvers build on what gqlgen generates from widget.graphqls, in the
# file its follow-schema layout names after it, and on the WidgetList model it generates.
f=internal/controller/graphql/generated/widget.generated.go
[ -f "$f" ] || { echo "gen-e2e: make graphql wrote no $f" >&2; exit 1; }
grep -q '^type WidgetList struct' internal/controller/graphql/model/models_gen.go ||
	{ echo "gen-e2e: make graphql generated no WidgetList model" >&2; exit 1; }
make linter-golangci test
# The integration test needs the running stack, so here it is only built and vetted; make
# compose-up-integration-test runs it.
f=integration-test/widget_test.go
[ -f "$f" ] || { echo "gen-e2e: make new-domain wrote no $f" >&2; exit 1; }
echo "gen-e2e: go vet ./integration-test/..."
go vet ./integration-test/...
