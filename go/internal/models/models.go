// Package models contains data models for Mistral AI Libraries and Documents API
package models

import (
	"encoding/json"
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
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description *string    `json:"description,omitempty"`
	NbDocuments int        `json:"nb_documents"`
	TotalSize   int        `json:"total_size"`
	ChunkSize   *int       `json:"chunk_size,omitempty"`
	OwnerID     *string    `json:"owner_id,omitempty"`
	OwnerType   *string    `json:"owner_type,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Document represents a Mistral AI Document
type Document struct {
	ID            string         `json:"id"`
	LibraryID     string         `json:"library_id"`
	Name          string         `json:"name"`
	Extension     *string        `json:"extension,omitempty"`
	MimeType      *string        `json:"mime_type,omitempty"`
	Size          int            `json:"size"`
	Hash          string         `json:"hash"`
	ProcessStatus DocumentStatus `json:"process_status"`
	CreatedAt     time.Time      `json:"created_at"`
}

// SyncConfig represents configuration for two-way sync
type SyncConfig struct {
	LocalPath       string        `yaml:"local_path" json:"local_path"`
	LibraryID       string        `yaml:"library_id" json:"library_id"`
	Direction       SyncDirection `yaml:"direction" json:"direction"`
	Mode           SyncMode      `yaml:"mode" json:"mode"`
	BatchSize      int           `yaml:"batch_size" json:"batch_size"`
	MaxWorkers     int           `yaml:"max_workers" json:"max_workers"`
	DryRun         bool          `yaml:"dry_run" json:"dry_run"`
	Force          bool          `yaml:"force" json:"force"`
	StateFile      string        `yaml:"state_file" json:"state_file"`
	Extensions     []string      `yaml:"extensions" json:"extensions"`
	ExcludePatterns []string    `yaml:"exclude_patterns" json:"exclude_patterns"`
	IncludePatterns []string    `yaml:"include_patterns" json:"include_patterns"`
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
	StartTime    time.Time   `json:"start_time"`
	EndTime      time.Time   `json:"end_time"`
}

// APIError represents an API error response
type APIError struct {
	Message    string `json:"message"`
	Code       string `json:"code,omitempty"`
	StatusCode int    `json:"status_code,omitempty"`
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
	Data   []Library `json:"data"`
	HasMore bool      `json:"has_more"`
	Total  int       `json:"total"`
}

// ListDocumentsResponse represents the API response for listing documents
type ListDocumentsResponse struct {
	Data   []Document `json:"data"`
	HasMore bool       `json:"has_more"`
	Total  int        `json:"total"`
}

// UnmarshalJSON handles custom unmarshaling for Library
func (l *Library) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Extract fields
	if id, ok := raw["id"]; ok {
		if err := json.Unmarshal(id, &l.ID); err != nil {
			return err
		}
	}
	if name, ok := raw["name"]; ok {
		if err := json.Unmarshal(name, &l.Name); err != nil {
			return err
		}
	}
	if desc, ok := raw["description"]; ok {
		var d string
		if err := json.Unmarshal(desc, &d); err == nil {
			l.Description = &d
		}
	}
	if nbDocs, ok := raw["nb_documents"]; ok {
		if err := json.Unmarshal(nbDocs, &l.NbDocuments); err != nil {
			return err
		}
	}
	if totalSize, ok := raw["total_size"]; ok {
		if err := json.Unmarshal(totalSize, &l.TotalSize); err != nil {
			return err
		}
	}
	if createdAt, ok := raw["created_at"]; ok {
		if err := json.Unmarshal(createdAt, &l.CreatedAt); err != nil {
			return err
		}
	}
	if updatedAt, ok := raw["updated_at"]; ok {
		if err := json.Unmarshal(updatedAt, &l.UpdatedAt); err != nil {
			return err
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

	// Extract fields
	if id, ok := raw["id"]; ok {
		if err := json.Unmarshal(id, &d.ID); err != nil {
			return err
		}
	}
	if libId, ok := raw["library_id"]; ok {
		if err := json.Unmarshal(libId, &d.LibraryID); err != nil {
			return err
		}
	}
	if name, ok := raw["name"]; ok {
		if err := json.Unmarshal(name, &d.Name); err != nil {
			return err
		}
	}
	if size, ok := raw["size"]; ok {
		if err := json.Unmarshal(size, &d.Size); err != nil {
			return err
		}
	}
	if hash, ok := raw["hash"]; ok {
		if err := json.Unmarshal(hash, &d.Hash); err != nil {
			return err
		}
	}
	if createdAt, ok := raw["created_at"]; ok {
		if err := json.Unmarshal(createdAt, &d.CreatedAt); err != nil {
			return err
		}
	}

	return nil
}
