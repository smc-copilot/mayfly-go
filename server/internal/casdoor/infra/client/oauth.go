package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"mayfly-go/internal/casdoor/domain/directory"
)

type OAuthClient struct {
	cfg       Config
	transport *httpClient
	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

func NewOAuthClient(cfg Config) *OAuthClient {
	return &OAuthClient{cfg: cfg, transport: New(cfg).(*httpClient)}
}

func (c *OAuthClient) reader(ctx context.Context, organization string) (*httpClient, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token == "" || !time.Now().Before(c.expiresAt) {
		payload, err := json.Marshal(map[string]string{"grant_type": "client_credentials", "client_id": c.cfg.ClientID, "client_secret": c.cfg.ClientSecret})
		if err != nil {
			return nil, fmt.Errorf("token_request_invalid")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.cfg.ServerURL, "/")+"/api/login/oauth/access_token", bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("token_request_invalid")
		}
		req.Header.Set("Content-Type", "application/json")
		started := time.Now()
		resp, err := c.transport.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("token_endpoint_unavailable")
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
		closeErr := resp.Body.Close()
		if readErr != nil || closeErr != nil || len(body) > 1<<20 || resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("token_request_rejected")
		}
		var token struct {
			AccessToken string  `json:"access_token"`
			TokenType   string  `json:"token_type"`
			ExpiresIn   float64 `json:"expires_in"`
		}
		if json.Unmarshal(body, &token) != nil || token.AccessToken == "" || !strings.EqualFold(token.TokenType, "Bearer") || token.ExpiresIn <= 0 || token.ExpiresIn > 31536000 {
			return nil, fmt.Errorf("token_response_invalid")
		}
		ttl := time.Duration(token.ExpiresIn * float64(time.Second))
		margin := min(ttl/10, 30*time.Second)
		c.token, c.expiresAt = token.AccessToken, started.Add(ttl-margin)
		if !time.Now().Before(c.expiresAt) {
			c.token = ""
			return nil, fmt.Errorf("token_already_expired")
		}
	}
	cfg := c.cfg
	cfg.BearerToken = c.token
	cfg.Organization = organization
	cfg.AppOwner = ""
	cfg.ClientSecret = ""
	return New(cfg).(*httpClient), nil
}

func (c *OAuthClient) Reader(ctx context.Context, organization string) (Client, error) {
	return c.reader(ctx, organization)
}

func (c *OAuthClient) Organizations(ctx context.Context) ([]directory.Organization, error) {
	reader, err := c.reader(ctx, "")
	if err != nil {
		return nil, err
	}
	rows := []directory.Organization{}
	seen := map[string]bool{}
	expected := -1
	for page := 1; page <= 10000; page++ {
		body, err := reader.get(ctx, "/api/get-organizations", url.Values{"owner": {"admin"}, "p": {fmt.Sprint(page)}, "pageSize": {"100"}})
		if err != nil {
			return nil, fmt.Errorf("organization_discovery_failed")
		}
		var envelope struct {
			Status string `json:"status"`
		}
		if json.Unmarshal(body, &envelope) != nil || envelope.Status != "ok" {
			return nil, fmt.Errorf("organization_response_invalid")
		}
		items, total, err := parseList("/api/get-organizations", body)
		if err != nil {
			return nil, fmt.Errorf("organization_response_invalid")
		}
		if total >= 0 {
			if expected >= 0 && expected != total {
				return nil, fmt.Errorf("organization_total_changed")
			}
			expected = total
		}
		if len(items) == 0 {
			if expected >= 0 && len(rows) != expected {
				return nil, fmt.Errorf("organization_pagination_incomplete")
			}
			return rows, nil
		}
		for _, item := range items {
			name, owner := str(item, "name"), str(item, "owner")
			if name == "" || owner != "admin" || seen[name] {
				return nil, fmt.Errorf("organization_pagination_incomplete")
			}
			seen[name] = true
			rows = append(rows, directory.Organization{SourceID: owner + "/" + name, Name: name, DisplayName: str(item, "displayName")})
		}
		if expected >= 0 {
			if len(rows) > expected {
				return nil, fmt.Errorf("organization_total_invalid")
			}
			if len(rows) == expected {
				return rows, nil
			}
		}
	}
	return nil, fmt.Errorf("organization_pagination_limit")
}

// ValidateManagement verifies the credential source as well as built-in visibility.
func (c *OAuthClient) ValidateManagement(ctx context.Context) ([]directory.Organization, error) {
	reader, err := c.reader(ctx, "")
	if err != nil {
		return nil, err
	}
	body, err := reader.get(ctx, "/api/get-applications", url.Values{"owner": {"admin"}})
	if err != nil {
		return nil, fmt.Errorf("management_identity_unverified")
	}
	items, _, err := parseList("/api/get-applications", body)
	if err != nil {
		return nil, fmt.Errorf("management_identity_unverified")
	}
	matched := false
	for _, item := range items {
		if str(item, "clientId") == c.cfg.ClientID && str(item, "organization") == "built-in" {
			matched = true
		}
	}
	if !matched {
		return nil, fmt.Errorf("global_management_identity_required")
	}
	orgs, err := c.Organizations(ctx)
	if err != nil {
		return nil, err
	}
	for _, org := range orgs {
		if org.Name == "built-in" {
			return orgs, nil
		}
	}
	return nil, fmt.Errorf("global_management_identity_required")
}
