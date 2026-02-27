#!/bin/bash

if [ "$1" != "golua" ] && [ "$1" != "ollama" ]; then
    echo "Usage: $0 [golua|ollama] <VERSION>"
    echo "  golua   - Build golua runtime image"
    echo "  ollama  - Build ollama runtime image"
    echo "  VERSION - Image version tag (e.g., v1.0.0, latest)"
    exit 1
fi

if [ -z "$2" ]; then
    echo "No VERSION specified"
    echo "Usage: $0 [golua|ollama] <VERSION>"
    exit 1
fi

if [ "$1" = "golua" ]; then
    docker build --progress=plain \
        -f Dockerfile.golua \
        -t chriswebber/docker-golua:"$2" .
else
    docker build --progress=plain \
        -f Dockerfile.ollama \
        -t chriswebber/docker-ollama:"$2" .
fi