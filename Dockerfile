FROM golang:1.24 AS builder

WORKDIR /app
COPY go.mod ./
RUN go mod tidy

COPY . .
RUN CGO_ENABLED=0 go build -o /app/main .
RUN CGO_ENABLED=0 go build -o /app/bootstrap ./cmd/bootstrap

FROM ghcr.io/openclaw/openclaw:latest

USER root
WORKDIR /app
COPY --from=builder /app/main /app/main
COPY --from=builder /app/bootstrap /app/bootstrap
COPY start-vmdocker-agent.sh /usr/local/bin/start-vmdocker-agent.sh
COPY docker_entrypoint.sh /usr/local/bin/docker_entrypoint.sh
COPY openclaw.default.json /app/openclaw.default.json
RUN set -eux; \
    if command -v apt-get >/dev/null 2>&1; then \
        apt-get update; \
        DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends curl bash ca-certificates; \
        rm -rf /var/lib/apt/lists/*; \
    elif command -v apk >/dev/null 2>&1; then \
        apk add --no-cache curl bash ca-certificates; \
    elif command -v microdnf >/dev/null 2>&1; then \
        microdnf install -y curl bash ca-certificates; \
        microdnf clean all; \
    fi
RUN chmod +x /usr/local/bin/start-vmdocker-agent.sh /usr/local/bin/docker_entrypoint.sh /app/main /app/bootstrap
RUN chown -R 65532:65532 /app && chmod 644 /app/openclaw.default.json

ENV RUNTIME_TYPE=openclaw
ENV OPENCLAW_GATEWAY_PORT=18789
ENV OPENCLAW_GATEWAY_BIND=loopback
ENV OPENCLAW_GATEWAY_URL=http://127.0.0.1:18789
ENV OPENCLAW_CONFIG_TEMPLATE_PATH=/app/openclaw.default.json
ENV OPENCLAW_TIMEOUT_MS=30000
ENV HOME=/tmp
EXPOSE 8080 18789

USER 65532:65532
CMD ["/usr/local/bin/start-vmdocker-agent.sh"]
