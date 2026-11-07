#!/usr/bin/env bash
# Generator end to end: proves that what make new-domain and make new-usecase write builds, lints
# and passes its tests.
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

# commit commits everything in the worktree under a throwaway identity, so the next run of the
# generator finds the files it edits clean (rule R3).
commit() {
	git -C "$wt" add --all
	git -C "$wt" -c user.name=gen-e2e -c user.email=gen-e2e@localhost -c commit.gpgsign=false \
		commit --quiet --allow-empty --no-verify -m "gen-e2e: $1"
}

# operations fails unless docs/openapi/v1.json, which make openapi exported among the regenerators,
# has each operation ID given.
operations() {
	for op in "$@"; do
		grep -q "\"operationId\": \"$op\"" docs/openapi/v1.json ||
			{ echo "gen-e2e: docs/openapi/v1.json has no $op operation" >&2; exit 1; }
	done
}

# has fails unless the file or folder $1 holds the text $2, which a regenerator wrote: $3 says
# which.
has() {
	grep -rqF -- "$2" "$1" || { echo "gen-e2e: $3 wrote no $2 in $1" >&2; exit 1; }
}

commit "the working tree"

cd "$wt"
make web-deps
make new-domain name=widget
echo "gen-e2e: what generating changed:"
git status --short
# make openapi ran among the regenerators: the REST operations must be in the exported document,
# which make web-api then typed for the web client.
operations create-widget list-widgets get-widget update-widget delete-widget
# make proto-v1 ran too: the gRPC controller builds on the Go code it generates from widget.proto.
for f in docs/proto/v1/widget.pb.go docs/proto/v1/widget_grpc.pb.go; do
	[ -f "$f" ] || { echo "gen-e2e: make proto-v1 wrote no $f" >&2; exit 1; }
done
# So did make graphql: the resolvers build on what gqlgen generates from widget_types.graphqls, in
# the file its follow-schema layout names after it, and on the WidgetList model it generates.
f=internal/controller/graphql/generated/widget_types.generated.go
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
commit "new-domain widget"

# A command and a query on the generated domain, then a command on task, which is hand-written:
# each gets its REST operation, its gRPC method and its GraphQL field, and the regenerators that
# make new-usecase runs must have made each of them into code.
make new-usecase domain=widget name=archive-widget kind=command
git status --short
operations archive-widget
has docs/proto/v1/widget_grpc.pb.go 'WidgetService_ArchiveWidget_FullMethodName' 'make proto-v1'
# gqlgen puts a root type's fields in the file of the schema file that defines it, schema.graphqls,
# which every field's schema file extends.
has internal/controller/graphql/generated '_Mutation_archiveWidget(' 'make graphql'
make linter-golangci test
commit "new-usecase widget archive-widget"

make new-usecase domain=widget name=count-widgets kind=query
git status --short
operations count-widgets
has docs/proto/v1/widget_grpc.pb.go 'WidgetService_CountWidgets_FullMethodName' 'make proto-v1'
has internal/controller/graphql/model/models_gen.go 'type CountWidgetsResult struct' 'make graphql'
make linter-golangci test
commit "new-usecase widget count-widgets"

make new-usecase domain=task name=archive-task kind=command
git status --short
operations archive-task
has docs/proto/v1/task_grpc.pb.go 'TaskService_ArchiveTask_FullMethodName' 'make proto-v1'
has internal/controller/graphql/generated '_Mutation_archiveTask(' 'make graphql'
make linter-golangci test
