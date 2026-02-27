#!/bin/bash

if [ -z "$1" ]; then
    echo "No VERSION specified"
    echo "Usage: $0 <VERSION>"
    echo "  VERSION - Image version tag (e.g., v1.0.0, latest)"
    exit 1
fi

docker build --progress=plain \
    -f Dockerfile.testrt \
    -t chriswebber/docker-testrt:"$1" .
