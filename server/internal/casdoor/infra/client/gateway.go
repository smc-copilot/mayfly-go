package client

import (
	"context"
	"crypto/sha256"
	"fmt"
	"mayfly-go/internal/casdoor/domain/directory"
	"sync"
)

// OAuthGateway shares token caches without storing plaintext credentials in map keys.
type OAuthGateway struct {
	mu      sync.Mutex
	clients map[[32]byte]*OAuthClient
}

func NewOAuthGateway() *OAuthGateway { return &OAuthGateway{clients: map[[32]byte]*OAuthClient{}} }
func (g *OAuthGateway) client(cfg directory.ProbeConfig) *OAuthClient {
	key := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%s\x00%t\x00%d", cfg.ServerURL, cfg.ClientID, cfg.ClientSecret, cfg.TLSVerify, cfg.Timeout)))
	g.mu.Lock()
	defer g.mu.Unlock()
	if c := g.clients[key]; c != nil {
		return c
	}
	// Bound credential generations retained in memory.
	if len(g.clients) >= 128 {
		g.clients = map[[32]byte]*OAuthClient{}
	}
	c := NewOAuthClient(cfg)
	g.clients[key] = c
	return c
}
func (g *OAuthGateway) Validate(ctx context.Context, cfg directory.ProbeConfig) ([]directory.Organization, error) {
	return g.client(cfg).ValidateManagement(ctx)
}
func (g *OAuthGateway) Reader(ctx context.Context, cfg directory.ProbeConfig, org string) (directory.Reader, error) {
	return g.client(cfg).Reader(ctx, org)
}
