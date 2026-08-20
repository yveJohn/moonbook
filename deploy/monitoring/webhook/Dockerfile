FROM golang:1.24.2-alpine3.21 AS builder
WORKDIR /src
COPY go.mod main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/monitoring-webhook .

FROM alpine:3.21
RUN addgroup -S moonbook && adduser -S -G moonbook moonbook
COPY --from=builder /out/monitoring-webhook /usr/local/bin/monitoring-webhook
USER moonbook
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/monitoring-webhook"]
