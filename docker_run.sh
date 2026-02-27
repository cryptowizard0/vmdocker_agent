# ollama runtime 
docker run --name hymatrix-ollama \
    -d \
    -p 8080:8080 \
    -e RUNTIME_TYPE=ollama \
    hymatrix/docker-ollama:latest