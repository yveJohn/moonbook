FROM golang:1.24.2-alpine3.21 AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/moonbook-server . \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/moonbook-migrate ./cmd/moonbook-migrate \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/moonbook-admin ./cmd/moonbook-admin \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/moonbook-finance-reconcile ./cmd/moonbook-finance-reconcile \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/moonbook-legacy-migrate ./cmd/moonbook-legacy-migrate \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/moonbook-migration-audit ./cmd/moonbook-migration-audit \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/moonbook-config ./cmd/moonbook-config

FROM alpine:3.21

RUN apk add --no-cache ca-certificates curl tzdata \
    && addgroup -S moonbook \
    && adduser -S -G moonbook -h /app moonbook

WORKDIR /app
COPY --from=builder /out/ /app/
COPY --chown=moonbook:moonbook config.moonbook.yaml.tpl /app/config.moonbook.yaml.tpl
COPY --chown=moonbook:moonbook resource /app/resource
COPY --chown=moonbook:moonbook deploy/compose/server-entrypoint.sh /usr/local/bin/moonbook-entrypoint

RUN chmod 0755 /usr/local/bin/moonbook-entrypoint \
    && mkdir -p /run/moonbook /app/log /app/uploads \
    && chown -R moonbook:moonbook /run/moonbook /app/log /app/uploads

USER moonbook
EXPOSE 8888
ENTRYPOINT ["moonbook-entrypoint"]
CMD ["moonbook-server", "-c", "/run/moonbook/config.yaml"]
