# Step 1: Modules caching
FROM golang:1.27-alpine3.23 AS modules

COPY go.mod go.sum /modules/

WORKDIR /modules

RUN go mod download

# Step 2: Builder
FROM golang:1.27-alpine3.23 AS builder

COPY --from=modules /go/pkg /go/pkg
COPY . /app

WORKDIR /app

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -tags migrate -o /bin/app ./cmd/app \
    # scaffold:queue:begin
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o /bin/worker ./cmd/worker \
    # scaffold:queue:end
    && mkdir /empty

# Step 3: Final
FROM scratch

COPY --from=builder /app/config /config
COPY --from=arigaio/atlas:1.3.4 /atlas /bin/atlas
COPY --from=builder /bin/app /app
# scaffold:queue:begin
# Same image, second entry point: the compose worker service runs /worker instead.
COPY --from=builder /bin/worker /worker
# scaffold:queue:end
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
# scratch has no /tmp, and atlasexec writes the schema to a temp dir before running atlas.
COPY --from=builder --chmod=1777 /empty /tmp

CMD ["/app"]
