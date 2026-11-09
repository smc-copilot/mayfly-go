package repository

import (
	"context"
	"mayfly-go/internal/casdoor/domain/directory"
	"mayfly-go/internal/casdoor/domain/entity"
)

type ServiceStore interface {
	List(context.Context, uint64) ([]entity.Service, error)
	Get(context.Context, uint64) (*entity.Service, error)
	Credential(context.Context, uint64) (*entity.ManagementCredential, error)
	Scope(context.Context, uint64, uint64) (string, error)
	Save(context.Context, *entity.Service, *entity.ManagementCredential, []directory.Organization, uint64) error
	Discover(context.Context, *entity.Service, []directory.Organization) error
	Organizations(context.Context, uint64) ([]entity.ServiceOrganization, error)
	Organization(context.Context, uint64, uint64) (*entity.ServiceOrganization, error)
	Failure(context.Context, uint64, uint64, string) error
}
