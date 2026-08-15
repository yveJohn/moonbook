#!/bin/sh
set -eu

required_vars='POSTGRES_DB POSTGRES_USER POSTGRES_PASSWORD REDIS_PASSWORD MINIO_ROOT_USER MINIO_ROOT_PASSWORD MINIO_BUCKET MOONBOOK_JWT_SIGNING_KEY MOONBOOK_METRICS_TOKEN'
for name in $required_vars; do
  value="$(printenv "$name" || true)"
  if [ -z "$value" ]; then
    echo "missing required environment variable: $name" >&2
    exit 2
  fi
done

: "${MOONBOOK_SERVER_PORT:=8888}"
: "${MOONBOOK_POSTGRES_HOST:=postgres}"
: "${MOONBOOK_POSTGRES_PORT:=5432}"
: "${MOONBOOK_REDIS_ADDR:=redis:6379}"
: "${MOONBOOK_MINIO_ENDPOINT:=minio:9000}"
: "${MOONBOOK_MINIO_BUCKET_URL:=http://minio:9000/${MINIO_BUCKET}}"

export MOONBOOK_SERVER_PORT MOONBOOK_POSTGRES_HOST MOONBOOK_POSTGRES_PORT
export MOONBOOK_REDIS_ADDR MOONBOOK_MINIO_ENDPOINT MOONBOOK_MINIO_BUCKET_URL

/app/moonbook-config render /app/config.moonbook.yaml.tpl /run/moonbook/config.yaml

if [ -z "${MOONBOOK_DATABASE_DSN:-}" ]; then
  MOONBOOK_DATABASE_DSN="$(/app/moonbook-config dsn)"
  export MOONBOOK_DATABASE_DSN
fi

case "${1:-}" in
  moonbook-server) shift; set -- /app/moonbook-server "$@" ;;
  moonbook-migrate) shift; set -- /app/moonbook-migrate "$@" ;;
  moonbook-admin) shift; set -- /app/moonbook-admin "$@" ;;
  moonbook-finance-reconcile) shift; set -- /app/moonbook-finance-reconcile "$@" ;;
esac

exec "$@"
