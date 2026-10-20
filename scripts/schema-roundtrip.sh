#!/usr/bin/env bash
# Schema round trip: proves the HCL in the working tree can be applied on top of the schema at a base
# git ref (up), rolled back to it (down), and applied again (up), and that each apply leaves nothing
# to sync. The base schema comes out of git with `git archive`, so no checkout is touched.
#
# The apply is declarative and drops whatever the target HCL does not declare, so it must never see a
# real database. Locally it starts a throwaway Postgres container that exists only while the script
# runs. CI passes ROUNDTRIP_PG_URL, the Postgres that bmizerany/setup-postgres starts on the runner;
# the script refuses a database that already holds anything.
#
# Usage: scripts/schema-roundtrip.sh [base-ref]   (default: origin/master). Run it through
# `make schema-roundtrip`.
set -euo pipefail

BASE_REF=${1:-origin/master}
SCHEMA_DIR=internal/app/schema
PG_IMAGE=docker.io/library/postgres:18.6-alpine

cd "$(dirname "$0")/.."

url=${ROUNDTRIP_PG_URL:-}

tools="git atlas"
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

tmp=$(mktemp -d)
container=""

cleanup() {
	if [ -n "$container" ]; then
		docker rm -f "$container" >/dev/null 2>&1 || true
	fi
	rm -rf "$tmp"
}
trap cleanup EXIT

if ! git cat-file -e "$BASE_REF:$SCHEMA_DIR" 2>/dev/null; then
	echo "schema-roundtrip: $BASE_REF has no $SCHEMA_DIR; nothing to round-trip against"
	exit 0
fi

git archive "$BASE_REF" "$SCHEMA_DIR" | tar -x -C "$tmp"
base="file://$tmp/$SCHEMA_DIR"
head="file://$SCHEMA_DIR"

if diff -r "$tmp/$SCHEMA_DIR" "$SCHEMA_DIR" >/dev/null; then
	echo "schema-roundtrip: schema unchanged since $BASE_REF; checking it applies cleanly"
fi

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

# An empty database inspects as the public schema alone; anything more means it is not a throwaway.
current=$(atlas schema inspect -u "$url" --format '{{ sql . }}')
if grep -qvE '^(--|CREATE SCHEMA |COMMENT ON SCHEMA |$)' <<<"$current"; then
	echo "schema-roundtrip: the database is not empty; it needs a throwaway one" >&2
	exit 1
fi

# apply STEP TARGET: apply TARGET, then fail unless a second plan against it is empty.
apply() {
	echo "== $1"
	atlas schema apply -u "$url" --to "$2" --auto-approve
	plan=$(atlas schema apply -u "$url" --to "$2" --dry-run)
	if ! grep -q "Schema is synced, no changes to be made" <<<"$plan"; then
		echo "schema-roundtrip: $1 left drift behind:" >&2
		echo "$plan" >&2
		exit 1
	fi
}

apply "base ($BASE_REF)" "$base"
apply "up (working tree)" "$head"
apply "down ($BASE_REF)" "$base"
apply "up again (working tree)" "$head"

echo "schema-roundtrip: up and down against $BASE_REF both apply cleanly"
