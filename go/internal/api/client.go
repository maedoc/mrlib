package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mistral-file-sync/internal/models"
)

// Client represents the Mistral AI API client
type Client struct {
	APIKey          string
	BaseURL         string
	Timeout         time.Duration
	RateLimitDelay  time.Duration
	MaxRetries      int
	HTTPClient      *http.Client
	// Cache for library name -> ID resolution
	libraryCache    map[string]string // name -> id
	libraryCacheTime time.Time        // last cache update
	cacheTTL        time.Duration     // cache time-to-live
}

// NewClient creates a new Mistral API client
func NewClient(apiKey string, baseURL string, timeout time.Duration, rateLimitDelay time.Duration, maxRetries int) *Client {
	return &Client{
		APIKey:          apiKey,
		BaseURL:         baseURL,
		Timeout:         timeout,
		RateLimitDelay:  rateLimitDelay,
		MaxRetries:      maxRetries,
		HTTPClient:      &http.Client{Timeout: timeout},
		libraryCache:    make(map[string]string),
		cacheTTL:        5 * time.Minute,
	}
}

// ResolveLibraryID resolves a library name or ID to a library ID
// If the input is already a valid UUID format, it returns it as-is
// Otherwise, it looks up the name in the cache or fetches from the API
func (c *Client) ResolveLibraryID(nameOrID string) (string, error) {
	// Check if it's already a UUID (simple heuristic: contains hyphens and is 36 chars)
	if len(nameOrID) == 36 {
		// Quick UUID check - if it has hyphens in the right places
		if nameOrID[8] == '-' && nameOrID[13] == '-' && nameOrID[18] == '-' && nameOrID[23] == '-' {
			return nameOrID, nil
		}
	}

	// Check cache first
	if id, ok := c.libraryCache[nameOrID]; ok {
		return id, nil
	}

	// Cache is stale, refresh it
	if time.Since(c.libraryCacheTime) > c.cacheTTL {
		if err := c.refreshLibraryCache(); err != nil {
			// Cache refresh failed, try direct lookup
			return c.lookupLibraryID(nameOrID)
		}
	}

	// Check cache again after refresh
	if id, ok := c.libraryCache[nameOrID]; ok {
		return id, nil
	}

	// Not in cache, do direct lookup
	return c.lookupLibraryID(nameOrID)
}

// refreshLibraryCache fetches all libraries and updates the name->ID cache
func (c *Client) refreshLibraryCache() error {
	libs, err := c.ListLibraries()
	if err != nil {
		return err
	}

	c.libraryCache = make(map[string]string)
	for _, lib := range libs {
		c.libraryCache[lib.Name] = lib.ID
	}
	c.libraryCacheTime = time.Now()
	return nil
}

// lookupLibraryID does a direct API lookup for a library by name
func (c *Client) lookupLibraryID(name string) (string, error) {
	libs, err := c.ListLibraries()
	if err != nil {
		return "", err
	}

	for _, lib := range libs {
		if lib.Name == name {
			// Update cache
			c.libraryCache[name] = lib.ID
			c.libraryCacheTime = time.Now()
			return lib.ID, nil
		}
	}

	return "", fmt.Errorf("library not found: %s", name)
}

// doRequest makes an HTTP request with retries
func (c *Client) doRequest(method, endpoint string, body io.Reader, headers map[string]string) (*http.Response, error) {
	url := c.BaseURL + endpoint

	// Add default headers
	if headers == nil {
		headers = make(map[string]string)
	}
	headers["Authorization"] = "Bearer " + c.APIKey
	// Only set Content-Type to application/json if not already set
	if _, ok := headers["Content-Type"]; !ok {
		headers["Content-Type"] = "application/json"
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	var resp *http.Response
	for attempt := 0; attempt <= c.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(1<<uint(attempt)) * time.Second) // Exponential backoff
		}

		resp, err = c.HTTPClient.Do(req)
		if err != nil {
			if attempt == c.MaxRetries {
				return nil, err
			}
			continue
		}

		// Check for rate limiting
		if resp.StatusCode == http.StatusTooManyRequests {
			if attempt < c.MaxRetries {
				time.Sleep(c.RateLimitDelay)
				continue
			}
		}

		// Check for authentication errors
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("authentication failed: %s", resp.Status)
		}

		// Success or non-retryable error
		break
	}

	// Apply rate limit delay
	if c.RateLimitDelay > 0 && method != "GET" {
		time.Sleep(c.RateLimitDelay)
	}

	return resp, nil
}

// handleError handles API errors
func (c *Client) handleError(resp *http.Response, context string) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	// For non-success responses, we need to read the body
	// But we can't read it twice, so we'll read it here and return the error
	body, readErr := io.ReadAll(resp.Body)
	defer resp.Body.Close()

	if readErr != nil {
		return fmt.Errorf("failed to read error response: %v", readErr)
	}

	// Try to parse as JSON error
	var apiErr models.APIError
	if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Message != "" {
		return fmt.Errorf("API error (%d): %s (code: %s)", resp.StatusCode, apiErr.Message, apiErr.Code)
	}

	// Return raw body as error
	return fmt.Errorf("API error (%d): %s - %s", resp.StatusCode, context, string(body))
}

// ListLibraries lists all libraries
func (c *Client) ListLibraries() ([]models.Library, error) {
	resp, err := c.doRequest("GET", "/libraries", nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := c.handleError(resp, "list libraries"); err != nil {
		return nil, err
	}

	var result models.ListLibrariesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// GetLibrary gets a specific library by ID
func (c *Client) GetLibrary(libraryID string) (*models.Library, error) {
	resp, err := c.doRequest("GET", fmt.Sprintf("/libraries/%s", libraryID), nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := c.handleError(resp, "get library"); err != nil {
		return nil, err
	}

	var library models.Library
	if err := json.NewDecoder(resp.Body).Decode(&library); err != nil {
		return nil, err
	}

	return &library, nil
}

// CreateLibrary creates a new library
func (c *Client) CreateLibrary(name string, description string) (*models.Library, error) {
	payload := map[string]interface{}{
		"name": name,
	}
	if description != "" {
		payload["description"] = description
	}

	body, _ := json.Marshal(payload)

	resp, err := c.doRequest("POST", "/libraries", bytes.NewBuffer(body), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := c.handleError(resp, "create library"); err != nil {
		return nil, err
	}

	var library models.Library
	if err := json.NewDecoder(resp.Body).Decode(&library); err != nil {
		return nil, err
	}

	return &library, nil
}

// UpdateLibrary updates a library
func (c *Client) UpdateLibrary(libraryID string, name string, description *string) (*models.Library, error) {
	payload := map[string]interface{}{
		"name": name,
	}
	if description != nil {
		payload["description"] = *description
	}

	body, _ := json.Marshal(payload)

	resp, err := c.doRequest("PATCH", fmt.Sprintf("/libraries/%s", libraryID), bytes.NewBuffer(body), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := c.handleError(resp, "update library"); err != nil {
		return nil, err
	}

	var library models.Library
	if err := json.NewDecoder(resp.Body).Decode(&library); err != nil {
		return nil, err
	}

	return &library, nil
}

// DeleteLibrary deletes a library
func (c *Client) DeleteLibrary(libraryID string) error {
	resp, err := c.doRequest("DELETE", fmt.Sprintf("/libraries/%s", libraryID), nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		return nil
	}

	return c.handleError(resp, "delete library")
}

// ListDocuments lists all documents in a library
func (c *Client) ListDocuments(libraryID string) ([]models.Document, error) {
	resp, err := c.doRequest("GET", fmt.Sprintf("/libraries/%s/documents", libraryID), nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := c.handleError(resp, "list documents"); err != nil {
		return nil, err
	}

	var result models.ListDocumentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// GetDocument gets a specific document by ID
func (c *Client) GetDocument(libraryID string, documentID string) (*models.Document, error) {
	resp, err := c.doRequest("GET", fmt.Sprintf("/libraries/%s/documents/%s", libraryID, documentID), nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := c.handleError(resp, "get document"); err != nil {
		return nil, err
	}

	var doc models.Document
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, err
	}

	return &doc, nil
}

// UploadDocument uploads a file to a library
func (c *Client) UploadDocument(libraryID string, filePath string) (*models.Document, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add file
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, err
	}

	if _, err := io.Copy(part, file); err != nil {
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	resp, err := c.doRequest("POST", fmt.Sprintf("/libraries/%s/documents", libraryID), body, map[string]string{
		"Content-Type": writer.FormDataContentType(),
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := c.handleError(resp, "upload document"); err != nil {
		return nil, err
	}

	var doc models.Document
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, err
	}

	return &doc, nil
}

// DeleteDocument deletes a document from a library
func (c *Client) DeleteDocument(libraryID string, documentID string) error {
	resp, err := c.doRequest("DELETE", fmt.Sprintf("/libraries/%s/documents/%s", libraryID, documentID), nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		return nil
	}

	return c.handleError(resp, "delete document")
}

// GetSignedURL gets a signed URL for downloading a document
func (c *Client) GetSignedURL(libraryID string, documentID string) (string, error) {
	resp, err := c.doRequest("GET", fmt.Sprintf("/libraries/%s/documents/%s/signed-url", libraryID, documentID), nil, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if err := c.handleError(resp, "get signed URL"); err != nil {
		return "", err
	}

	body, _ := io.ReadAll(resp.Body)
	url := strings.Trim(string(body), "\"\n")
	return url, nil
}

// DownloadDocument downloads a document to a file
func (c *Client) DownloadDocument(libraryID string, documentID string, outputPath string) error {
	signedURL, err := c.GetSignedURL(libraryID, documentID)
	if err != nil {
		return err
	}

	resp, err := http.Get(signedURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download: %s", resp.Status)
	}

	// Create output directory if needed
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}

	outFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, resp.Body); err != nil {
		return err
	}

	return nil
}
