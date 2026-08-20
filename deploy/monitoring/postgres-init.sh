#!/bin/sh
set -eu

if [ "${POSTGRES_EXPORTER_PASSWORD:-monitoring_not_configured}" = "monitoring_not_configured" ]; then
  echo "POSTGRES_EXPORTER_PASSWORD must be configured for the monitoring profile" >&2
  exit 2
fi

export PGPASSWORD="$POSTGRES_PASSWORD"
psql --host=postgres --username="$POSTGRES_USER" --dbname="$POSTGRES_DB" \
  --set=monitor_password="$POSTGRES_EXPORTER_PASSWORD" <<'SQL'
SELECT format('CREATE ROLE moonbook_monitor LOGIN PASSWORD %L', :'monitor_password')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'moonbook_monitor')
\gexec
SELECT format('ALTER ROLE moonbook_monitor PASSWORD %L', :'monitor_password')
\gexec
GRANT pg_monitor TO moonbook_monitor;
GRANT CONNECT ON DATABASE :"DBNAME" TO moonbook_monitor;
SQL
