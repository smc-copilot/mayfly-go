package init

import (
	"mayfly-go/internal/octop/api"
	"mayfly-go/internal/octop/application"
	"mayfly-go/internal/octop/infra/persistence"
	"mayfly-go/pkg/starter"
)

func init() {
	starter.AddInitIocFunc(func() {
		persistence.InitIoc()
		application.InitIoc()
		api.InitIoc()
	})
	starter.AddInitFunc(application.Init)
}
