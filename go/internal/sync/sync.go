package sync

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/maedoc/mrlib/go/internal/api"
	"github.com/maedoc/mrlib/go/internal/models"
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
		stateFile = ".mrlib_sync_state.json"
		s.Config.StateFile = stateFile
	}

	data, err := json.MarshalIndent(s.State, "", "  ")
	if err != nil {
		return err
	}

	if dir := filepath.Dir(stateFile); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return os.WriteFile(stateFile, data, 0600)
}

// AddAction adds a sync action to the log
func (s *Syncer) AddAction(actionType string, path string, documentID *string, err error) {
	var errStr *string
	if err != nil {
		msg := err.Error()
		errStr = &msg
	}
	action := models.SyncAction{
		ActionType: actionType,
		Path:       path,
		DocumentID: documentID,
		Timestamp:  time.Now(),
		Error:      errStr,
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
			if strings.HasPrefix(e, ".") {
				e = e[1:]
			}
			if strings.ToLower(e) == strings.TrimPrefix(ext, ".") {
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
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return false
		}
		if strings.Contains(path, pattern) {
			return false
		}
	}

	// Check include patterns
	if len(s.Config.IncludePatterns) > 0 {
		for _, pattern := range s.Config.IncludePatterns {
			if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
				return true
			}
			if strings.Contains(path, pattern) {
				return true
			}
		}
		return false
	}

	return true
}

// GetLocalFiles returns a map of local files keyed by path relative to the sync root
func (s *Syncer) GetLocalFiles() (map[string]models.FileInfo, error) {
	files := make(map[string]models.FileInfo)

	root, err := filepath.Abs(s.Config.LocalPath)
	if err != nil {
		return nil, err
	}

	err = filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if !s.MatchesFilter(path) {
			return nil
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)
		files[relPath] = models.FileInfo{
			Path:      path,
			Name:      relPath,
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

	// Build remote file map keyed by relative path (documents may use
	// subdirectory names such as "a/b.pdf")
	remoteFiles := make(map[string]models.Document)
	for _, doc := range remoteDocs {
		remoteFiles[filepath.ToSlash(doc.Name)] = doc
	}

	// Perform sync based on direction and mode
	switch s.Config.Direction {
	case models.SyncDirectionUp, models.SyncDirectionBoth:
		s.syncUp(localFiles, remoteFiles)
	}

	if s.Config.Direction == models.SyncDirectionDown || s.Config.Direction == models.SyncDirectionBoth {
		s.syncDown(localFiles, remoteFiles)
	}

	// Save state (skip in dry-run mode)
	if !s.Config.DryRun {
		if err := s.SaveState(); err != nil {
			fmt.Printf("Warning: failed to save state: %v\n", err)
		}
	}

	return s.GetResult(), nil
}

// syncUp synchronizes local files to remote
func (s *Syncer) syncUp(localFiles map[string]models.FileInfo, remoteFiles map[string]models.Document) {
	for path, localFile := range localFiles {
		if remoteDoc, exists := remoteFiles[path]; exists {
			// File exists on both sides - check if needs update
			if s.shouldUpdate(localFile, remoteDoc) {
				if s.Config.DryRun {
					s.AddAction("update", path, &remoteDoc.ID, nil)
					continue
				}
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
			if s.Config.DryRun {
				s.AddAction("add", path, nil, nil)
				continue
			}
			if doc, err := s.Client.UploadDocument(s.Config.LibraryID, localFile.Path); err != nil {
				s.AddAction("error", path, nil, err)
			} else {
				s.AddAction("add", path, &doc.ID, nil)
			}
		}
	}

	// Handle deletions based on mode
	if s.Config.Mode == models.SyncModeMirror {
		for name, remoteDoc := range remoteFiles {
			if _, exists := localFiles[name]; !exists {
				// File only exists remotely
				if s.Config.DryRun {
					s.AddAction("delete", name, &remoteDoc.ID, nil)
					continue
				}
				if err := s.Client.DeleteDocument(s.Config.LibraryID, remoteDoc.ID); err != nil {
					s.AddAction("error", name, &remoteDoc.ID, err)
				} else {
					s.AddAction("delete", name, &remoteDoc.ID, nil)
				}
			}
		}
	}
}

// syncDown synchronizes remote files to local
func (s *Syncer) syncDown(localFiles map[string]models.FileInfo, remoteFiles map[string]models.Document) {
	for name, remoteDoc := range remoteFiles {
		outputPath, err := api.SafeJoin(s.Config.LocalPath, name)
		if err != nil {
			s.AddAction("error", name, &remoteDoc.ID, err)
			continue
		}
		if localFile, exists := localFiles[name]; exists {
			// File exists on both sides - check if needs update
			if s.shouldUpdate(localFile, remoteDoc) {
				if s.Config.DryRun {
					s.AddAction("update", name, &remoteDoc.ID, nil)
					continue
				}
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
			if s.Config.DryRun {
				s.AddAction("add", name, &remoteDoc.ID, nil)
				continue
			}
			if err := s.Client.DownloadDocument(s.Config.LibraryID, remoteDoc.ID, outputPath); err != nil {
				s.AddAction("error", name, &remoteDoc.ID, err)
			} else {
				s.AddAction("add", name, &remoteDoc.ID, nil)
			}
		}
	}

	// Handle deletions based on mode
	if s.Config.Mode == models.SyncModeMirror {
		for path := range localFiles {
			if _, exists := remoteFiles[path]; !exists {
				// File only exists locally
				if s.Config.DryRun {
					s.AddAction("delete", path, nil, nil)
					continue
				}
				if err := os.Remove(filepath.Join(s.Config.LocalPath, path)); err != nil {
					s.AddAction("error", path, nil, err)
				} else {
					s.AddAction("delete", path, nil, nil)
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

	remoteTime := remoteDoc.UpdatedAt
	if remoteTime.IsZero() {
		remoteTime = remoteDoc.CreatedAt
	}

	// Compare timestamps
	return localFile.ModTime.After(remoteTime)
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
		remoteMap[filepath.ToSlash(doc.Name)] = doc
	}

	// Compare
	for path, localFile := range localFiles {
		if remoteDoc, exists := remoteMap[path]; exists {
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
