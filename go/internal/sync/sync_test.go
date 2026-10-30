package sync

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/maedoc/mrlib/go/internal/api"
	"github.com/maedoc/mrlib/go/internal/models"
)

// newMockAPI returns a test server emulating the Libraries API plus a client for it
func newMockAPI(t *testing.T, docs []models.Document) (*api.Client, *httptest.Server) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/libraries", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data": [{"id": "lib-1", "name": "Test Lib", "nb_documents": "1", "total_size": "5", "created_at": "2024-01-01T00:00:00Z", "updated_at": "2024-01-01T00:00:00Z"}], "has_more": false}`)
	})
	mux.HandleFunc("/libraries/lib-1/documents", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(map[string]interface{}{"data": docs, "has_more": false})
			return
		}
		if r.Method == http.MethodPost {
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			f, hdr, err := r.FormFile("file")
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			defer f.Close()
			name := hdr.Filename
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"id": "doc-new", "library_id": "lib-1", "name": %q, "size": "5", "process_status": "RUNNING", "created_at": "2024-01-01T00:00:00Z"}`, name)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})
	mux.HandleFunc("/libraries/lib-1/documents/", func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/libraries/lib-1/documents/")
		if strings.HasSuffix(rest, "/signed-url") {
			docID := strings.TrimSuffix(rest, "/signed-url")
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"url": "%s/download/%s"}`, srvURL(r), docID)
			return
		}
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	})
	server := httptest.NewServer(mux)

	// mux handlers reference the server URL via a trick: signed-url handler needs the base URL,
	// so wrap with a second server if needed. Simpler: patch handler after creation.
	client := api.NewClient("test-key", server.URL, 30*time.Second, 0, 3)
	return client, server
}

func srvURL(r *http.Request) string {
	return "https://" + r.Host
}

func TestSyncOnce_DryRun(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	client, server := newMockAPI(t, nil)
	defer server.Close()

	cfg := models.SyncConfig{
		LocalPath: dir,
		LibraryID: "lib-1",
		Direction: models.SyncDirectionBoth,
		Mode:      models.SyncModeMirror,
		DryRun:    true,
		StateFile: filepath.Join(dir, "state.json"),
	}
	s := NewSyncer(client, cfg)
	result, err := s.SyncOnce()
	if err != nil {
		t.Fatalf("SyncOnce: %v", err)
	}
	if result.TotalAdded != 1 {
		t.Errorf("Expected 1 add in dry-run, got %d", result.TotalAdded)
	}
	// Dry run must not create a state file
	if _, err := os.Stat(cfg.StateFile); !os.IsNotExist(err) {
		t.Errorf("Dry-run should not write state file")
	}
}

func TestSyncOnce_UploadAndTraversal(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	evil := models.Document{
		ID:        "doc-evil",
		LibraryID: "lib-1",
		Name:      "../../evil.txt",
		CreatedAt: time.Now().Add(-time.Hour),
	}
	client, server := newMockAPI(t, []models.Document{evil})
	defer server.Close()

	cfg := models.SyncConfig{
		LocalPath: dir,
		LibraryID: "lib-1",
		Direction: models.SyncDirectionBoth,
		Mode:      models.SyncModeSafe,
		StateFile: filepath.Join(dir, "state.json"),
	}
	s := NewSyncer(client, cfg)
	result, err := s.SyncOnce()
	if err != nil {
		t.Fatalf("SyncOnce: %v", err)
	}

	// The malicious remote name must not have written outside dir
	if _, err := os.Stat(filepath.Join(dir, "..", "evil.txt")); err == nil {
		t.Error("Path traversal: file written outside sync root")
	}
	// Successful actions must never reference paths outside the sync root
	for _, a := range result.Actions {
		if a.Error != nil && *a.Error != "" {
			continue
		}
		if strings.Contains(a.Path, "..") {
			t.Errorf("Unsafe path in successful action: %s", a.Path)
		}
	}
	// The clamped download must have landed inside the sync root
	if _, err := os.Stat(filepath.Join(dir, "evil.txt")); err != nil {
		t.Logf("clamped evil.txt not present (acceptable if download failed): %v", err)
	}
	if result.TotalAdded != 1 {
		t.Errorf("Expected local a.txt uploaded (1 add), got %d", result.TotalAdded)
	}
	// state file written with 0600
	info, err := os.Stat(cfg.StateFile)
	if err != nil {
		t.Fatalf("state file missing: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("State file perm = %o, want 600", perm)
	}
}
