package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestClient_ListLibraries(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/libraries" {
			t.Errorf("Expected /libraries, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("page_size") == "" {
			t.Errorf("Expected page_size query parameter, got %s", r.URL.RawQuery)
		}

		// Check authorization header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key" {
			t.Errorf("Expected Bearer test-key, got %s", auth)
		}

		w.WriteHeader(http.StatusOK)
		// nb_documents and total_size returned as strings (current API behavior)
		w.Write([]byte(`{"data": [{"id": "lib-1", "name": "Test Lib", "nb_documents": "3", "total_size": "1024", "created_at": "2024-01-01T00:00:00Z", "updated_at": "2024-01-01T00:00:00Z"}], "has_more": false}`))
	}))
	defer server.Close()

	client := NewClient("test-key", server.URL, 30*time.Second, 0, 3)
	libs, err := client.ListLibraries()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(libs) != 1 {
		t.Fatalf("Expected 1 library, got %d", len(libs))
	}

	if libs[0].ID != "lib-1" {
		t.Errorf("Expected lib-1, got %s", libs[0].ID)
	}

	if libs[0].NbDocuments != 3 {
		t.Errorf("Expected NbDocuments 3 (string-encoded), got %d", libs[0].NbDocuments)
	}

	if libs[0].TotalSize != 1024 {
		t.Errorf("Expected TotalSize 1024 (string-encoded), got %d", libs[0].TotalSize)
	}
}

func TestClient_ListLibraries_Pagination(t *testing.T) {
	page := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/libraries" {
			t.Errorf("Expected /libraries, got %s", r.URL.Path)
		}
		page++
		w.WriteHeader(http.StatusOK)
		if page == 1 {
			fmt.Fprint(w, `{"data": [{"id": "lib-1", "name": "A", "created_at": "2024-01-01T00:00:00Z", "updated_at": "2024-01-01T00:00:00Z"}], "has_more": true, "total": 2}`)
		} else {
			fmt.Fprint(w, `{"data": [{"id": "lib-2", "name": "B", "created_at": "2024-01-01T00:00:00Z", "updated_at": "2024-01-01T00:00:00Z"}], "has_more": false, "total": 2}`)
		}
	}))
	defer server.Close()

	client := NewClient("test-key", server.URL, 30*time.Second, 0, 3)
	libs, err := client.ListLibraries()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(libs) != 2 {
		t.Fatalf("Expected 2 libraries across pages, got %d", len(libs))
	}
	if page != 2 {
		t.Errorf("Expected 2 pages fetched, got %d", page)
	}
}

func TestClient_CreateLibrary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/libraries" {
			t.Errorf("Expected /libraries, got %s", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": "new-lib", "name": "New Library", "nb_documents": 0, "total_size": 0, "created_at": "2024-01-01T00:00:00Z", "updated_at": "2024-01-01T00:00:00Z"}`))
	}))
	defer server.Close()

	client := NewClient("test-key", server.URL, 30*time.Second, 0, 3)
	lib, err := client.CreateLibrary("New Library", "")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if lib.ID != "new-lib" {
		t.Errorf("Expected new-lib, got %s", lib.ID)
	}
}

func TestClient_UploadDocument(t *testing.T) {
	// Create a temp file
	tmpFile, err := os.CreateTemp("", "test-upload-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString("test content"); err != nil {
		t.Fatal(err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}

		// Check content type is multipart
		contentType := r.Header.Get("Content-Type")
		if !strings.Contains(contentType, "multipart/form-data") {
			t.Errorf("Expected multipart/form-data, got %s", contentType)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": "doc-1", "name": "test-upload.txt", "size": "12", "hash": "abc123", "process_status": "RUNNING", "processing_status": "RUNNING", "created_at": "2024-01-01T00:00:00Z"}`))
	}))
	defer server.Close()

	client := NewClient("test-key", server.URL, 30*time.Second, 0, 3)
	doc, err := client.UploadDocument("lib-1", tmpFile.Name())

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if doc.ID != "doc-1" {
		t.Errorf("Expected doc-1, got %s", doc.ID)
	}

	if doc.Size != 12 {
		t.Errorf("Expected size 12 (string-encoded), got %d", doc.Size)
	}
}

func TestClient_AuthenticationError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message": "Invalid API key", "code": "auth_error"}`))
	}))
	defer server.Close()

	client := NewClient("invalid-key", server.URL, 30*time.Second, 0, 3)
	_, err := client.ListLibraries()

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("Expected authentication error, got: %v", err)
	}
}

func TestClient_RateLimitRetry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": [], "has_more": false}`))
	}))
	defer server.Close()

	client := NewClient("test-key", server.URL, 30*time.Second, 0, 3)
	_, err := client.ListLibraries()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestClient_GetSignedURL_JSONObject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/libraries/lib-1/documents/doc-1/signed-url" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"url": "https://storage.example.com/signed/file.pdf?sig=abc"}`))
	}))
	defer server.Close()

	client := NewClient("test-key", server.URL, 30*time.Second, 0, 3)
	u, err := client.GetSignedURL("lib-1", "doc-1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if u != "https://storage.example.com/signed/file.pdf?sig=abc" {
		t.Errorf("Unexpected signed URL: %s", u)
	}
}

func TestClient_GetSignedURL_BareString(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`"https://storage.example.com/signed/file.pdf?sig=xyz"`))
	}))
	defer server.Close()

	client := NewClient("test-key", server.URL, 30*time.Second, 0, 3)
	u, err := client.GetSignedURL("lib-1", "doc-1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if u != "https://storage.example.com/signed/file.pdf?sig=xyz" {
		t.Errorf("Unexpected signed URL: %s", u)
	}
}

func TestClient_DownloadDocument_RejectsNonHTTPS(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"url": "http://evil.example.com/file.pdf"}`))
	}))
	defer server.Close()

	client := NewClient("test-key", server.URL, 30*time.Second, 0, 3)
	err := client.DownloadDocument("lib-1", "doc-1", t.TempDir()+"/out.pdf")
	if err == nil || !strings.Contains(err.Error(), "non-HTTPS") {
		t.Errorf("Expected non-HTTPS refusal, got: %v", err)
	}
}

func TestSafeJoin(t *testing.T) {
	base := t.TempDir()

	ok, err := SafeJoin(base, "a/b.pdf")
	if err != nil || ok != base+"/a/b.pdf" {
		t.Errorf("Expected joined path, got %q err=%v", ok, err)
	}

	// Traversal input is clamped inside base, never escaping it
	p, err := SafeJoin(base, "../../etc/passwd")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if p != base+"/etc/passwd" {
		t.Errorf("Expected clamped path inside base, got %q", p)
	}

	if _, err := SafeJoin(base, "/etc/passwd"); err != nil {
		t.Errorf("Absolute path should be treated as relative, got err=%v", err)
	}
}
