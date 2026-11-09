package directory

import (
	"context"
	"time"
)

// Application 和 User 是源目录数据，供接口适配器及应用层使用。
type Application struct {
	ClientID     string
	SourceID     string
	Name         string
	DisplayName  string
	Organization string
	Owner        string
	Enabled      bool
	EnabledKnown bool
	AppType      string
	HomepageURL  string
}

type User struct {
	ObjectID     string
	SourceID     string
	Username     string
	DisplayName  string
	Organization string
	Email        string
	Enabled      bool
	EnabledKnown bool
	UpdatedAt    string
}

type Object struct {
	SourceID   string
	Projection string
}

type Reader interface {
	ListApplications(ctx context.Context, page, pageSize int) ([]Application, int, error)
	ListUsers(ctx context.Context, page, pageSize int) ([]User, int, error)
}

type ProbeConfig struct {
	ServerURL    string
	Organization string
	AppOwner     string
	ClientID     string
	ClientSecret string
	Timeout      time.Duration
	TLSVerify    bool
	BearerToken  string
}

type Organization struct {
	SourceID    string `json:"sourceId"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

type Prober interface {
	TestService(context.Context) error
	TestApps(context.Context) error
	TestUsers(context.Context) error
}

type ProbeFactory interface {
	NewProbe(config ProbeConfig) Prober
}
