# openclaw runtime
docker run --name hymatrix-openclaw \
    -d \
    -p 8080:8080 \
    -p 18789:18789 \
    -e RUNTIME_TYPE=openclaw \
    -e OPENCLAW_GATEWAY_URL=http://127.0.0.1:18789 \
    chriswebber/docker-openclaw:latest
