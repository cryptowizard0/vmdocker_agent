FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /app/main .

FROM node:22-alpine AS openclaw-builder
RUN apk add --no-cache git cmake make g++ python3
RUN npm install -g openclaw@latest --ignore-scripts

FROM node:22-alpine

WORKDIR /app
RUN apk add --no-cache ca-certificates curl
COPY --from=builder /app/main /app/main
COPY --from=openclaw-builder /usr/local/lib/node_modules /usr/local/lib/node_modules
COPY --from=openclaw-builder /usr/local/bin/openclaw /usr/local/bin/openclaw
COPY docker_entrypoint.sh /usr/local/bin/docker_entrypoint.sh
RUN chmod +x /usr/local/bin/docker_entrypoint.sh

ENV RUNTIME_TYPE=openclaw
ENV OPENCLAW_GATEWAY_PORT=18789
ENV OPENCLAW_GATEWAY_URL=http://127.0.0.1:18789
ENV OPENCLAW_TIMEOUT_MS=30000
EXPOSE 8080 18789

CMD ["/usr/local/bin/docker_entrypoint.sh"]
