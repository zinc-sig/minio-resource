#!/bin/bash

set -e

echo "Building Minio Resource Docker image..."

# Build the Docker image
docker build -t minio-resource:latest .

echo "Docker image built successfully!"
echo ""
echo "To test the resource locally, you can run:"
echo "  docker run -i minio-resource:latest /opt/resource/check < test_input.json"
echo ""
echo "To push to a registry:"
echo "  docker tag minio-resource:latest your-registry/minio-resource:latest"
echo "  docker push your-registry/minio-resource:latest"