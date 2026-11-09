package application

import (
	"context"
	"mayfly-go/internal/casdoor/domain/entity"
)

func (s *ServiceManager) Applications(ctx context.Context, service, organization uint64, keyword string, page, size int) ([]entity.Application, int64, error) {
	org, err := s.Organization(ctx, service, organization, false)
	if err != nil {
		return nil, 0, err
	}
	return s.deps.Query.ListApplications(ctx, org.ConnectionID, keyword, page, size)
}
func (s *ServiceManager) Users(ctx context.Context, service, organization uint64, keyword string, page, size int) ([]entity.User, int64, error) {
	org, err := s.Organization(ctx, service, organization, false)
	if err != nil {
		return nil, 0, err
	}
	return s.deps.Query.ListUsers(ctx, org.ConnectionID, keyword, page, size)
}
func (s *ServiceManager) Objects(ctx context.Context, service, organization uint64, kind string, page, size int) ([]entity.DirectoryObject, int64, error) {
	org, err := s.Organization(ctx, service, organization, false)
	if err != nil {
		return nil, 0, err
	}
	return s.deps.Query.ListObjects(ctx, org.ConnectionID, kind, page, size)
}
func (s *ServiceManager) Operations(ctx context.Context, service, organization uint64, page, size int) ([]entity.Change, int64, error) {
	org, err := s.Organization(ctx, service, organization, false)
	if err != nil {
		return nil, 0, err
	}
	return s.deps.Operations.ListOperations(ctx, org.ConnectionID, page, size)
}
