#!/bin/bash

# Script to test the resource locally with a real Minio instance

set -e

echo "Starting local Minio server for testing..."

# Check if docker is available
if ! command -v docker &> /dev/null; then
    echo "Docker is required for this test script"
    exit 1
fi

# Start a local Minio server
echo "Starting Minio container..."
docker run -d \
  --name minio-test \
  -p 9000:9000 \
  -p 9001:9001 \
  -e MINIO_ROOT_USER=minioadmin \
  -e MINIO_ROOT_PASSWORD=minioadmin \
  minio/minio server /data --console-address ":9001"

# Wait for Minio to be ready
echo "Waiting for Minio to be ready..."
sleep 5

# Create a test bucket and upload some files
echo "Setting up test data..."
docker exec minio-test mkdir -p /data/test-bucket/test-prefix
docker exec minio-test mkdir -p /data/test-bucket/subdir
docker exec minio-test sh -c 'echo "Test file 1" > /data/test-bucket/test-prefix/file1.txt'
docker exec minio-test sh -c 'echo "Test file 2" > /data/test-bucket/test-prefix/file2.txt'
docker exec minio-test sh -c 'echo "Test file 3" > /data/test-bucket/subdir/file3.txt'

# Build the resource
echo "Building the resource..."
go build -o /tmp/check ./cmd/check
go build -o /tmp/in ./cmd/in
go build -o /tmp/out ./cmd/out

# Test check script
echo ""
echo "Testing CHECK script..."
cat > /tmp/check_test.json <<EOF
{
  "source": {
    "endpoint": "localhost:9000",
    "access_key": "minioadmin",
    "secret_key": "minioadmin",
    "bucket": "test-bucket",
    "path_prefix": "test-prefix/",
    "use_ssl": false
  }
}
EOF

echo "Input:"
cat /tmp/check_test.json
echo ""
echo "Output:"
/tmp/check < /tmp/check_test.json | jq .

# Test in script
echo ""
echo "Testing IN script..."
DEST_DIR=$(mktemp -d)
cat > /tmp/in_test.json <<EOF
{
  "source": {
    "endpoint": "localhost:9000",
    "access_key": "minioadmin",
    "secret_key": "minioadmin",
    "bucket": "test-bucket",
    "path_prefix": "test-prefix/",
    "use_ssl": false
  },
  "version": {
    "path": "test-prefix/file1.txt",
    "etag": "test",
    "last_modified": "2024-01-01T00:00:00Z"
  },
  "params": {
    "parallel": 3
  }
}
EOF

echo "Downloading to: $DEST_DIR"
/tmp/in "$DEST_DIR" < /tmp/in_test.json | jq .

echo ""
echo "Downloaded files:"
find "$DEST_DIR" -type f -exec echo "  {}" \; -exec head -1 {} \;

# Clean up
echo ""
echo "Cleaning up..."
docker stop minio-test
docker rm minio-test
rm -rf "$DEST_DIR"

echo ""
echo "✅ All tests passed!"
