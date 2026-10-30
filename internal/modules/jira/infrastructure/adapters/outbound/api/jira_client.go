// Package api implements the Jira client port over the REST API v2 of a Jira Server/Data Center.
//
//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package api

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/domain/ports/outbound"
)

const (
	requestTimeout = 30 * time.Second
	connectTimeout = 10 * time.Second
	// maxRedirects is the redirect budget of net/http's default policy, which the Rust client used too.
	maxRedirects = 10
	// maxResponseBytes bounds what is read from one answer; a broad JQL or a hostile server cannot exhaust memory.
	maxResponseBytes = 8 << 20
	pageSize         = 100
	maxComments      = 200
)

// MaxImportedIssues bounds the issues imported per sync, so a broad JQL cannot flood the local database.
const MaxImportedIssues = 500

// JiraClient talks to Jira over HTTP(S) with a Bearer personal access token.
//
// Credentials stay out of URLs and errors: the token travels only in the Authorization header, and
// an error carries the text of the server, never the request.
type JiraClient struct {
	strict  *http.Client
	lenient *http.Client
	now     func() time.Time
}

var _ outbound.JiraClient = (*JiraClient)(nil)

// NewJiraClient builds the client. Two HTTP clients are kept: one verifies certificates strictly and
// one accepts untrusted certificates, because that is a property of the server the user configured.
func NewJiraClient(now func() time.Time) *JiraClient {
	return &JiraClient{strict: newHTTPClient(false), lenient: newHTTPClient(true), now: now}
}

// Verify checks URL and token and returns the display name of the authenticated user.
func (c *JiraClient) Verify(ctx context.Context, connection models.Connection) (string, error) {
	body, err := c.send(ctx, connection, http.MethodGet, "myself", nil, nil, "")
	if err != nil {
		return "", err
	}
	doc, err := decodeDocument(body)
	if err != nil {
		return "", err
	}
	if name := textAt(doc, "/displayName"); name != nil {
		return *name, nil
	}
	if name := textAt(doc, "/name"); name != nil {
		return *name, nil
	}
	return "", nil
}

// Search returns the issues of one JQL query, following the pages up to MaxImportedIssues.
func (c *JiraClient) Search(ctx context.Context, connection models.Connection, jql string) ([]models.RemoteIssue, error) {
	var all []models.RemoteIssue
	startAt := 0
	for {
		query := url.Values{}
		query.Set("jql", jql)
		query.Set("fields", issueFields)
		query.Set("startAt", strconv.Itoa(startAt))
		query.Set("maxResults", strconv.Itoa(pageSize))
		body, err := c.send(ctx, connection, http.MethodGet, "search", query, nil, "")
		if err != nil {
			return nil, err
		}
		doc, err := decodeDocument(body)
		if err != nil {
			return nil, err
		}
		page, next, total, err := searchPageFromJSON(connection, c.now, doc)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if len(page) == 0 || next >= total || len(all) >= MaxImportedIssues {
			break
		}
		startAt = next
	}
	if len(all) > MaxImportedIssues {
		all = all[:MaxImportedIssues]
	}
	return all, nil
}

// GetIssue reads one issue. A key that is not a plain PROJ-123 key is refused before any request.
func (c *JiraClient) GetIssue(ctx context.Context, connection models.Connection, key string) (models.RemoteIssue, error) {
	path, err := issuePath(key, "")
	if err != nil {
		return models.RemoteIssue{}, err
	}
	query := url.Values{"fields": {issueFields}}
	body, err := c.send(ctx, connection, http.MethodGet, path, query, nil, key)
	if err != nil {
		return models.RemoteIssue{}, err
	}
	doc, err := decodeDocument(body)
	if err != nil {
		return models.RemoteIssue{}, err
	}
	return issueFromJSON(connection, c.now, doc)
}

// Comments reads the latest comments of an issue.
func (c *JiraClient) Comments(ctx context.Context, connection models.Connection, key string) ([]models.Comment, error) {
	path, err := issuePath(key, "/comment")
	if err != nil {
		return nil, err
	}
	query := url.Values{"maxResults": {strconv.Itoa(maxComments)}}
	body, err := c.send(ctx, connection, http.MethodGet, path, query, nil, key)
	if err != nil {
		return nil, err
	}
	doc, err := decodeDocument(body)
	if err != nil {
		return nil, err
	}
	return commentsFromJSON(c.now, doc), nil
}

// AddComment posts a comment on the issue.
func (c *JiraClient) AddComment(ctx context.Context, connection models.Connection, key string, body string) error {
	path, err := issuePath(key, "/comment")
	if err != nil {
		return err
	}
	payload, err := encodePayload(map[string]any{"body": body})
	if err != nil {
		return err
	}
	_, err = c.send(ctx, connection, http.MethodPost, path, nil, payload, key)
	return err
}

// Transitions lists the status changes Jira allows right now for the issue.
func (c *JiraClient) Transitions(ctx context.Context, connection models.Connection, key string) ([]models.Transition, error) {
	path, err := issuePath(key, "/transitions")
	if err != nil {
		return nil, err
	}
	body, err := c.send(ctx, connection, http.MethodGet, path, nil, nil, key)
	if err != nil {
		return nil, err
	}
	doc, err := decodeDocument(body)
	if err != nil {
		return nil, err
	}
	return transitionsFromJSON(doc), nil
}

// Transition moves the issue through one of its allowed transitions.
func (c *JiraClient) Transition(ctx context.Context, connection models.Connection, key string, transitionID string) error {
	path, err := issuePath(key, "/transitions")
	if err != nil {
		return err
	}
	payload, err := encodePayload(map[string]any{"transition": map[string]any{"id": transitionID}})
	if err != nil {
		return err
	}
	_, err = c.send(ctx, connection, http.MethodPost, path, nil, payload, key)
	return err
}

// send performs one request and returns the body of a successful answer. notFoundKey, when set, turns
// a 404 into IssueNotFound for that key.
func (c *JiraClient) send(
	ctx context.Context,
	connection models.Connection,
	method string,
	path string,
	query url.Values,
	payload []byte,
	notFoundKey string,
) ([]byte, error) {
	target := connection.APIURL(path)
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	var reader io.Reader
	if payload != nil {
		reader = strings.NewReader(string(payload))
	}
	request, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return nil, jiraerrors.Remote(networkMessage(err))
	}
	request.Header.Set("Authorization", "Bearer "+connection.Token())
	request.Header.Set("Accept", "application/json")
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.clientFor(connection).Do(request)
	if err != nil {
		return nil, jiraerrors.Remote(describeTransport(err))
	}
	defer func() { _ = response.Body.Close() }()
	body, readErr := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	status := response.StatusCode
	if status >= 200 && status < 300 {
		if readErr != nil {
			return nil, jiraerrors.Remote("respuesta no válida: " + networkMessage(readErr))
		}
		return body, nil
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return nil, jiraerrors.Unauthorized()
	}
	if status == http.StatusNotFound && notFoundKey != "" {
		return nil, jiraerrors.IssueNotFound(notFoundKey)
	}
	detail := errorMessage(body)
	if detail == "" {
		detail = statusText(status)
	}
	return nil, jiraerrors.Remote(fmt.Sprintf("HTTP %d: %s", status, detail))
}

func (c *JiraClient) clientFor(connection models.Connection) *http.Client {
	if connection.AcceptInvalidCerts() {
		return c.lenient
	}
	return c.strict
}

// issuePath is the REST path of an issue resource. Keys that are not plain PROJ-123 keys are refused,
// so a crafted key cannot redirect the request to another endpoint.
func issuePath(key string, suffix string) (string, error) {
	if !models.IsValidIssueKey(key) {
		return "", jiraerrors.InvalidIssueKey(key)
	}
	return "issue/" + key + suffix, nil
}

// newHTTPClient applies the timeouts and the redirect policy. A redirect is followed like the Rust
// client did, but the Authorization header is dropped as soon as the origin changes: the token is
// never handed to another server.
func newHTTPClient(acceptInvalidCerts bool) *http.Client {
	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		DialContext:         (&net.Dialer{Timeout: connectTimeout}).DialContext,
		ForceAttemptHTTP2:   true,
		TLSHandshakeTimeout: connectTimeout,
		// Only the server the user marked as insecure gets this client, never the default one.
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: acceptInvalidCerts},
	}
	return &http.Client{
		Timeout:   requestTimeout,
		Transport: transport,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return errors.New("too many redirects")
			}
			if len(via) > 0 && request.Method != via[0].Method && via[0].Method == http.MethodPost {
				return errors.New("cannot follow redirect that downgrades POST to " + request.Method)
			}
			if !sameOrigin(request.URL, via[0].URL) {
				request.Header.Del("Authorization")
			}
			return nil
		},
	}
}

// sameOrigin compares scheme and host (port included).
func sameOrigin(first *url.URL, second *url.URL) bool {
	return strings.EqualFold(first.Scheme, second.Scheme) && strings.EqualFold(first.Host, second.Host)
}

// describeTransport gives the user a short reason for a request that never got an answer.
func describeTransport(err error) string {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "tiempo de espera agotado"
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) && opErr.Op == "dial" {
		return fmt.Sprintf("no se pudo conectar (%s)", networkMessage(err))
	}
	return networkMessage(err)
}

// networkMessage drops the URL that net/http puts in its errors, so nothing else leaks with it.
func networkMessage(err error) string {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Err.Error()
	}
	return err.Error()
}

func statusText(status int) string {
	if text := http.StatusText(status); text != "" {
		return fmt.Sprintf("%d %s", status, text)
	}
	return strconv.Itoa(status)
}
