package api

import (
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

		// Check authorization header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key" {
			t.Errorf("Expected Bearer test-key, got %s", auth)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": [{"id": "lib-1", "name": "Test Lib", "nb_documents": 0, "total_size": 0, "created_at": "2024-01-01T00:00:00Z", "updated_at": "2024-01-01T00:00:00Z"}]}`))
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
		w.Write([]byte(`{"id": "doc-1", "name": "test-upload.txt", "size": 12, "hash": "abc123", "process_status": "done", "created_at": "2024-01-01T00:00:00Z"}`))
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
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": []}`))
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
