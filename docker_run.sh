# test runtime
docker run --name hymatrix-test \
    -d \
    -p 8080:8080 \
    -e RUNTIME_TYPE=test \
    chriswebber/docker-testrt:latest
