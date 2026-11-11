package langfuse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"mayfly-go/internal/octop/domain/entity"
)

const (
	ModeV4      = "v4"
	ModeClassic = "classic"
)

type Verification struct {
	Version     string
	APIMode     string
	ProjectId   string
	ProjectName string
	Checks      []entity.ReadConnectionCheck
}

// Verify authenticates the key, validates its real project and tests the read
// protocol. Only successful verification is cached, for this client instance.
func (c *Client) Verify(ctx context.Context) (*Verification, error) {
	c.verifyMu.Lock()
	defer c.verifyMu.Unlock()
	if c.verified != nil {
		copy := *c.verified
		return &copy, nil
	}
	result := &Verification{}
	steps := []struct {
		name string
		run  func() error
	}{
		{"health", func() error { return c.verifyHealth(ctx, result) }},
		{"auth", func() error { return c.verifyProject(ctx, result) }},
		{"project", func() error {
			if c.expectedProject != "" && c.expectedProject != result.ProjectId {
				return ErrProject
			}
			return nil
		}},
		{"read", func() error { return c.verifyRead(ctx, result.APIMode) }},
	}
	for _, step := range steps {
		err := step.run()
		state := entity.SourceStateReady
		if err != nil {
			state = entity.SourceStateFailed
		}
		result.Checks = append(result.Checks, entity.ReadConnectionCheck{Name: step.name, State: state})
		if err != nil {
			return result, err
		}
	}
	c.verified = result
	copy := *result
	return &copy, nil
}

func (c *Client) verifyHealth(ctx context.Context, result *Verification) error {
	if c.cfgErr != nil {
		return c.cfgErr
	}
	body, status, err := c.do(ctx, http.MethodGet, "/api/public/health", nil)
	if err != nil {
		return err
	}
	if err := mapStatus(status); err != nil {
		return err
	}
	var response struct {
		Status  string `json:"status"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("%w: health response", ErrUnsupported)
	}
	if !strings.EqualFold(response.Status, "OK") {
		return fmt.Errorf("%w: health status", ErrUnsupported)
	}
	result.Version = response.Version
	major, err := strconv.Atoi(strings.Split(strings.TrimPrefix(response.Version, "v"), ".")[0])
	if err != nil {
		return fmt.Errorf("%w: missing version", ErrUnsupported)
	}
	switch major {
	case 4:
		result.APIMode = ModeV4
	case 2, 3:
		result.APIMode = ModeClassic
	default:
		return fmt.Errorf("%w: server version", ErrUnsupported)
	}
	return nil
}

func (c *Client) verifyProject(ctx context.Context, result *Verification) error {
	body, status, err := c.do(ctx, http.MethodGet, "/api/public/projects", nil)
	if err != nil {
		return err
	}
	if err := mapStatus(status); err != nil {
		return err
	}
	var response struct {
		Data []struct {
			Id   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("%w: project response", ErrProject)
	}
	// A project API key must resolve to one project; do not guess by display name.
	if len(response.Data) != 1 || response.Data[0].Id == "" {
		return ErrProject
	}
	result.ProjectId, result.ProjectName = response.Data[0].Id, response.Data[0].Name
	return nil
}

func (c *Client) verifyRead(ctx context.Context, mode string) error {
	path := "/api/public/traces?limit=1"
	if mode == ModeV4 {
		end := time.Now().UTC()
		params := url.Values{"limit": {"1"}, "fields": {"core,basic"}, "fromStartTime": {end.Add(-15 * time.Minute).Format(time.RFC3339Nano)}, "toStartTime": {end.Format(time.RFC3339Nano)}}
		path = "/api/public/v2/observations?" + params.Encode()
	}
	body, status, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	if err := mapStatus(status); err != nil {
		return err
	}
	var response struct {
		Data []json.RawMessage `json:"data"`
		Meta *struct {
			Cursor *string `json:"cursor"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("%w: read response", ErrUnsupported)
	}
	if response.Data == nil || (mode == ModeV4 && response.Meta == nil) {
		return fmt.Errorf("%w: read envelope", ErrUnsupported)
	}
	return nil
}
