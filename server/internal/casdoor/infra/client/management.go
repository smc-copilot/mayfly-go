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
)

// ManagementClient implements only operations with a verified Casdoor contract.
type ManagementClient struct{ client *httpClient }

type RoleDetail struct {
	Owner       string   `json:"owner"`
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName"`
	Description string   `json:"description"`
	Users       []string `json:"users"`
	Groups      []string `json:"groups"`
	Roles       []string `json:"roles"`
	Domains     []string `json:"domains"`
	IsEnabled   bool     `json:"isEnabled"`
}

type UserPatch struct {
	DisplayName *string
	Email       *string
}

// ApplicationIdentity is read from a verified snapshot, never from an untrusted patch.
type ApplicationIdentity struct {
	Owner        string
	Name         string
	Organization string
	ClientID     string
}

type ApplicationPatch struct {
	DisplayName *string
}

func (c *ManagementClient) PatchApplication(ctx context.Context, app ApplicationIdentity, patch ApplicationPatch) error {
	if app.Organization != c.client.cfg.Organization || app.Owner == "" || app.Name == "" || app.ClientID == "" || strings.ContainsAny(app.Owner+app.Name, "/") {
		return fmt.Errorf("invalid application identity")
	}
	if patch.DisplayName == nil || strings.TrimSpace(*patch.DisplayName) == "" {
		return fmt.Errorf("display name required")
	}
	body := map[string]any{"owner": app.Owner, "name": app.Name, "organization": app.Organization, "clientId": app.ClientID, "displayName": *patch.DisplayName}
	return c.post(ctx, "/api/update-application", url.Values{"id": {app.Owner + "/" + app.Name}, "columns": {"displayName"}}, body)
}

type UserDetail struct {
	Owner       string `json:"owner"`
	Name        string `json:"name"`
	Id          string `json:"id"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	IsForbidden *bool  `json:"isForbidden"`
}

func (c *ManagementClient) GetUser(ctx context.Context, userID string) (*UserDetail, error) {
	if !sameOrganization(userID, c.client.cfg.Organization) {
		return nil, fmt.Errorf("invalid user identity")
	}
	body, err := c.client.get(ctx, "/api/get-user", url.Values{"id": {userID}})
	if err != nil {
		return nil, err
	}
	var response struct {
		Data *UserDetail `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("invalid user response")
	}
	if response.Data != nil && response.Data.Owner+"/"+response.Data.Name != userID {
		return nil, fmt.Errorf("unexpected user identity")
	}
	return response.Data, nil
}

func (c *ManagementClient) PatchUser(ctx context.Context, userID string, patch UserPatch) error {
	if !sameOrganization(userID, c.client.cfg.Organization) {
		return fmt.Errorf("invalid user identity")
	}
	body := map[string]any{}
	columns := []string{}
	if patch.DisplayName != nil {
		if strings.TrimSpace(*patch.DisplayName) == "" {
			return fmt.Errorf("display name required")
		}
		body["displayName"] = *patch.DisplayName
		columns = append(columns, "displayName")
	}
	if patch.Email != nil {
		body["email"] = *patch.Email
		columns = append(columns, "email")
	}
	if len(columns) == 0 {
		return fmt.Errorf("empty user patch")
	}
	return c.post(ctx, "/api/update-user", url.Values{"id": {userID}, "columns": {strings.Join(columns, ",")}}, body)
}

func NewManagementClient(cfg Config) *ManagementClient {
	return &ManagementClient{client: New(cfg).(*httpClient)}
}

func (c *ManagementClient) GetRole(ctx context.Context, roleID string) (*RoleDetail, error) {
	if !sameOrganization(roleID, c.client.cfg.Organization) {
		return nil, fmt.Errorf("invalid role identity")
	}
	body, err := c.client.get(ctx, "/api/get-role", url.Values{"id": {roleID}})
	if err != nil {
		return nil, err
	}
	var response struct {
		Data *RoleDetail `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("invalid role response")
	}
	if response.Data != nil && response.Data.Owner+"/"+response.Data.Name != roleID {
		return nil, fmt.Errorf("unexpected role identity")
	}
	return response.Data, nil
}

// WriteUnknownError means a request may have reached Casdoor; callers must reconcile by reading back.
type WriteUnknownError struct{}

func (WriteUnknownError) Error() string { return "remote write outcome is unknown" }

func (c *ManagementClient) ReplaceRoleUsers(ctx context.Context, roleID string, users []string) error {
	if !sameOrganization(roleID, c.client.cfg.Organization) || len(users) > 100 {
		return fmt.Errorf("invalid role update")
	}
	for _, user := range users {
		if !sameOrganization(user, c.client.cfg.Organization) {
			return fmt.Errorf("invalid role member")
		}
	}
	if users == nil {
		users = []string{}
	}
	return c.post(ctx, "/api/update-role", url.Values{"id": {roleID}, "columns": {"users"}}, map[string]any{"users": users})
}

func sameOrganization(id, organization string) bool {
	parts := strings.Split(id, "/")
	return len(parts) == 2 && parts[0] != "" && parts[0] == organization && parts[1] != ""
}

func (c *ManagementClient) post(ctx context.Context, path string, query url.Values, body any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	endpoint := strings.TrimRight(c.client.cfg.ServerURL, "/") + path + "?" + query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("invalid management request")
	}
	req.SetBasicAuth(c.client.cfg.ClientID, c.client.cfg.ClientSecret)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.client.Do(req)
	if err != nil {
		return WriteUnknownError{}
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return WriteUnknownError{}
	}
	if resp.StatusCode >= 500 || resp.StatusCode == http.StatusRequestTimeout || (resp.StatusCode >= 300 && resp.StatusCode < 400) {
		return WriteUnknownError{}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("casdoor management request rejected (HTTP %d)", resp.StatusCode)
	}
	var result struct {
		Status string `json:"status"`
		Data   string `json:"data"`
	}
	if json.Unmarshal(data, &result) != nil {
		return WriteUnknownError{}
	}
	if result.Status != "ok" {
		return fmt.Errorf("casdoor management request rejected")
	}
	if result.Data != "Affected" {
		return fmt.Errorf("casdoor management request had no effect")
	}
	return nil
}
