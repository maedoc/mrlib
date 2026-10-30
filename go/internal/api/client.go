package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/maedoc/mrlib/go/internal/models"
)

const (
	maxErrorBodyBytes = 1 << 20
	maxPageSize       = 100
)

// Client represents the Mistral AI API client
type Client struct {
	APIKey         string
	BaseURL        string
	Timeout        time.Duration
	RateLimitDelay time.Duration
	MaxRetries     int
	HTTPClient     *http.Client
	// Cache for library name -> ID resolution
	libraryCache     map[string]string // name -> id
	libraryCacheTime time.Time         // last cache update
	cacheTTL         time.Duration     // cache time-to-live
}

// NewClient creates a new Mistral API client
func NewClient(apiKey string, baseURL string, timeout time.Duration, rateLimitDelay time.Duration, maxRetries int) *Client {
	return &Client{
		APIKey:         apiKey,
		BaseURL:        strings.TrimRight(baseURL, "/"),
		Timeout:        timeout,
		RateLimitDelay: rateLimitDelay,
		MaxRetries:     maxRetries,
		HTTPClient:     &http.Client{Timeout: timeout},
		libraryCache:   make(map[string]string),
		cacheTTL:       5 * time.Minute,
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

// retryable reports whether the response status is worth retrying
func retryable(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests ||
		statusCode == http.StatusInternalServerError ||
		statusCode == http.StatusBadGateway ||
		statusCode == http.StatusServiceUnavailable ||
		statusCode == http.StatusGatewayTimeout
}

// retryDelay returns how long to wait before the given retry attempt
func (c *Client) retryDelay(resp *http.Response, attempt int) time.Duration {
	if resp != nil {
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if secs, err := strconv.Atoi(strings.TrimSpace(ra)); err == nil && secs >= 0 {
				return time.Duration(secs) * time.Second
			}
			if t, err := http.ParseTime(ra); err == nil {
				if d := time.Until(t); d > 0 {
					return d
				}
			}
		}
	}
	backoff := time.Duration(1<<uint(attempt)) * time.Second
	if max := 30 * time.Second; backoff > max {
		backoff = max
	}
	return backoff
}

// doRequest makes an HTTP request with retries.
// The body reader is replayed on each attempt via req.GetBody.
func (c *Client) doRequest(method, endpoint string, body io.Reader, headers map[string]string) (*http.Response, error) {
	url := c.BaseURL + endpoint

	if headers == nil {
		headers = make(map[string]string)
	}
	headers["Authorization"] = "Bearer " + c.APIKey
	// Only set Content-Type to application/json if not already set
	if _, ok := headers["Content-Type"]; !ok {
		headers["Content-Type"] = "application/json"
	}

	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest(method, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	if bodyBytes != nil {
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(bodyBytes)), nil
		}
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	var resp *http.Response
	for attempt := 0; attempt <= c.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(c.retryDelay(resp, attempt-1))
			// Re-create the request body for the retry
			if req.GetBody != nil {
				b, err := req.GetBody()
				if err != nil {
					return nil, err
				}
				req.Body = b
			}
		}

		resp, err = c.HTTPClient.Do(req)
		if err != nil {
			if attempt == c.MaxRetries {
				return nil, err
			}
			continue
		}

		if resp.StatusCode == http.StatusUnauthorized {
			io.Copy(io.Discard, io.LimitReader(resp.Body, maxErrorBodyBytes))
			resp.Body.Close()
			return nil, fmt.Errorf("authentication failed: %s", resp.Status)
		}

		if retryable(resp.StatusCode) && attempt < c.MaxRetries {
			io.Copy(io.Discard, io.LimitReader(resp.Body, maxErrorBodyBytes))
			resp.Body.Close()
			resp = nil
			continue
		}

		break
	}

	if resp == nil {
		return nil, fmt.Errorf("request failed after %d retries", c.MaxRetries)
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

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
	resp.Body.Close()

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

// listPage is the shared envelope for paginated list responses.
// The API has used both top-level has_more/total and a pagination object
// with next_page_token, so both are accepted.
type listPage struct {
	Data       json.RawMessage `json:"data"`
	HasMore    bool            `json:"has_more"`
	Total      int             `json:"total"`
	Pagination *struct {
		HasMore       bool   `json:"has_more"`
		NextPageToken string `json:"next_page_token"`
	} `json:"pagination"`
}

// listAll fetches all pages of a paginated endpoint
func (c *Client) listAll(endpoint string, pageSize int, decode func(json.RawMessage) error) error {
	if pageSize <= 0 || pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	page := 0
	pageToken := ""
	for {
		sep := "?"
		if strings.Contains(endpoint, "?") {
			sep = "&"
		}
		pageEndpoint := fmt.Sprintf("%s%spage=%d&page_size=%d", endpoint, sep, page, pageSize)
		if pageToken != "" {
			pageEndpoint += "&page_token=" + url.QueryEscape(pageToken)
		}

		resp, err := c.doRequest("GET", pageEndpoint, nil, nil)
		if err != nil {
			return err
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return c.handleError(resp, "list")
		}

		var lp listPage
		decodeErr := json.NewDecoder(resp.Body).Decode(&lp)
		resp.Body.Close()
		if decodeErr != nil {
			return decodeErr
		}
		if err := decode(lp.Data); err != nil {
			return err
		}

		hasMore := lp.HasMore
		nextToken := ""
		if lp.Pagination != nil {
			if lp.Pagination.HasMore {
				hasMore = true
			}
			nextToken = lp.Pagination.NextPageToken
		}

		if !hasMore && nextToken == "" {
			return nil
		}
		if nextToken != "" {
			pageToken = nextToken
		} else {
			page++
		}
	}
}

// ListLibraries lists all libraries (following pagination)
func (c *Client) ListLibraries() ([]models.Library, error) {
	var libs []models.Library
	err := c.listAll("/libraries", maxPageSize, func(data json.RawMessage) error {
		var page []models.Library
		if err := json.Unmarshal(data, &page); err != nil {
			return err
		}
		libs = append(libs, page...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return libs, nil
}

// GetLibrary gets a specific library by ID
func (c *Client) GetLibrary(libraryID string) (*models.Library, error) {
	resp, err := c.doRequest("GET", fmt.Sprintf("/libraries/%s", url.PathEscape(libraryID)), nil, nil)
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

	resp, err := c.doRequest("PATCH", fmt.Sprintf("/libraries/%s", url.PathEscape(libraryID)), bytes.NewBuffer(body), nil)
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
	resp, err := c.doRequest("DELETE", fmt.Sprintf("/libraries/%s", url.PathEscape(libraryID)), nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		return nil
	}

	return c.handleError(resp, "delete library")
}

// ListDocuments lists all documents in a library (following pagination)
func (c *Client) ListDocuments(libraryID string) ([]models.Document, error) {
	var docs []models.Document
	endpoint := fmt.Sprintf("/libraries/%s/documents", url.PathEscape(libraryID))
	err := c.listAll(endpoint, maxPageSize, func(data json.RawMessage) error {
		var page []models.Document
		if err := json.Unmarshal(data, &page); err != nil {
			return err
		}
		docs = append(docs, page...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return docs, nil
}

// GetDocument gets a specific document by ID
func (c *Client) GetDocument(libraryID string, documentID string) (*models.Document, error) {
	resp, err := c.doRequest("GET", fmt.Sprintf("/libraries/%s/documents/%s", url.PathEscape(libraryID), url.PathEscape(documentID)), nil, nil)
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

	resp, err := c.doRequest("POST", fmt.Sprintf("/libraries/%s/documents", url.PathEscape(libraryID)), body, map[string]string{
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
	resp, err := c.doRequest("DELETE", fmt.Sprintf("/libraries/%s/documents/%s", url.PathEscape(libraryID), url.PathEscape(documentID)), nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		return nil
	}

	return c.handleError(resp, "delete document")
}

// signedURLResponse is the JSON body returned by the signed-url endpoint
type signedURLResponse struct {
	URL string `json:"url"`
}

// GetSignedURL gets a signed URL for downloading a document
func (c *Client) GetSignedURL(libraryID string, documentID string) (string, error) {
	resp, err := c.doRequest("GET", fmt.Sprintf("/libraries/%s/documents/%s/signed-url", url.PathEscape(libraryID), url.PathEscape(documentID)), nil, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if err := c.handleError(resp, "get signed URL"); err != nil {
		return "", err
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))

	// The endpoint returns a JSON object {"url": "..."}; accept a bare
	// JSON string too for compatibility.
	var sur signedURLResponse
	if err := json.Unmarshal(body, &sur); err == nil && sur.URL != "" {
		return sur.URL, nil
	}
	var rawURL string
	if err := json.Unmarshal(body, &rawURL); err == nil && rawURL != "" {
		return rawURL, nil
	}
	return "", fmt.Errorf("could not parse signed URL response: %s", string(body))
}

// SafeJoin returns the path joined to base, or an error if the result would
// escape base (path traversal protection for remote-controlled names)
func SafeJoin(base, name string) (string, error) {
	clean := filepath.Clean(string(filepath.Separator) + name)
	joined := filepath.Join(base, clean)
	if joined != base && !strings.HasPrefix(joined, base+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid file name: %q", name)
	}
	return joined, nil
}

// DownloadDocument downloads a document to a file.
// The output is written to a temporary file first and atomically renamed,
// so a failed download never leaves a partial file behind.
func (c *Client) DownloadDocument(libraryID string, documentID string, outputPath string) error {
	signedURL, err := c.GetSignedURL(libraryID, documentID)
	if err != nil {
		return err
	}

	u, err := url.Parse(signedURL)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return fmt.Errorf("refusing to download from non-HTTPS signed URL")
	}

	dlClient := &http.Client{
		Timeout: c.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.Scheme != "https" {
				return fmt.Errorf("refusing redirect to non-HTTPS URL")
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, signedURL, nil)
	if err != nil {
		return err
	}
	resp, err := dlClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download: %s", resp.Status)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(outputPath), ".mrlib-dl-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := io.Copy(tmp, io.LimitReader(resp.Body, 1<<31)); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0644); err != nil {
		return err
	}
	return os.Rename(tmpName, outputPath)
}
