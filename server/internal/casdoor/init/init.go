package init

import (
	"mayfly-go/internal/casdoor/api"
	"mayfly-go/internal/casdoor/application"
	"mayfly-go/internal/casdoor/domain/directory"
	"mayfly-go/internal/casdoor/infra/client"
	"mayfly-go/internal/casdoor/infra/persistence"
	"mayfly-go/internal/casdoor/infra/secret"
	"mayfly-go/internal/pkg/config"
	"mayfly-go/pkg/ioc"
	"mayfly-go/pkg/starter"

	"gorm.io/gorm"
)

func init() {
	starter.AddInitIocFunc(func() {
		ioc.Register(application.NewIntegrationRegistry())
	})
}

// BuildDirectoryAPI 创建独立模块的只读目录服务；调用方决定何时注册路由。
func BuildDirectoryAPI(db *gorm.DB, aes config.Aes) *api.DirectoryAPI {
	connections := persistence.NewConnectionRepository(db)
	scopes := persistence.NewScopeRepository(db)
	access := application.NewScopeAuthorizer(scopes)
	codec := secret.NewAESCodec(aes)
	return &api.DirectoryAPI{
		ListService: application.NewConnectionListService(persistence.NewConnectionQuery(db)),
		Connections: application.NewConnectionService(connections, codec, access),
		Tester:      application.NewConnectionTester(connections, codec, access, client.Factory{}),
		Syncer:      application.NewSyncService(connections, codec, access, persistence.NewSnapshotRepository(db), persistence.NewSyncStateRepository(db), persistence.NewObjectSnapshotRepository(db), persistence.NewObjectSyncStateRepository(db), application.NewConnectionLocks(), func(cfg directory.ProbeConfig) directory.Reader { return client.New(cfg) }),
		Directory:   application.NewDirectoryQueryService(persistence.NewDirectoryQuery(db), access),
		Operations:  application.NewOperationQueryService(persistence.NewOperationQuery(db), access),
	}
}
