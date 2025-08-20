package minio

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/zinc-sig/minio-resource/pkg/models"
)

// Client wraps the Minio client with helper methods
type Client struct {
	client     *minio.Client
	bucket     string
	pathPrefix string
}

// NewClient creates a new Minio client from the provided source configuration
func NewClient(source models.Source) (*Client, error) {
	// Create Minio client options
	opts := &minio.Options{
		Creds:  credentials.NewStaticV4(source.AccessKey, source.SecretKey, ""),
		Secure: source.UseSSLValue(),
		Region: source.Region,
	}

	// Configure SSL verification
	if source.SkipSSLVerification && source.UseSSLValue() {
		opts.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}

	// Create the client
	minioClient, err := minio.New(source.Endpoint, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	// Ensure path prefix ends with / if not empty
	pathPrefix := source.PathPrefix
	if pathPrefix != "" && !strings.HasSuffix(pathPrefix, "/") {
		pathPrefix += "/"
	}

	return &Client{
		client:     minioClient,
		bucket:     source.Bucket,
		pathPrefix: pathPrefix,
	}, nil
}

// ObjectInfo contains information about an object in the bucket
type ObjectInfo struct {
	Path         string
	ETag         string
	LastModified time.Time
	Size         int64
}

// ListObjects lists all objects in the bucket with the configured path prefix
func (c *Client) ListObjects(ctx context.Context) ([]ObjectInfo, error) {
	opts := minio.ListObjectsOptions{
		Prefix:    c.pathPrefix,
		Recursive: true,
	}

	var objects []ObjectInfo
	for object := range c.client.ListObjects(ctx, c.bucket, opts) {
		if object.Err != nil {
			return nil, fmt.Errorf("error listing objects: %w", object.Err)
		}

		// Skip directories (they have size 0 and end with /)
		if strings.HasSuffix(object.Key, "/") && object.Size == 0 {
			continue
		}

		objects = append(objects, ObjectInfo{
			Path:         object.Key,
			ETag:         object.ETag,
			LastModified: object.LastModified,
			Size:         object.Size,
		})
	}

	return objects, nil
}

// GetObject downloads a single object from the bucket
func (c *Client) GetObject(ctx context.Context, objectPath string) (io.ReadCloser, error) {
	object, err := c.client.GetObject(ctx, c.bucket, objectPath, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get object %s: %w", objectPath, err)
	}
	return object, nil
}

// DownloadResult contains the result of a download operation
type DownloadResult struct {
	Path  string
	Error error
}

// DownloadAllObjects downloads all objects with the configured path prefix to the destination directory
func (c *Client) DownloadAllObjects(ctx context.Context, destDir string, parallel int) ([]DownloadResult, error) {
	// List all objects first
	objects, err := c.ListObjects(ctx)
	for _, object := range objects {
		if err != nil {
			return nil, err
		}
		fmt.Println(object.Path)
	}

	if parallel <= 0 {
		parallel = 5 // Default parallelism
	}

	// Create a semaphore for controlling parallelism
	sem := make(chan struct{}, parallel)
	var wg sync.WaitGroup
	results := make([]DownloadResult, len(objects))

	for i, obj := range objects {
		wg.Add(1)
		go func(idx int, object ObjectInfo) {
			defer wg.Done()

			sem <- struct{}{}        // Acquire semaphore
			defer func() { <-sem }() // Release semaphore

			result := DownloadResult{Path: object.Path}

			// Calculate local path by removing the prefix
			localPath := strings.TrimPrefix(object.Path, c.pathPrefix)
			if localPath == "" {
				localPath = filepath.Base(object.Path)
			}

			fullPath := filepath.Join(destDir, localPath)

			// Create directory if needed
			dir := filepath.Dir(fullPath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				result.Error = fmt.Errorf("failed to create directory %s: %w", dir, err)
				results[idx] = result
				return
			}

			// Download the object
			if err := c.downloadObject(ctx, object.Path, fullPath); err != nil {
				result.Error = err
			}

			results[idx] = result
		}(i, obj)
	}

	wg.Wait()
	return results, nil
}

// downloadObject downloads a single object to a file
func (c *Client) downloadObject(ctx context.Context, objectPath, destPath string) error {
	// Get the object
	object, err := c.client.GetObject(ctx, c.bucket, objectPath, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to get object %s: %w", objectPath, err)
	}
	defer object.Close()

	// Create the destination file
	file, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", destPath, err)
	}
	defer file.Close()

	// Copy the content
	_, err = io.Copy(file, object)
	if err != nil {
		return fmt.Errorf("failed to write file %s: %w", destPath, err)
	}

	return nil
}

// PutObject uploads an object to the bucket
func (c *Client) PutObject(ctx context.Context, objectPath string, reader io.Reader, size int64, contentType string) error {
	opts := minio.PutObjectOptions{
		ContentType: contentType,
	}

	_, err := c.client.PutObject(ctx, c.bucket, objectPath, reader, size, opts)
	if err != nil {
		return fmt.Errorf("failed to put object %s: %w", objectPath, err)
	}

	return nil
}

// BucketExists checks if the configured bucket exists and is accessible
func (c *Client) BucketExists(ctx context.Context) (bool, error) {
	exists, err := c.client.BucketExists(ctx, c.bucket)
	if err != nil {
		return false, fmt.Errorf("failed to check bucket existence: %w", err)
	}
	return exists, nil
}
