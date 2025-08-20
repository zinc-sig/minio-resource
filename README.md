# Minio Resource for Concourse CI

A Concourse CI resource type for downloading all files from a Minio/S3-compatible bucket with a given path prefix.

## Features

- **Bulk Download**: Downloads all files matching a path prefix from a Minio bucket
- **Version Tracking**: Tracks changes using ETags and modification times
- **Parallel Downloads**: Configurable parallel download support for better performance
- **Directory Structure Preservation**: Maintains the original directory structure when downloading
- **SSL Support**: Configurable SSL/TLS with optional certificate verification
- **Flexible Authentication**: Supports access key and secret key injection

## Source Configuration

| Parameter | Required | Description |
|-----------|----------|-------------|
| `endpoint` | Yes | Minio server endpoint (e.g., `minio.example.com` or `localhost:9000`) |
| `access_key` | Yes | Minio access key ID |
| `secret_key` | Yes | Minio secret access key |
| `bucket` | Yes | Name of the bucket to access |
| `path_prefix` | No | Path prefix to filter objects (e.g., `data/exports/`) |
| `use_ssl` | No | Enable SSL/TLS connection (default: `true`) |
| `region` | No | AWS region (for S3-compatible services) |
| `skip_ssl_verification` | No | Skip SSL certificate verification (default: `false`) |

## Behavior

### `check`: Detect new versions

The check script lists all objects in the bucket with the specified path prefix and returns them as versions. Each object is tracked by:
- Path
- ETag (for content changes)
- Last modified timestamp

New versions are detected when:
- New files are added to the bucket
- Existing files are modified (ETag changes)
- Files are updated (modification time changes)

### `in`: Download all files

The in script downloads **all files** from the bucket that match the configured path prefix. Files are downloaded to the destination directory while preserving the directory structure.

#### Parameters

| Parameter | Required | Description |
|-----------|----------|-------------|
| `parallel` | No | Number of parallel downloads (default: 5) |

### `out`: Upload files (optional)

The out script is disabled by default since this resource is primarily designed for downloading. To enable uploads:

#### Parameters

| Parameter | Required | Description |
|-----------|----------|-------------|
| `upload_enabled` | No | Enable file uploads (default: `false`) |
| `file` | No | File pattern to upload (default: `*`) |

## Example Pipeline Configuration

```yaml
resource_types:
- name: minio-resource
  type: docker-image
  source:
    repository: your-registry/minio-resource
    tag: latest

resources:
- name: minio-files
  type: minio-resource
  source:
    endpoint: minio.example.com
    access_key: ((minio-access-key))
    secret_key: ((minio-secret-key))
    bucket: my-bucket
    path_prefix: data/exports/
    use_ssl: true

jobs:
- name: process-files
  plan:
  - get: minio-files
    trigger: true
    params:
      parallel: 10  # Download 10 files concurrently
  - task: process
    config:
      platform: linux
      image_resource:
        type: docker-image
        source:
          repository: ubuntu
      inputs:
      - name: minio-files
      run:
        path: bash
        args:
        - -c
        - |
          echo "Processing downloaded files..."
          find minio-files -type f -name "*.txt" | while read file; do
            echo "Processing: $file"
            # Your processing logic here
          done
```

## Using with Concourse Credentials Manager

Store your Minio credentials securely using Concourse's credential management:

```yaml
# In your pipeline
resources:
- name: minio-files
  type: minio-resource
  source:
    endpoint: minio.example.com
    access_key: ((minio.access_key))
    secret_key: ((minio.secret_key))
    bucket: ((minio.bucket))
    path_prefix: data/
```

Then set the credentials using the Concourse CLI:
```bash
fly -t my-target set-pipeline \
  -p my-pipeline \
  -c pipeline.yml \
  -v minio.access_key=AKIAIOSFODNN7EXAMPLE \
  -v minio.secret_key=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY \
  -v minio.bucket=my-bucket
```

## Building and Testing

### Building the Docker Image

```bash
./scripts/build.sh
```

### Running Tests

```bash
# Run unit tests
./scripts/test.sh

# Test with a local Minio instance
./scripts/test_local.sh
```

### Testing Locally with Docker

1. Create a test configuration file:
```json
{
  "source": {
    "endpoint": "play.min.io",
    "access_key": "minioadmin",
    "secret_key": "minioadmin",
    "bucket": "test-bucket",
    "path_prefix": "test/"
  }
}
```

2. Test the check script:
```bash
docker run -i minio-resource:latest /opt/resource/check < test_config.json
```

3. Test the in script:
```bash
docker run -i -v /tmp/output:/tmp/output minio-resource:latest \
  /opt/resource/in /tmp/output < test_config.json
```

## Development

### Project Structure

```
.
├── cmd/
│   ├── check/      # Check script implementation
│   ├── in/         # In script implementation
│   └── out/        # Out script implementation
├── pkg/
│   ├── models/     # Data models for requests/responses
│   └── minio/      # Minio client wrapper
├── scripts/        # Build and test scripts
├── Dockerfile      # Container image definition
├── go.mod         # Go module definition
└── README.md      # This file
```

### Adding New Features

1. Modify the appropriate script in `cmd/`
2. Update models if needed in `pkg/models/`
3. Add any new Minio operations to `pkg/minio/`
4. Update tests and documentation
5. Build and test the Docker image

## Troubleshooting

### SSL Certificate Issues

If you encounter SSL certificate verification errors with self-signed certificates:

```yaml
source:
  endpoint: minio.internal.company.com
  skip_ssl_verification: true  # Only for testing/development
  use_ssl: true
```

### Permission Errors

Ensure your access key has the necessary permissions:
- `s3:ListBucket` - Required for check and in scripts
- `s3:GetObject` - Required for in script
- `s3:PutObject` - Required for out script (if uploads enabled)

### Debugging

Enable verbose output by checking stderr in your Concourse task:

```yaml
- task: debug
  config:
    run:
      path: sh
      args:
      - -c
      - |
        # The resource writes debug info to stderr
        cat minio-files/.resource_version.json
```

## License

Apache 2.0

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## Support

For issues and questions, please open an issue on the GitHub repository.