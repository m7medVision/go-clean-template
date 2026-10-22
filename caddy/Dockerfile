# Caddy with the caddy-waf (fabriziosalmi) and Souin cache (darkweak) modules. The build context is
# caddy/.

# scaffold:web:begin
# The web client, built with the Vite+ toolchain image (vp provisions web/.node-version and bun) from
# the web-src context: web/, narrowed by web/.dockerignore.
FROM ghcr.io/voidzero-dev/vite-plus:1.3.0 AS web

WORKDIR /web

COPY --from=web-src --chown=vp:vp package.json bun.lock .node-version ./
RUN vp install --frozen-lockfile

COPY --from=web-src --chown=vp:vp . ./
RUN vp build

# scaffold:web:end
# Souin ships no storage of its own since v1.7.0, so the in-memory otter storage is built in too.
FROM caddy:2.11.7-builder AS builder

ARG CADDY_WAF_VERSION=v0.4.15

RUN xcaddy build \
        --with github.com/fabriziosalmi/caddy-waf@${CADDY_WAF_VERSION} \
        --with github.com/darkweak/souin/plugins/caddy@v1.7.9 \
        --with github.com/darkweak/storages/otter/caddy@v0.0.20 \
    && cp "$(go env GOMODCACHE)/github.com/fabriziosalmi/caddy-waf@${CADDY_WAF_VERSION}/rules.json" /waf-rules.json

FROM caddy:2.11.7-alpine

COPY --from=builder /usr/bin/caddy /usr/bin/caddy
COPY --from=builder /waf-rules.json /etc/caddy/waf/rules.json
COPY Caddyfile /etc/caddy/Caddyfile
# scaffold:web:begin
COPY --from=web /web/dist /srv/web
# scaffold:web:end
