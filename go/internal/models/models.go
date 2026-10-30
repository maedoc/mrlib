// Package models contains data models for Mistral AI Libraries and Documents API
package models

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// LibraryOwnerType represents the type of library owner
type LibraryOwnerType string

const (
	// OwnerTypeUser represents a user-owned library
	OwnerTypeUser LibraryOwnerType = "user"
	// OwnerTypeWorkspace represents a workspace-owned library
	OwnerTypeWorkspace LibraryOwnerType = "workspace"
)

// DocumentStatus represents the processing status of a document
type DocumentStatus string

const (
	// DocumentStatusPending means the document is waiting to be processed
	DocumentStatusPending DocumentStatus = "pending"
	// DocumentStatusInProgress means the document is being processed
	DocumentStatusInProgress DocumentStatus = "in_progress"
	// DocumentStatusDone means the document has been processed
	DocumentStatusDone DocumentStatus = "done"
	// DocumentStatusError means the document processing failed
	DocumentStatusError DocumentStatus = "error"
)

// SyncDirection represents the direction of synchronization
type SyncDirection string

const (
	// SyncDirectionUp means sync from local to remote only
	SyncDirectionUp SyncDirection = "up"
	// SyncDirectionDown means sync from remote to local only
	SyncDirectionDown SyncDirection = "down"
	// SyncDirectionBoth means bidirectional sync
	SyncDirectionBoth SyncDirection = "both"
)

// SyncMode represents the conflict resolution strategy
type SyncMode string

const (
	// SyncModeMirror means exact two-way mirror (deletes files that don't exist on the other side)
	SyncModeMirror SyncMode = "mirror"
	// SyncModeAdditive means only adds, never deletes
	SyncModeAdditive SyncMode = "additive"
	// SyncModeSafe means adds and updates, but doesn't delete
	SyncModeSafe SyncMode = "safe"
)

// Library represents a Mistral AI Library
type Library struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	NbDocuments int       `json:"nb_documents"`
	TotalSize   int       `json:"total_size"`
	ChunkSize   *int      `json:"chunk_size,omitempty"`
	OwnerID     *string   `json:"owner_id,omitempty"`
	OwnerType   *string   `json:"owner_type,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Document represents a Mistral AI Document
type Document struct {
	ID               string    `json:"id"`
	LibraryID        string    `json:"library_id"`
	Name             string    `json:"name"`
	Extension        *string   `json:"extension,omitempty"`
	MimeType         *string   `json:"mime_type,omitempty"`
	Size             int64     `json:"size"`
	Hash             *string   `json:"hash,omitempty"`
	ProcessStatus    string    `json:"process_status,omitempty"`
	ProcessingStatus string    `json:"processing_status,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at,omitempty"`
}

// flexInt unmarshals a JSON number or a numeric string into an int64.
// The Libraries API has returned both encodings for fields such as
// nb_documents, total_size and size.
type flexInt int64

// UnmarshalJSON accepts either a JSON number or a quoted number
func (f *flexInt) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*f = flexInt(n)
	return nil
}

// flexTime unmarshals RFC3339 timestamps, tolerating empty/null values
type flexTime time.Time

// UnmarshalJSON accepts an RFC3339 string or null
func (f *flexTime) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	if s == "" || s == "null" {
		*f = flexTime(time.Time{})
		return nil
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return err
	}
	*f = flexTime(t)
	return nil
}

// SyncConfig represents configuration for two-way sync
type SyncConfig struct {
	LocalPath       string        `yaml:"local_path" json:"local_path"`
	LibraryID       string        `yaml:"library_id" json:"library_id"`
	Direction       SyncDirection `yaml:"direction" json:"direction"`
	Mode            SyncMode      `yaml:"mode" json:"mode"`
	BatchSize       int           `yaml:"batch_size" json:"batch_size"`
	MaxWorkers      int           `yaml:"max_workers" json:"max_workers"`
	DryRun          bool          `yaml:"dry_run" json:"dry_run"`
	Force           bool          `yaml:"force" json:"force"`
	StateFile       string        `yaml:"state_file" json:"state_file"`
	Extensions      []string      `yaml:"extensions" json:"extensions"`
	ExcludePatterns []string      `yaml:"exclude_patterns" json:"exclude_patterns"`
	IncludePatterns []string      `yaml:"include_patterns" json:"include_patterns"`
}

// SyncState represents the state of a sync operation
type SyncState struct {
	LibraryID string    `json:"library_id"`
	LocalPath string    `json:"local_path"`
	LastSync  time.Time `json:"last_sync"`
	SyncHash  string    `json:"sync_hash,omitempty"`
}

// SyncAction represents a single sync action
type SyncAction struct {
	ActionType string    `json:"action_type"` // add, update, delete, skip
	Path       string    `json:"path"`
	DocumentID *string   `json:"document_id,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
	Error      *string   `json:"error,omitempty"`
}

// SyncResult represents the result of a sync operation
type SyncResult struct {
	Actions      []SyncAction `json:"actions"`
	TotalAdded   int          `json:"total_added"`
	TotalUpdated int          `json:"total_updated"`
	TotalDeleted int          `json:"total_deleted"`
	TotalSkipped int          `json:"total_skipped"`
	TotalErrors  int          `json:"total_errors"`
	StartTime    time.Time    `json:"start_time"`
	EndTime      time.Time    `json:"end_time"`
}

// APIError represents an API error response
type APIError struct {
	Message    string                 `json:"message"`
	Code       string                 `json:"code,omitempty"`
	StatusCode int                    `json:"status_code,omitempty"`
	Details    map[string]interface{} `json:"details,omitempty"`
}

// FileInfo represents local file information
type FileInfo struct {
	Path      string
	Name      string
	Size      int64
	ModTime   time.Time
	IsDir     bool
	Extension string
}

// ListLibrariesResponse represents the API response for listing libraries
type ListLibrariesResponse struct {
	Data    []Library `json:"data"`
	HasMore bool      `json:"has_more"`
	Total   int       `json:"total"`
}

// ListDocumentsResponse represents the API response for listing documents
type ListDocumentsResponse struct {
	Data    []Document `json:"data"`
	HasMore bool       `json:"has_more"`
	Total   int        `json:"total"`
}

// UnmarshalJSON handles custom unmarshaling for Library
func (l *Library) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	getString := func(key string) (string, error) {
		v, ok := raw[key]
		if !ok {
			return "", nil
		}
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return "", err
		}
		return s, nil
	}
	getOptString := func(key string) (*string, error) {
		v, ok := raw[key]
		if !ok || string(v) == "null" {
			return nil, nil
		}
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return nil, nil
		}
		return &s, nil
	}
	getInt := func(key string) (int64, error) {
		v, ok := raw[key]
		if !ok || string(v) == "null" {
			return 0, nil
		}
		var f flexInt
		if err := json.Unmarshal(v, &f); err != nil {
			return 0, err
		}
		return int64(f), nil
	}
	getTime := func(key string) (time.Time, error) {
		v, ok := raw[key]
		if !ok || string(v) == "null" {
			return time.Time{}, nil
		}
		var f flexTime
		if err := json.Unmarshal(v, &f); err != nil {
			return time.Time{}, err
		}
		return time.Time(f), nil
	}

	var err error
	if l.ID, err = getString("id"); err != nil {
		return err
	}
	if l.Name, err = getString("name"); err != nil {
		return err
	}
	if l.Description, err = getOptString("description"); err != nil {
		return err
	}
	var nb int64
	if nb, err = getInt("nb_documents"); err != nil {
		return err
	}
	l.NbDocuments = int(nb)
	var size int64
	if size, err = getInt("total_size"); err != nil {
		return err
	}
	l.TotalSize = int(size)
	if l.CreatedAt, err = getTime("created_at"); err != nil {
		return err
	}
	if l.UpdatedAt, err = getTime("updated_at"); err != nil {
		return err
	}
	if l.OwnerID, err = getOptString("owner_id"); err != nil {
		return err
	}
	if l.OwnerType, err = getOptString("owner_type"); err != nil {
		return err
	}
	if v, ok := raw["chunk_size"]; ok && string(v) != "null" {
		var f flexInt
		if err := json.Unmarshal(v, &f); err == nil {
			n := int(int64(f))
			l.ChunkSize = &n
		}
	}

	return nil
}

// UnmarshalJSON handles custom unmarshaling for Document
func (d *Document) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	getString := func(key string) (string, error) {
		v, ok := raw[key]
		if !ok {
			return "", nil
		}
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return "", err
		}
		return s, nil
	}
	getOptString := func(key string) (*string, error) {
		v, ok := raw[key]
		if !ok || string(v) == "null" {
			return nil, nil
		}
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return nil, nil
		}
		return &s, nil
	}
	getInt := func(key string) (int64, error) {
		v, ok := raw[key]
		if !ok || string(v) == "null" {
			return 0, nil
		}
		var f flexInt
		if err := json.Unmarshal(v, &f); err != nil {
			return 0, err
		}
		return int64(f), nil
	}
	getTime := func(key string) (time.Time, error) {
		v, ok := raw[key]
		if !ok || string(v) == "null" {
			return time.Time{}, nil
		}
		var f flexTime
		if err := json.Unmarshal(v, &f); err != nil {
			return time.Time{}, err
		}
		return time.Time(f), nil
	}

	var err error
	if d.ID, err = getString("id"); err != nil {
		return err
	}
	if d.LibraryID, err = getString("library_id"); err != nil {
		return err
	}
	if d.Name, err = getString("name"); err != nil {
		return err
	}
	var size int64
	if size, err = getInt("size"); err != nil {
		return err
	}
	d.Size = size
	if d.Hash, err = getOptString("hash"); err != nil {
		return err
	}
	if d.Extension, err = getOptString("extension"); err != nil {
		return err
	}
	if d.MimeType, err = getOptString("mime_type"); err != nil {
		return err
	}
	if d.ProcessStatus, err = getString("process_status"); err != nil {
		return err
	}
	if d.ProcessingStatus, err = getString("processing_status"); err != nil {
		return err
	}
	if d.CreatedAt, err = getTime("created_at"); err != nil {
		return err
	}
	if d.UpdatedAt, err = getTime("updated_at"); err != nil {
		return err
	}

	return nil
}
