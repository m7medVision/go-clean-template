#!/usr/bin/env bash
# Migration round trip: every migration in the working tree goes up, down to zero and up again on a
# throwaway Postgres, and no migration that exists at the base ref was changed or removed.
#
# Down to zero drops every table, so it must never see a real database. Locally it starts a
# throwaway Postgres container that exists only while the script runs. CI passes ROUNDTRIP_PG_URL,
# the Postgres that bmizerany/setup-postgres starts on the runner; the script refuses a database
# that already has migrations applied.
#
# Usage: scripts/schema-roundtrip.sh [base-ref]   (default: origin/master). Run it through
# `make schema-roundtrip`.
set -euo pipefail

BASE_REF=${1:-origin/master}
MIGRATIONS_DIR=internal/app/migrations
PG_IMAGE=docker.io/library/postgres:18.6-alpine

cd "$(dirname "$0")/.."

url=${ROUNDTRIP_PG_URL:-}

tools="git go"
if [ -z "$url" ]; then
	tools="$tools docker"
fi

for cmd in $tools; do
	if ! command -v "$cmd" >/dev/null 2>&1; then
		echo "schema-roundtrip: $cmd is not on PATH" >&2
		exit 1
	fi
done

if ! git rev-parse --verify --quiet "$BASE_REF^{commit}" >/dev/null; then
	echo "schema-roundtrip: unknown git ref $BASE_REF (fetch it first)" >&2
	exit 1
fi

if git cat-file -e "$BASE_REF:$MIGRATIONS_DIR" 2>/dev/null; then
	changed=$(git diff --name-only --diff-filter=MDR "$BASE_REF" -- "$MIGRATIONS_DIR" | grep '\.sql$' || true)
	if [ -n "$changed" ]; then
		echo "schema-roundtrip: migrations merged on $BASE_REF were edited or removed; add a new one instead:" >&2
		echo "$changed" >&2
		exit 1
	fi
fi

tmp=$(mktemp -d)
container=""

cleanup() {
	if [ -n "$container" ]; then
		docker rm -f "$container" >/dev/null 2>&1 || true
	fi
	rm -rf "$tmp"
}
trap cleanup EXIT

if [ -z "$url" ]; then
	container=$(docker run -d --rm -e POSTGRES_PASSWORD=roundtrip -p 127.0.0.1::5432 "$PG_IMAGE")
	port=$(docker port "$container" 5432/tcp | head -n1 | sed 's/.*://')
	url="postgres://postgres:roundtrip@127.0.0.1:$port/postgres?sslmode=disable"

	for _ in $(seq 30); do
		if docker exec "$container" pg_isready -U postgres -h 127.0.0.1 >/dev/null 2>&1; then
			break
		fi
		sleep 1
	done
	docker exec "$container" pg_isready -U postgres -h 127.0.0.1 >/dev/null
fi

go build -o "$tmp/migrate" ./cmd/migrate
migrate() { "$tmp/migrate" -url "$url" -dir "$MIGRATIONS_DIR" "$@"; }

migrate require-empty

echo "== up"
migrate up
echo "== down to zero"
migrate down-to 0
echo "== up again"
migrate up
migrate status

echo "schema-roundtrip: every migration goes up, down and up again"
