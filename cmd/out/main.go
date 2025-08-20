package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	minioClient "github.com/zinc-sig/minio-resource/pkg/minio"
	"github.com/zinc-sig/minio-resource/pkg/models"
)

func main() {
	// Get source directory from command line
	if len(os.Args) < 2 {
		fatal("usage: %s <source>", os.Args[0])
	}
	sourceDir := os.Args[1]

	// Parse input
	var request models.OutRequest
	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		fatal("failed to decode request: %v", err)
	}

	// Validate source configuration
	if err := validateSource(request.Source); err != nil {
		fatal("invalid source configuration: %v", err)
	}

	// Check if upload is disabled (default behavior for download-only resource)
	uploadDisabled := true
	if request.Params != nil {
		if enabled, ok := request.Params["upload_enabled"].(bool); ok {
			uploadDisabled = !enabled
		}
	}

	if uploadDisabled {
		// Return a minimal response for no-op out
		fmt.Fprintf(os.Stderr, "Upload is disabled. This resource is configured for download-only operation.\n")
		fmt.Fprintf(os.Stderr, "To enable uploads, set params.upload_enabled to true in your pipeline.\n")

		// Return empty version with current timestamp
		response := models.OutResponse{
			Version: models.Version{
				Path:         "no-upload",
				ETag:         "disabled",
				LastModified: time.Now(),
			},
			Metadata: []models.Metadata{
				{
					Name:  "upload_status",
					Value: "disabled",
				},
			},
		}

		if err := json.NewEncoder(os.Stdout).Encode(response); err != nil {
			fatal("failed to encode response: %v", err)
		}
		return
	}

	// If upload is enabled, proceed with upload logic
	// Get the file pattern to upload
	filePattern := "*"
	if request.Params != nil {
		if pattern, ok := request.Params["file"].(string); ok {
			filePattern = pattern
		}
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

	// Find files to upload
	pattern := filepath.Join(sourceDir, filePattern)
	files, err := filepath.Glob(pattern)
	if err != nil {
		fatal("failed to find files with pattern %s: %v", pattern, err)
	}

	if len(files) == 0 {
		fatal("no files found matching pattern: %s", filePattern)
	}

	var uploadedFiles []string
	var lastVersion models.Version

	// Upload each file
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to stat file %s: %v\n", file, err)
			continue
		}

		if info.IsDir() {
			continue
		}

		// Calculate object path
		relativePath := strings.TrimPrefix(file, sourceDir)
		relativePath = strings.TrimPrefix(relativePath, "/")

		objectPath := filepath.Join(request.Source.PathPrefix, relativePath)
		objectPath = strings.ReplaceAll(objectPath, "\\", "/") // Ensure forward slashes

		// Open file
		reader, err := os.Open(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to open file %s: %v\n", file, err)
			continue
		}
		defer reader.Close()

		// Upload file
		fmt.Fprintf(os.Stderr, "Uploading %s to %s\n", file, objectPath)
		err = client.PutObject(ctx, objectPath, reader, info.Size(), "application/octet-stream")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to upload file %s: %v\n", file, err)
			continue
		}

		uploadedFiles = append(uploadedFiles, objectPath)

		// Keep track of last uploaded file for version
		lastVersion = models.Version{
			Path:         objectPath,
			ETag:         fmt.Sprintf("upload-%d", time.Now().Unix()),
			LastModified: time.Now(),
		}
	}

	if len(uploadedFiles) == 0 {
		fatal("no files were uploaded successfully")
	}

	// Prepare metadata
	metadata := []models.Metadata{
		{
			Name:  "files_uploaded",
			Value: fmt.Sprintf("%d", len(uploadedFiles)),
		},
		{
			Name:  "upload_pattern",
			Value: filePattern,
		},
	}

	// Output the response
	response := models.OutResponse{
		Version:  lastVersion,
		Metadata: metadata,
	}

	if err := json.NewEncoder(os.Stdout).Encode(response); err != nil {
		fatal("failed to encode response: %v", err)
	}

	fmt.Fprintf(os.Stderr, "Successfully uploaded %d files\n", len(uploadedFiles))
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
