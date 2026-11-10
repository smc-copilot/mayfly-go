package api

import "mayfly-go/pkg/ioc"

func InitIoc() {
	ioc.Register(new(Instance))
	ioc.Register(new(Casdoor))
	ioc.Register(new(Event))
	ioc.Register(new(Business))
	ioc.Register(new(Library))
	ioc.Register(new(ReadConnection))
}
