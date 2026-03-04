FROM golang:1.24 AS builder

WORKDIR /app
COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /app/main .

FROM ghcr.io/openclaw/openclaw:latest

USER root
WORKDIR /app
COPY --from=builder /app/main /app/main
COPY docker_entrypoint.sh /usr/local/bin/docker_entrypoint.sh
COPY openclaw.default.json /tmp/openclaw/openclaw.json
RUN chmod +x /usr/local/bin/docker_entrypoint.sh
RUN chown -R 65532:65532 /app /tmp/openclaw && chmod 755 /tmp/openclaw && chmod 644 /tmp/openclaw/openclaw.json

ENV RUNTIME_TYPE=openclaw
ENV OPENCLAW_GATEWAY_PORT=18789
ENV OPENCLAW_GATEWAY_BIND=loopback
ENV OPENCLAW_GATEWAY_URL=http://127.0.0.1:18789
ENV OPENCLAW_CONFIG_PATH=/tmp/openclaw/openclaw.json
ENV OPENCLAW_TIMEOUT_MS=30000
ENV HOME=/tmp
EXPOSE 8080 18789

USER 65532:65532
CMD ["/usr/local/bin/docker_entrypoint.sh"]
