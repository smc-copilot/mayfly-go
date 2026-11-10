package persistence

import "mayfly-go/pkg/ioc"

func InitIoc() {
	ioc.Register(newInstanceRepo())
	ioc.Register(newCasdoorConnectionRepo())
	ioc.Register(newCasdoorAppRepo())
	ioc.Register(newCasdoorUserRepo())
	ioc.Register(newCasdoorAuthRepo())
	ioc.Register(newIdentityLinkRepo())
	ioc.Register(newEventRepo())
	ioc.Register(newCheckpointRepo())
	ioc.Register(newCollectorRunRepo())
	ioc.Register(newMetricSampleRepo())
	ioc.Register(newSettingsRepo())
	ioc.Register(newReadConnectionRepo())
}
