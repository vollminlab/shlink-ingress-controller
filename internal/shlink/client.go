package shlink

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client is the interface for interacting with the Shlink API.
type Client interface {
	GetShortURL(slug string) (*ShortURL, error)
	CreateShortURL(slug, longURL string) error
	DeleteShortURL(slug string) error
}

// ShortURL represents a Shlink short URL resource.
type ShortURL struct {
	ShortCode string `json:"shortCode"`
	LongURL   string `json:"longUrl"`
}

// HTTPClient implements Client using the Shlink REST API.
type HTTPClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// New creates an HTTPClient for the given base URL (e.g. "https://go.vollminlab.com/rest/v3") and API key.
func New(baseURL, apiKey string) *HTTPClient {
	return &HTTPClient{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// GetShortURL returns the ShortURL for slug, or nil if the slug does not exist (404).
// Any non-200/404 response returns an error.
func (c *HTTPClient) GetShortURL(slug string) (*ShortURL, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/short-urls/%s", c.baseURL, slug), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET short-urls/%s: unexpected status %d", slug, resp.StatusCode)
	}

	var su ShortURL
	if err := json.NewDecoder(resp.Body).Decode(&su); err != nil {
		return nil, err
	}
	return &su, nil
}

type createRequest struct {
	LongURL    string `json:"longUrl"`
	CustomSlug string `json:"customSlug"`
}

// CreateShortURL creates a short link. Returns nil if the slug already exists (409).
func (c *HTTPClient) CreateShortURL(slug, longURL string) error {
	body, err := json.Marshal(createRequest{LongURL: longURL, CustomSlug: slug})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/short-urls", c.baseURL), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return nil // slug already exists — treat as success
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("POST short-urls: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// DeleteShortURL deletes the short link for slug.
func (c *HTTPClient) DeleteShortURL(slug string) error {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/short-urls/%s", c.baseURL, slug), nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Api-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		return nil
	}
	return fmt.Errorf("DELETE short-urls/%s: unexpected status %d", slug, resp.StatusCode)
}
