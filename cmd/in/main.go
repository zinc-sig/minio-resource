package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	minioClient "github.com/zinc-sig/minio-resource/pkg/minio"
	"github.com/zinc-sig/minio-resource/pkg/models"
)

func main() {
	// Get destination directory from command line
	if len(os.Args) < 2 {
		fatal("usage: %s <destination>", os.Args[0])
	}
	destination := os.Args[1]

	// Parse input
	var request models.InRequest
	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		fatal("failed to decode request: %v", err)
	}

	// Validate source configuration
	if err := validateSource(request.Source); err != nil {
		fatal("invalid source configuration: %v", err)
	}

	// Create Minio client
	client, err := minioClient.NewClient(request.Source)
	if err != nil {
		fatal("failed to create minio client: %v", err)
	}

	// Check bucket exists
	ctx := context.Background()
	exists, err := client.BucketExists(ctx)
	if err != nil {
		fatal("failed to check bucket existence: %v", err)
	}
	if !exists {
		fatal("bucket %s does not exist or is not accessible", request.Source.Bucket)
	}

	// Determine parallelism from params
	parallel := 5
	if request.Params != nil {
		if p, ok := request.Params["parallel"]; ok {
			switch v := p.(type) {
			case float64:
				parallel = int(v)
			case string:
				if pInt, err := strconv.Atoi(v); err == nil {
					parallel = pInt
				}
			}
		}
	}

	// Log what we're doing
	fmt.Fprintf(os.Stderr, "Downloading all files from bucket '%s' with prefix '%s'\n",
		request.Source.Bucket, request.Source.PathPrefix)
	fmt.Fprintf(os.Stderr, "Using %d parallel downloads\n", parallel)

	// Download all objects
	results, err := client.DownloadAllObjects(ctx, destination, parallel)
	if err != nil {
		fatal("failed to download objects: %v", err)
	}

	// Check for errors and collect metadata
	var metadata []models.Metadata
	successCount := 0
	failCount := 0

	for _, result := range results {
		if result.Error != nil {
			fmt.Fprintf(os.Stderr, "Error downloading %s: %v\n", result.Path, result.Error)
			failCount++
		} else {
			fmt.Fprintf(os.Stderr, "Downloaded: %s\n", result.Path)
			successCount++
		}
	}

	// Add download statistics as metadata
	metadata = append(metadata,
		models.Metadata{
			Name:  "files_downloaded",
			Value: strconv.Itoa(successCount),
		},
		models.Metadata{
			Name:  "files_failed",
			Value: strconv.Itoa(failCount),
		},
		models.Metadata{
			Name:  "path_prefix",
			Value: request.Source.PathPrefix,
		},
	)

	// Write version file (for debugging and tracking)
	versionFile := filepath.Join(destination, ".resource_version.json")
	versionData, _ := json.MarshalIndent(request.Version, "", "  ")
	if err := os.WriteFile(versionFile, versionData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to write version file: %v\n", err)
	}

	// If specific version was requested, include its metadata
	if request.Version.Path != "" {
		metadata = append(metadata,
			models.Metadata{
				Name:  "version_path",
				Value: request.Version.Path,
			},
			models.Metadata{
				Name:  "version_etag",
				Value: request.Version.ETag,
			},
			models.Metadata{
				Name:  "version_modified",
				Value: request.Version.LastModified.Format("2006-01-02 15:04:05"),
			},
		)
	}

	// Check if any downloads failed
	if failCount > 0 && successCount == 0 {
		fatal("all downloads failed")
	}

	// Output the response
	response := models.InResponse{
		Version:  request.Version,
		Metadata: metadata,
	}

	if err := json.NewEncoder(os.Stdout).Encode(response); err != nil {
		fatal("failed to encode response: %v", err)
	}

	// Log summary
	fmt.Fprintf(os.Stderr, "\nDownload complete: %d succeeded, %d failed\n", successCount, failCount)
}

func validateSource(source models.Source) error {
	if source.Endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}
	if source.AccessKey == "" {
		return fmt.Errorf("access_key is required")
	}
	if source.SecretKey == "" {
		return fmt.Errorf("secret_key is required")
	}
	if source.Bucket == "" {
		return fmt.Errorf("bucket is required")
	}
	return nil
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
