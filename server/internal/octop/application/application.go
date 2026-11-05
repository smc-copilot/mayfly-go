package application

import (
	"mayfly-go/pkg/ioc"
)

func InitIoc() {
	ioc.Register(new(instanceAppImpl))
	ioc.Register(new(casdoorAppImpl))
	ioc.Register(new(eventAppImpl))
	ioc.Register(new(collectorAppImpl))
	ioc.Register(new(settingsAppImpl))
}

func Init() {
	GetInstanceApp().InitMetricCron()
	GetCasdoorApp().InitCron()
	GetCollectorApp().InitCron()
}

func GetInstanceApp() *instanceAppImpl {
	return ioc.Get[*instanceAppImpl]()
}

func GetCasdoorApp() Casdoor {
	return ioc.Get[Casdoor]()
}

func GetEventApp() Event {
	return ioc.Get[Event]()
}

func GetCollectorApp() Collector {
	return ioc.Get[Collector]()
}

func GetSettingsApp() Settings {
	return ioc.Get[Settings]()
}
