#!/usr/bin/env bash
# Dev mode in one tmux window: the compose services in DEV_SERVICES in docker, and the Go binaries on
# the host under air.
# The Vite+ web dev server runs beside them. # scaffold:web
# Run it through `make dev`, which exports .env (or .env.example) and DEV_SERVICES; stop with
# `make dev-down`.
set -euo pipefail

cd "$(dirname "$0")/.."

# One session per checkout, named after its directory, so a generated project runs beside this one.
SESSION=$(basename "$PWD")

required=(tmux docker go curl)
required+=(vp) # scaffold:web
for cmd in "${required[@]}"; do
	if ! command -v "$cmd" >/dev/null 2>&1; then
		echo "make dev: $cmd is not on PATH" >&2
		exit 1
	fi
done

if [ -z "${PG_URL:-}" ] || [ -z "${DEV_SERVICES:-}" ]; then
	echo "make dev: PG_URL or DEV_SERVICES is not set; run this through make dev" >&2
	exit 1
fi

read -ra services <<<"$DEV_SERVICES"

attach() {
	if [ -n "${TMUX:-}" ]; then
		exec tmux switch-client -t "=$SESSION"
	fi

	exec tmux attach-session -t "=$SESSION"
}

if tmux has-session -t "=$SESSION" 2>/dev/null; then
	attach
fi

export TRACING_ENABLED=false
# scaffold:web:begin
# The web client is the Vite dev server here, not Caddy: the links the app builds point at it.
export APP_BASE_URL=http://localhost:${WEB_PORT:-3000}
# scaffold:web:end

# A pane gets the tmux server's environment, not this shell's, when a server is already running, so
# hand every setting over explicitly: each name in the env file make loaded, with the value make
# exported (command-line overrides such as PG_PORT=5433 included), plus PATH.
env_file=.env
[ -f "$env_file" ] || env_file=.env.example

env_args=(-e "PATH=$PATH")
while IFS= read -r name; do
	env_args+=(-e "$name=${!name-}")
done < <(sed -nE 's/^([A-Za-z_][A-Za-z0-9_]*)=.*/\1/p' "$env_file")

docker compose up -d "${services[@]}"

pane() {
	local title=$1 cmd=$2 id

	if tmux has-session -t "=$SESSION" 2>/dev/null; then
		id=$(tmux split-window -t "=$SESSION:dev" -c "$PWD" -P -F '#{pane_id}' "${env_args[@]}" "$cmd")
	else
		id=$(tmux new-session -d -s "$SESSION" -n dev -c "$PWD" -P -F '#{pane_id}' \
			-x "$(tput cols 2>/dev/null || echo 200)" -y "$(tput lines 2>/dev/null || echo 50)" \
			"${env_args[@]}" "$cmd")
		# Keep a pane open when its process exits, so a crash or a build error stays readable.
		tmux set-option -w -t "$id" remain-on-exit on
		tmux set-option -w -t "$id" pane-border-status top
	fi

	tmux select-pane -t "$id" -T "$title"
	tmux select-layout -t "$id" tiled >/dev/null
}

pane app 'go tool air -c .air.app.toml'
# scaffold:queue:begin
# Only the app applies the schema, and it serves /healthz once it has.
pane worker 'until curl -sf -o /dev/null "http://localhost:$HTTP_PORT/healthz"; do
	echo "waiting for the app on :$HTTP_PORT..."; sleep 2
done
exec go tool air -c .air.worker.toml'
# scaffold:queue:end
pane web 'vp -C web dev' # scaffold:web
title=$(printf ' + %s' "${services[@]}")
pane "${title:3}" "docker compose logs -f ${services[*]}"

tmux select-pane -t "=$SESSION:dev.0"

attach
