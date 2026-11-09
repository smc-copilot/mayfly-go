package integration

import "context"

type ResourceQuery struct {
	Keyword  string
	PageNum  int
	PageSize int
}

type ResourceRef struct {
	System      string
	ResourceID  string
	DisplayName string
	URL         string
}

type ResourcePage struct {
	List  []ResourceRef
	Total int64
}

type BindingCommand struct {
	ConnectionID  uint64
	AppSnapshotID uint64
	ResourceID    string
	Action        string
}

type Adapter interface {
	Key() string
	ListResources(ctx context.Context, query ResourceQuery) (*ResourcePage, error)
	ListBindings(ctx context.Context, connectionID, appSnapshotID uint64) ([]ResourceRef, error)
	ChangeBinding(ctx context.Context, command BindingCommand) error
}
