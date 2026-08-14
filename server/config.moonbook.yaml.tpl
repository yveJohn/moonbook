# Moonbook modification notice: local generated configuration template, 2026-08-14.
jwt:
  signing-key: ${MOONBOOK_JWT_SIGNING_KEY}
  expires-time: 7d
  buffer-time: 1d
  issuer: moonbook

zap:
  level: info
  prefix: "[moonbook]"
  format: json
  director: log
  encode-level: LowercaseLevelEncoder
  stacktrace-key: stacktrace
  show-line: true
  log-in-console: true
  retention-day: 14
  access-req-body: false
  access-resp-data: false
  access-req-headers: false
  access-log-max-bytes: 1024
  file-only-modules: []

redis:
  useCluster: false
  addr: ${MOONBOOK_REDIS_ADDR}
  password: ${REDIS_PASSWORD}
  db: 0
  clusterAddrs: []

redis-list: []

mongo:
  coll: ""
  options: ""
  database: ""
  username: ""
  password: ""
  auth-source: ""
  min-pool-size: 0
  max-pool-size: 0
  socket-timeout-ms: 0
  connect-timeout-ms: 0
  is-zap: false
  hosts: []

email:
  to: ""
  port: 0
  from: ""
  host: ""
  is-ssl: false
  secret: ""
  nickname: "Moonbook"

system:
  addr: ${MOONBOOK_SERVER_PORT}
  db-type: pgsql
  oss-type: minio
  use-redis: true
  use-mongo: false
  use-multipoint: false
  iplimit-count: 15000
  iplimit-time: 3600
  router-prefix: ""
  use-strict-auth: false
  disable-auto-migrate: true

pgsql:
  path: ${MOONBOOK_POSTGRES_HOST}
  port: ${MOONBOOK_POSTGRES_PORT}
  config: "sslmode=disable TimeZone=Asia/Kuala_Lumpur"
  db-name: ${POSTGRES_DB}
  username: ${POSTGRES_USER}
  password: ${POSTGRES_PASSWORD}
  max-idle-conns: 10
  max-open-conns: 50
  conn-max-lifetime: 3600
  log-mode: warn

mysql: {}
oracle: {}
mssql: {}
sqlite: {}
db-list: []

local:
  path: uploads/file
  store-path: uploads/file

autocode:
  web: web/src
  root: ""
  server: server
  module: github.com/flipped-aurora/gin-vue-admin/server
  ai-path: ""

qiniu: {}
aliyun-oss: {}
tencent-cos: {}
aws-s3: {}
cloudflare-r2: {}
hua-wei-obs: {}

minio:
  endpoint: ${MOONBOOK_MINIO_ENDPOINT}
  access-key-id: ${MINIO_ROOT_USER}
  access-key-secret: ${MINIO_ROOT_PASSWORD}
  bucket-name: ${MINIO_BUCKET}
  use-ssl: false
  base-path: ""
  bucket-url: ${MOONBOOK_MINIO_BUCKET_URL}

media:
  chunk-dir: uploads/chunks
  max-file-size: 0
  session-ttl: 24

disk-list:
  - mount-point: "/"

cors:
  mode: strict-whitelist
  whitelist:
    - allow-origin: http://localhost:8080
      allow-headers: Content-Type,Authorization,X-Token,X-User-Id
      allow-methods: GET,POST,PUT,PATCH,DELETE,OPTIONS
      expose-headers: Content-Length,Content-Type
      allow-credentials: true
    - allow-origin: http://localhost:5174
      allow-headers: Content-Type,Authorization,X-Token,X-User-Id
      allow-methods: GET,POST,PUT,PATCH,DELETE,OPTIONS
      expose-headers: Content-Length,Content-Type
      allow-credentials: true
    - allow-origin: ${MOONBOOK_READER_ORIGIN}
      allow-headers: Content-Type,Authorization,X-Token,X-User-Id
      allow-methods: GET,POST,PUT,PATCH,DELETE,OPTIONS
      expose-headers: Content-Length,Content-Type
      allow-credentials: true

mcp:
  name: MOONBOOK_MCP
  version: v1.0.0
  addr: 18889

app:
  node: local
  app-id: moonbook
  env: dev

metrics:
  enabled: true
  token: ${MOONBOOK_METRICS_TOKEN}
