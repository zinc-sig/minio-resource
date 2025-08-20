package models

import (
	"encoding/json"
	"time"
)

// Source represents the configuration for connecting to Minio
type Source struct {
	Endpoint            string `json:"endpoint"`
	AccessKey           string `json:"access_key"`
	SecretKey           string `json:"secret_key"`
	Bucket              string `json:"bucket"`
	PathPrefix          string `json:"path_prefix,omitempty"`
	UseSSL              *bool  `json:"use_ssl,omitempty"`
	Region              string `json:"region,omitempty"`
	SkipSSLVerification bool   `json:"skip_ssl_verification,omitempty"`
}

// Version represents a specific version of the resource
type Version struct {
	Path         string    `json:"path"`
	ETag         string    `json:"etag"`
	LastModified time.Time `json:"last_modified"`
}

// CheckRequest is the input for the check script
type CheckRequest struct {
	Source  Source  `json:"source"`
	Version Version `json:"version,omitempty"`
}

// CheckResponse is the output from the check script
type CheckResponse []Version

// InRequest is the input for the in script
type InRequest struct {
	Source  Source         `json:"source"`
	Version Version        `json:"version"`
	Params  map[string]any `json:"params,omitempty"`
}

// InResponse is the output from the in script
type InResponse struct {
	Version  Version    `json:"version"`
	Metadata []Metadata `json:"metadata,omitempty"`
}

// OutRequest is the input for the out script
type OutRequest struct {
	Source Source         `json:"source"`
	Params map[string]any `json:"params,omitempty"`
}

// OutResponse is the output from the out script
type OutResponse struct {
	Version  Version    `json:"version"`
	Metadata []Metadata `json:"metadata,omitempty"`
}

// Metadata represents key-value pairs returned by in/out scripts
type Metadata struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// UseSSLValue returns the value of UseSSL, defaulting to true if not set
func (s *Source) UseSSLValue() bool {
	if s.UseSSL == nil {
		return true
	}
	return *s.UseSSL
}

// UnmarshalJSON implements custom unmarshaling for Version to handle time parsing
func (v *Version) UnmarshalJSON(data []byte) error {
	type Alias Version
	aux := &struct {
		LastModified string `json:"last_modified"`
		*Alias
	}{
		Alias: (*Alias)(v),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if aux.LastModified != "" {
		t, err := time.Parse(time.RFC3339, aux.LastModified)
		if err != nil {
			// Try parsing as Unix timestamp string
			var timestamp int64
			if err := json.Unmarshal([]byte(aux.LastModified), &timestamp); err == nil {
				v.LastModified = time.Unix(timestamp, 0)
			} else {
				return err
			}
		} else {
			v.LastModified = t
		}
	}

	return nil
}

// MarshalJSON implements custom marshaling for Version to format time correctly
func (v Version) MarshalJSON() ([]byte, error) {
	type Alias Version
	return json.Marshal(&struct {
		LastModified string `json:"last_modified"`
		*Alias
	}{
		LastModified: v.LastModified.Format(time.RFC3339),
		Alias:        (*Alias)(&v),
	})
}
