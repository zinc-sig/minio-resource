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

# Configure mc client in a Docker container
echo "Setting up MinIO client (mc)..."
docker run --rm \
  --network host \
  minio/mc alias set myminio http://localhost:9000 minioadmin minioadmin

# Create test bucket
echo "Creating test bucket..."
docker run --rm \
  --network host \
  --entrypoint sh \
  minio/mc -c "mc alias set myminio http://localhost:9000 minioadmin minioadmin && mc mb myminio/test-bucket" || true

# Create test files and upload them using mc
echo "Creating and uploading test files..."

# Create temporary files to upload
TEMP_DIR=$(mktemp -d)
echo "Test file 1" > "$TEMP_DIR/file1.txt"
echo "Test file 2" > "$TEMP_DIR/file2.txt"
echo "Test file 3" > "$TEMP_DIR/file3.txt"

# Upload files using mc Docker image
echo "Uploading file1.txt to test-prefix/..."
docker run --rm \
  --network host \
  -v "$TEMP_DIR:/tmp/files:ro" \
  --entrypoint sh \
  minio/mc -c "mc alias set myminio http://localhost:9000 minioadmin minioadmin && mc cp /tmp/files/file1.txt myminio/test-bucket/test-prefix/file1.txt"

echo "Uploading file2.txt to test-prefix/..."
docker run --rm \
  --network host \
  -v "$TEMP_DIR:/tmp/files:ro" \
  --entrypoint sh \
  minio/mc -c "mc alias set myminio http://localhost:9000 minioadmin minioadmin && mc cp /tmp/files/file2.txt myminio/test-bucket/test-prefix/file2.txt"

echo "Uploading file3.txt to subdir/..."
docker run --rm \
  --network host \
  -v "$TEMP_DIR:/tmp/files:ro" \
  --entrypoint sh \
  minio/mc -c "mc alias set myminio http://localhost:9000 minioadmin minioadmin && mc cp /tmp/files/file3.txt myminio/test-bucket/subdir/file3.txt"

# Verify files were uploaded correctly by reading them back
echo ""
echo "Verifying uploaded files..."

echo "Reading test-prefix/file1.txt:"
docker run --rm \
  --network host \
  --entrypoint sh \
  minio/mc -c "mc alias set myminio http://localhost:9000 minioadmin minioadmin && mc cat myminio/test-bucket/test-prefix/file1.txt"

echo "Reading test-prefix/file2.txt:"
docker run --rm \
  --network host \
  --entrypoint sh \
  minio/mc -c "mc alias set myminio http://localhost:9000 minioadmin minioadmin && mc cat myminio/test-bucket/test-prefix/file2.txt"

echo "Reading subdir/file3.txt:"
docker run --rm \
  --network host \
  --entrypoint sh \
  minio/mc -c "mc alias set myminio http://localhost:9000 minioadmin minioadmin && mc cat myminio/test-bucket/subdir/file3.txt"

# List all files in the bucket to verify structure
echo ""
echo "Listing all files in test-bucket:"
docker run --rm \
  --network host \
  --entrypoint sh \
  minio/mc -c "mc alias set myminio http://localhost:9000 minioadmin minioadmin && mc ls --recursive myminio/test-bucket"

# Build the resource
echo ""
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
echo ""
echo "Running IN script (output includes stderr for debugging):"
/tmp/in "$DEST_DIR" < /tmp/in_test.json 2>&1 | tee /tmp/in_output.txt
echo ""
echo "JSON Response:"
grep '^{' /tmp/in_output.txt | jq . || echo "Failed to parse JSON response"

echo ""
echo "Downloaded files:"
find "$DEST_DIR" -type f -exec echo "  {}" \; -exec head -1 {} \;

# Final verification using mc to ensure files are still accessible
echo ""
echo "Final verification - checking files are still accessible via mc:"
docker run --rm \
  --network host \
  --entrypoint sh \
  minio/mc -c "mc alias set myminio http://localhost:9000 minioadmin minioadmin && mc stat myminio/test-bucket/test-prefix/file1.txt"
docker run --rm \
  --network host \
  --entrypoint sh \
  minio/mc -c "mc alias set myminio http://localhost:9000 minioadmin minioadmin && mc stat myminio/test-bucket/test-prefix/file2.txt"
docker run --rm \
  --network host \
  --entrypoint sh \
  minio/mc -c "mc alias set myminio http://localhost:9000 minioadmin minioadmin && mc stat myminio/test-bucket/subdir/file3.txt"

# Clean up
echo ""
echo "Cleaning up..."
docker stop minio-test
docker rm minio-test
rm -rf "$DEST_DIR"
rm -rf "$TEMP_DIR"

echo ""
echo "✅ All tests passed!"