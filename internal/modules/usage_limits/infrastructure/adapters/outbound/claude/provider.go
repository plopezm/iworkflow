// Package claude is the facade over the Claude subscription usage: it reads the OAuth token Claude Code stores
// and asks the usage endpoint of Anthropic. The endpoint is not documented, so every failure degrades to
// ErrUnavailable instead of breaking the caller.
package claude

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	usageerrors "iworkflow/internal/modules/usage_limits/domain/errors"
	"iworkflow/internal/modules/usage_limits/domain/models"
	"iworkflow/internal/modules/usage_limits/domain/ports/outbound"
)

const (
	// DefaultEndpoint is the usage endpoint of the subscription.
	DefaultEndpoint string = "https://api.anthropic.com/api/oauth/usage"
	betaHeader      string = "oauth-2025-04-20"
	requestTimeout         = 10 * time.Second
	maxBodyBytes    int64  = 1 << 20
)

// Config is what the facade needs. Every field is injected so tests need no network and no real credentials.
type Config struct {
	// CredentialsPath is the file Claude Code keeps its OAuth credentials in (~/.claude/.credentials.json).
	CredentialsPath string
	// Endpoint is the usage URL; DefaultEndpoint when empty.
	Endpoint string
	// Client is the HTTP client; a client with a timeout when nil.
	Client *http.Client
	Clock  outbound.ClockPort
}

// Provider implements UsageProviderPort for Claude.
type Provider struct {
	credentialsPath string
	endpoint        string
	client          *http.Client
	clock           outbound.ClockPort
}

var _ outbound.UsageProviderPort = (*Provider)(nil)

// NewProvider builds the facade.
func NewProvider(cfg Config) *Provider {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: requestTimeout}
	}
	return &Provider{credentialsPath: cfg.CredentialsPath, endpoint: endpoint, client: client, clock: cfg.Clock}
}

// ID is the provider this facade reads.
func (*Provider) ID() models.ProviderID { return models.ProviderClaude }

type credentialsFile struct {
	OAuth struct {
		AccessToken string `json:"accessToken"`
	} `json:"claudeAiOauth"`
}

type windowDTO struct {
	Utilization *float64 `json:"utilization"`
	ResetsAt    *string  `json:"resets_at"`
}

type usageDTO struct {
	FiveHour *windowDTO `json:"five_hour"`
	SevenDay *windowDTO `json:"seven_day"`
}

// Fetch asks the endpoint for the usage of the subscription. A missing credentials file or token means the
// provider is not configured. Error messages never carry the token or the response body.
func (p *Provider) Fetch(ctx context.Context) (models.ProviderUsage, error) {
	token, err := p.token()
	if err != nil {
		return models.ProviderUsage{}, err
	}
	body, err := p.get(ctx, token)
	if err != nil {
		return models.ProviderUsage{}, err
	}
	var dto usageDTO
	if err := json.Unmarshal(body, &dto); err != nil {
		return models.ProviderUsage{}, fmt.Errorf("decode the usage answer: %w", usageerrors.ErrUnavailable)
	}
	windows := make([]models.Window, 0, 2)
	for _, entry := range []struct {
		name models.WindowName
		dto  *windowDTO
	}{{models.WindowFiveHour, dto.FiveHour}, {models.WindowWeek, dto.SevenDay}} {
		window, ok, err := toWindow(entry.name, entry.dto)
		if err != nil {
			return models.ProviderUsage{}, fmt.Errorf("%w: %w", usageerrors.ErrUnavailable, err)
		}
		if ok {
			windows = append(windows, window)
		}
	}
	usage, err := models.NewProviderUsage(models.ProviderClaude, windows, p.clock.Now())
	if err != nil {
		return models.ProviderUsage{}, fmt.Errorf("%w: %w", usageerrors.ErrUnavailable, err)
	}
	return usage, nil
}

func (p *Provider) token() (string, error) {
	raw, err := os.ReadFile(p.credentialsPath)
	if errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("no Claude credentials: %w", usageerrors.ErrNotConfigured)
	}
	if err != nil {
		return "", fmt.Errorf("read the Claude credentials: %w", usageerrors.ErrUnavailable)
	}
	var creds credentialsFile
	if err := json.Unmarshal(raw, &creds); err != nil {
		return "", fmt.Errorf("decode the Claude credentials: %w", usageerrors.ErrUnavailable)
	}
	if creds.OAuth.AccessToken == "" {
		return "", fmt.Errorf("no subscription token: %w", usageerrors.ErrNotConfigured)
	}
	return creds.OAuth.AccessToken, nil
}

func (p *Provider) get(ctx context.Context, token string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("build the usage request: %w", usageerrors.ErrUnavailable)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("anthropic-beta", betaHeader)
	request.Header.Set("Accept", "application/json")
	response, err := p.client.Do(request)
	if err != nil {
		// The transport error repeats the URL only, but its text is not part of the contract: keep it out.
		return nil, fmt.Errorf("call the usage endpoint: %w", usageerrors.ErrUnavailable)
	}
	defer func() { _ = response.Body.Close() }() // a close failure after a full read changes nothing
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("usage endpoint answered %d: %w", response.StatusCode, usageerrors.ErrUnavailable)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("read the usage answer: %w", usageerrors.ErrUnavailable)
	}
	return body, nil
}

// toWindow maps one window; ok is false when the endpoint has no data for it (null).
func toWindow(name models.WindowName, dto *windowDTO) (models.Window, bool, error) {
	if dto == nil || dto.Utilization == nil {
		return models.Window{}, false, nil
	}
	var resetsAt time.Time
	if dto.ResetsAt != nil {
		parsed, err := time.Parse(time.RFC3339Nano, *dto.ResetsAt)
		if err != nil {
			return models.Window{}, false, fmt.Errorf("window %s reset time: %w", name, err)
		}
		resetsAt = parsed
	}
	window, err := models.NewWindow(name, *dto.Utilization, resetsAt)
	if err != nil {
		return models.Window{}, false, fmt.Errorf("map window %s: %w", name, err)
	}
	return window, true, nil
}
