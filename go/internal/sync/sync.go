package sync

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mistral-file-sync/internal/api"
	"mistral-file-sync/internal/models"
)

// Syncer handles two-way synchronization between local filesystem and Mistral libraries
type Syncer struct {
	Client    *api.Client
	Config    models.SyncConfig
	State     *models.SyncState
	Actions   []models.SyncAction
	StartTime time.Time
}

// NewSyncer creates a new Syncer instance
func NewSyncer(client *api.Client, config models.SyncConfig) *Syncer {
	return &Syncer{
		Client:  client,
		Config:  config,
		Actions: make([]models.SyncAction, 0),
	}
}

// LoadState loads the sync state from file
func (s *Syncer) LoadState() error {
	if s.Config.StateFile == "" {
		s.State = &models.SyncState{
			LibraryID: s.Config.LibraryID,
			LocalPath: s.Config.LocalPath,
		}
		return nil
	}

	data, err := os.ReadFile(s.Config.StateFile)
	if err != nil {
		if os.IsNotExist(err) {
			s.State = &models.SyncState{
				LibraryID: s.Config.LibraryID,
				LocalPath: s.Config.LocalPath,
			}
			return nil
		}
		return err
	}

	var state models.SyncState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	s.State = &state
	return nil
}

// SaveState saves the sync state to file
func (s *Syncer) SaveState() error {
	if s.State == nil {
		return nil
	}

	s.State.LastSync = time.Now()

	// Use default state file if not specified
	stateFile := s.Config.StateFile
	if stateFile == "" {
		stateFile = ".mistral_sync_state.json"
		s.Config.StateFile = stateFile
	}

	data, err := json.MarshalIndent(s.State, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(stateFile), 0755); err != nil {
		return err
	}

	return os.WriteFile(stateFile, data, 0644)
}

// AddAction adds a sync action to the log
func (s *Syncer) AddAction(actionType string, path string, documentID *string, err error) {
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	action := models.SyncAction{
		ActionType: actionType,
		Path:       path,
		DocumentID: documentID,
		Timestamp:  time.Now(),
		Error:      &errStr,
	}
	s.Actions = append(s.Actions, action)
}

// GetResult returns the sync result
func (s *Syncer) GetResult() models.SyncResult {
	counts := make(map[string]int)
	for _, action := range s.Actions {
		counts[action.ActionType]++
	}

	return models.SyncResult{
		Actions:      s.Actions,
		TotalAdded:   counts["add"],
		TotalUpdated: counts["update"],
		TotalDeleted: counts["delete"],
		TotalSkipped: counts["skip"],
		TotalErrors:  counts["error"],
		StartTime:    s.StartTime,
		EndTime:      time.Now(),
	}
}

// MatchesFilter checks if a file matches the sync filters
func (s *Syncer) MatchesFilter(path string) bool {
	// Check extensions
	if len(s.Config.Extensions) > 0 {
		ext := strings.ToLower(filepath.Ext(path))
		found := false
		for _, e := range s.Config.Extensions {
			if strings.ToLower(e) == ext {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check exclude patterns
	for _, pattern := range s.Config.ExcludePatterns {
		if strings.Contains(path, pattern) {
			return false
		}
	}

	// Check include patterns
	if len(s.Config.IncludePatterns) > 0 {
		for _, pattern := range s.Config.IncludePatterns {
			if strings.Contains(path, pattern) {
				return true
			}
		}
		return false
	}

	return true
}

// GetLocalFiles returns a map of local files
func (s *Syncer) GetLocalFiles() (map[string]models.FileInfo, error) {
	files := make(map[string]models.FileInfo)

	err := filepath.Walk(s.Config.LocalPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if !s.MatchesFilter(path) {
			return nil
		}

		relPath, _ := filepath.Rel(s.Config.LocalPath, path)
		files[relPath] = models.FileInfo{
			Path:      path,
			Name:      info.Name(),
			Size:      info.Size(),
			ModTime:   info.ModTime(),
			IsDir:     false,
			Extension: filepath.Ext(path),
		}

		return nil
	})

	return files, err
}

// SyncOnce performs a one-time synchronization
func (s *Syncer) SyncOnce() (models.SyncResult, error) {
	s.StartTime = time.Now()

	// Load current state
	if err := s.LoadState(); err != nil {
		return models.SyncResult{}, fmt.Errorf("failed to load state: %v", err)
	}

	// Get local and remote files
	localFiles, err := s.GetLocalFiles()
	if err != nil {
		return models.SyncResult{}, fmt.Errorf("failed to get local files: %v", err)
	}

	remoteDocs, err := s.Client.ListDocuments(s.Config.LibraryID)
	if err != nil {
		return models.SyncResult{}, fmt.Errorf("failed to get remote documents: %v", err)
	}

	// Build remote file map
	remoteFiles := make(map[string]models.Document)
	for _, doc := range remoteDocs {
		remoteFiles[doc.Name] = doc
	}

	// Perform sync based on direction and mode
	switch s.Config.Direction {
	case models.SyncDirectionUp, models.SyncDirectionBoth:
		s.syncUp(localFiles, remoteFiles)
	}

	if s.Config.Direction == models.SyncDirectionDown || s.Config.Direction == models.SyncDirectionBoth {
		s.syncDown(localFiles, remoteFiles)
	}

	// Save state
	if err := s.SaveState(); err != nil {
		fmt.Printf("Warning: failed to save state: %v\n", err)
	}

	return s.GetResult(), nil
}

// syncUp synchronizes local files to remote
func (s *Syncer) syncUp(localFiles map[string]models.FileInfo, remoteFiles map[string]models.Document) {
	for path, localFile := range localFiles {
		if remoteDoc, exists := remoteFiles[localFile.Name]; exists {
			// File exists on both sides - check if needs update
			if s.shouldUpdate(localFile, remoteDoc) {
				// Upload updated file
				if _, err := s.Client.UploadDocument(s.Config.LibraryID, localFile.Path); err != nil {
					s.AddAction("error", path, nil, err)
				} else {
					s.AddAction("update", path, &remoteDoc.ID, nil)
				}
			} else {
				s.AddAction("skip", path, &remoteDoc.ID, nil)
			}
		} else {
			// File only exists locally - upload it
			if doc, err := s.Client.UploadDocument(s.Config.LibraryID, localFile.Path); err != nil {
				s.AddAction("error", path, nil, err)
			} else {
				s.AddAction("add", path, &doc.ID, nil)
			}
		}
	}

	// Handle deletions based on mode
	if s.Config.Mode == models.SyncModeMirror || s.Config.Mode == models.SyncModeSafe {
		for name, remoteDoc := range remoteFiles {
			if _, exists := localFiles[name]; !exists {
				// File only exists remotely
				if s.Config.Mode == models.SyncModeMirror {
					if err := s.Client.DeleteDocument(s.Config.LibraryID, remoteDoc.ID); err != nil {
						s.AddAction("error", name, &remoteDoc.ID, err)
					} else {
						s.AddAction("delete", name, &remoteDoc.ID, nil)
					}
				} else {
					s.AddAction("skip", name, &remoteDoc.ID, nil)
				}
			}
		}
	}
}

// syncDown synchronizes remote files to local
func (s *Syncer) syncDown(localFiles map[string]models.FileInfo, remoteFiles map[string]models.Document) {
	for name, remoteDoc := range remoteFiles {
		if localFile, exists := localFiles[name]; exists {
			// File exists on both sides - check if needs update
			if s.shouldUpdate(localFile, remoteDoc) {
				// Download updated file
				outputPath := filepath.Join(s.Config.LocalPath, name)
				if err := s.Client.DownloadDocument(s.Config.LibraryID, remoteDoc.ID, outputPath); err != nil {
					s.AddAction("error", name, &remoteDoc.ID, err)
				} else {
					s.AddAction("update", name, &remoteDoc.ID, nil)
				}
			} else {
				s.AddAction("skip", name, &remoteDoc.ID, nil)
			}
		} else {
			// File only exists remotely - download it
			outputPath := filepath.Join(s.Config.LocalPath, name)
			if err := s.Client.DownloadDocument(s.Config.LibraryID, remoteDoc.ID, outputPath); err != nil {
				s.AddAction("error", name, &remoteDoc.ID, err)
			} else {
				s.AddAction("add", name, &remoteDoc.ID, nil)
			}
		}
	}

	// Handle deletions based on mode
	if s.Config.Mode == models.SyncModeMirror || s.Config.Mode == models.SyncModeSafe {
		for path := range localFiles {
			if _, exists := remoteFiles[path]; !exists {
				// File only exists locally
				if s.Config.Mode == models.SyncModeMirror {
					if err := os.Remove(filepath.Join(s.Config.LocalPath, path)); err != nil {
						s.AddAction("error", path, nil, err)
					} else {
						s.AddAction("delete", path, nil, nil)
					}
				} else {
					s.AddAction("skip", path, nil, nil)
				}
			}
		}
	}
}

// shouldUpdate determines if a file should be updated based on timestamps
func (s *Syncer) shouldUpdate(localFile models.FileInfo, remoteDoc models.Document) bool {
	// For now, always return true if Force is set
	if s.Config.Force {
		return true
	}

	// Compare timestamps
	return localFile.ModTime.After(remoteDoc.CreatedAt)
}

// GetStatus returns the current sync status
func (s *Syncer) GetStatus() (*models.SyncState, error) {
	if err := s.LoadState(); err != nil {
		return nil, err
	}
	return s.State, nil
}

// CompareLocalAndRemote compares local files with remote documents and returns pending changes
func (s *Syncer) CompareLocalAndRemote() (*SyncComparison, error) {
	localFiles, err := s.GetLocalFiles()
	if err != nil {
		return nil, err
	}

	remoteDocs, err := s.Client.ListDocuments(s.Config.LibraryID)
	if err != nil {
		return nil, err
	}

	comparison := &SyncComparison{
		LocalOnly:  make([]string, 0),
		RemoteOnly: make([]string, 0),
		Modified:   make([]string, 0),
		Same:       make([]string, 0),
	}

	// Build remote file map
	remoteMap := make(map[string]models.Document)
	for _, doc := range remoteDocs {
		remoteMap[doc.Name] = doc
	}

	// Compare
	for path, localFile := range localFiles {
		if remoteDoc, exists := remoteMap[localFile.Name]; exists {
			if s.shouldUpdate(localFile, remoteDoc) {
				comparison.Modified = append(comparison.Modified, path)
			} else {
				comparison.Same = append(comparison.Same, path)
			}
		} else {
			comparison.LocalOnly = append(comparison.LocalOnly, path)
		}
	}

	for name := range remoteMap {
		if _, exists := localFiles[name]; !exists {
			comparison.RemoteOnly = append(comparison.RemoteOnly, name)
		}
	}

	return comparison, nil
}

// SyncComparison represents the comparison between local and remote files
type SyncComparison struct {
	LocalOnly  []string `json:"local_only"`
	RemoteOnly []string `json:"remote_only"`
	Modified   []string `json:"modified"`
	Same       []string `json:"same"`
}
