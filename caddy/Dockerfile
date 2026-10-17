# Caddy with the caddy-waf (fabriziosalmi) and Souin cache (darkweak) modules, serving the React
# client from web/. The build context is the repository root; Dockerfile.dockerignore narrows it.

# The web client, built with the Vite+ toolchain image (vp provisions web/.node-version and bun).
FROM ghcr.io/voidzero-dev/vite-plus:1.3.0 AS web

WORKDIR /web

COPY --chown=vp:vp web/package.json web/bun.lock web/.node-version ./
RUN vp install --frozen-lockfile

COPY --chown=vp:vp web/ ./
RUN vp build

# Souin ships no storage of its own since v1.7.0, so the in-memory otter storage is built in too.
FROM caddy:2.11.6-builder AS builder

ARG CADDY_WAF_VERSION=v0.4.15

RUN xcaddy build \
        --with github.com/fabriziosalmi/caddy-waf@${CADDY_WAF_VERSION} \
        --with github.com/darkweak/souin/plugins/caddy@v1.7.9 \
        --with github.com/darkweak/storages/otter/caddy@v0.0.20 \
    && cp "$(go env GOMODCACHE)/github.com/fabriziosalmi/caddy-waf@${CADDY_WAF_VERSION}/rules.json" /waf-rules.json

FROM caddy:2.11.7-alpine

COPY --from=builder /usr/bin/caddy /usr/bin/caddy
COPY --from=builder /waf-rules.json /etc/caddy/waf/rules.json
COPY caddy/Caddyfile /etc/caddy/Caddyfile
COPY --from=web /web/dist /srv/web
