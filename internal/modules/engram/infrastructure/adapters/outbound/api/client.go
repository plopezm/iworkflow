// Package api is the outbound adapter that reads Engram through its local HTTP API. It only issues
// GET requests and decodes only the metadata the module needs: the memory content is never decoded.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	engramerrors "iworkflow/internal/modules/engram/domain/errors"
	"iworkflow/internal/modules/engram/domain/models"
	"iworkflow/internal/modules/engram/domain/ports/outbound"
)

// DefaultBaseURL is the usual address of the Engram HTTP API, for documentation and for setting IWF_ENGRAM_URL.
// Nothing applies it implicitly: the feature stays off until IWF_ENGRAM_URL is set to a valid URL.
const DefaultBaseURL = "http://127.0.0.1:7437"

const (
	requestTimeout   = 2 * time.Second
	projectsPath     = "/projects"
	recentPath       = "/observations/recent"
	maxResponseBytes = 4 << 20
	sqliteTimeLayout = "2006-01-02 15:04:05"
)

// Client implements outbound.MemoryStore over the Engram HTTP API.
type Client struct {
	baseURL *url.URL
	http    *http.Client
}

var _ outbound.MemoryStore = (*Client)(nil)

// projectsResponse is the envelope of GET /projects. Only the fields the module uses are declared.
type projectsResponse struct {
	Projects []projectWire `json:"projects"`
}

type projectWire struct {
	Name             string   `json:"name"`
	ObservationCount int      `json:"observation_count"`
	Directories      []string `json:"directories"`
}

// recentObservation is one element of GET /observations/recent. The content field is deliberately absent.
type recentObservation struct {
	ID         int    `json:"id"`
	Type       string `json:"type"`
	Title      string `json:"title"`
	LastSeenAt string `json:"last_seen_at"`
}

// NewClient builds a client for the Engram API at baseURL with an explicit 2 second timeout.
func NewClient(baseURL string) (*Client, error) {
	return newClient(baseURL, requestTimeout)
}

func newClient(baseURL string, timeout time.Duration) (*Client, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse engram base url: %w", err)
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, fmt.Errorf("engram base url %q must be an absolute http or https URL", baseURL)
	}
	return &Client{baseURL: parsed, http: &http.Client{Timeout: timeout}}, nil
}

// Projects lists the Engram projects with their directories.
func (c *Client) Projects(ctx context.Context) ([]models.MemoryProject, error) {
	var body projectsResponse
	if err := c.get(ctx, projectsPath, nil, &body); err != nil {
		return nil, err
	}
	projects := make([]models.MemoryProject, 0, len(body.Projects))
	for _, project := range body.Projects {
		projects = append(projects, models.NewMemoryProject(project.Name, project.ObservationCount, project.Directories))
	}
	return projects, nil
}

// Recent lists up to limit observations of a project, without their content.
func (c *Client) Recent(ctx context.Context, project string, limit int) ([]models.Memory, error) {
	query := url.Values{}
	query.Set("project", project)
	query.Set("limit", strconv.Itoa(limit))
	var body []recentObservation
	if err := c.get(ctx, recentPath, query, &body); err != nil {
		return nil, err
	}
	memories := make([]models.Memory, 0, len(body))
	for _, observation := range body {
		lastSeen, err := parseTimestamp(observation.LastSeenAt)
		if err != nil {
			return nil, fmt.Errorf("engram %s: observation %d timestamp: %w: %w", recentPath, observation.ID, engramerrors.ErrUnavailable, err)
		}
		memories = append(memories, models.NewMemory(observation.ID, observation.Type, observation.Title, lastSeen))
	}
	return memories, nil
}

// get issues a GET request and decodes the JSON answer into out. Any answer other than 200 OK, any
// transport failure and any unreadable payload is reported as engramerrors.ErrUnavailable, except
// when the caller's context itself was cancelled.
func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	endpoint := c.baseURL.JoinPath(path)
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return fmt.Errorf("build engram request %s: %w", path, err)
	}
	response, err := c.http.Do(request)
	if err != nil {
		return classify(ctx, path, "request failed", err)
	}
	defer func() {
		// The body is fully consumed or discarded; a close error carries no information for the caller.
		_ = response.Body.Close()
	}()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("engram %s answered status %d: %w", path, response.StatusCode, engramerrors.ErrUnavailable)
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, maxResponseBytes)).Decode(out); err != nil {
		return classify(ctx, path, "decode response", err)
	}
	return nil
}

func classify(ctx context.Context, path string, action string, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return fmt.Errorf("engram %s %s: %w", path, action, ctxErr)
	}
	return fmt.Errorf("engram %s %s: %w: %w", path, action, engramerrors.ErrUnavailable, err)
}

// parseTimestamp reads last_seen_at, which Engram stores as SQLite UTC text. An empty value is unknown.
func parseTimestamp(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	if parsed, err := time.ParseInLocation(sqliteTimeLayout, value, time.UTC); err == nil {
		return parsed, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse last_seen_at %q: %w", value, err)
	}
	return parsed, nil
}
