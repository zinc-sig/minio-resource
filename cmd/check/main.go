package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	minioClient "github.com/zinc-sig/minio-resource/pkg/minio"
	"github.com/zinc-sig/minio-resource/pkg/models"
)

func main() {
	// Parse input
	var request models.CheckRequest
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

	// List objects in the bucket
	objects, err := client.ListObjects(ctx)
	if err != nil {
		fatal("failed to list objects: %v", err)
	}

	// Convert objects to versions
	versions := make([]models.Version, 0, len(objects))
	for _, obj := range objects {
		version := models.Version{
			Path:         obj.Path,
			ETag:         obj.ETag,
			LastModified: obj.LastModified,
		}

		// If we have a current version, only include newer objects
		if request.Version.Path != "" {
			// Skip if this is the same object (by path and etag)
			if version.Path == request.Version.Path && version.ETag == request.Version.ETag {
				continue
			}

			// Skip if this object is older than the provided version
			if !version.LastModified.After(request.Version.LastModified) {
				continue
			}
		}

		versions = append(versions, version)
	}

	// Sort versions by last modified time (oldest first as per Concourse requirements)
	sort.Slice(versions, func(i, j int) bool {
		// If times are equal, sort by path for consistency
		if versions[i].LastModified.Equal(versions[j].LastModified) {
			return versions[i].Path < versions[j].Path
		}
		return versions[i].LastModified.Before(versions[j].LastModified)
	})

	// If no new versions and we have a current version, return it
	// This is required by Concourse to avoid issues
	if len(versions) == 0 && request.Version.Path != "" {
		versions = append(versions, request.Version)
	}

	// Output the response
	response := models.CheckResponse(versions)
	if err := json.NewEncoder(os.Stdout).Encode(response); err != nil {
		fatal("failed to encode response: %v", err)
	}
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
