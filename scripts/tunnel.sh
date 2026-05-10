#!/bin/bash

# Default port to 8080 if not provided
PORT=${1:-8080}

echo "Starting Cloudflare Tunnel for http://localhost:$PORT..."
echo "Note: This uses the Docker version of cloudflared."

# Run cloudflared using docker
# --net=host allows the container to access localhost on the host machine (Linux only)
docker run --rm -it --net=host cloudflare/cloudflared:latest tunnel --url http://localhost:$PORT
