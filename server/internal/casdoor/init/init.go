package init

import (
	"mayfly-go/internal/casdoor/application"
	"mayfly-go/pkg/ioc"
	"mayfly-go/pkg/starter"
)

func init() {
	starter.AddInitIocFunc(func() {
		ioc.Register(application.NewIntegrationRegistry())
	})
}
