package application

import (
	"context"
	"testing"

	"mayfly-go/internal/casdoor/domain/integration"
)

type fakeAdapter struct{ key string }

func (a *fakeAdapter) Key() string { return a.key }
func (a *fakeAdapter) ListResources(context.Context, integration.ResourceQuery) (*integration.ResourcePage, error) {
	return &integration.ResourcePage{}, nil
}
func (a *fakeAdapter) ListBindings(context.Context, uint64, uint64) ([]integration.ResourceRef, error) {
	return nil, nil
}
func (a *fakeAdapter) ChangeBinding(context.Context, integration.BindingCommand) error {
	return nil
}

func TestIntegrationRegistryRejectsDuplicateAndSupportsEmpty(t *testing.T) {
	registry := NewIntegrationRegistry()
	if len(registry.Keys()) != 0 {
		t.Fatal("没有注册适配器时，集成列表应为空")
	}
	first := &fakeAdapter{key: "business"}
	if err := registry.Register(first); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(&fakeAdapter{key: "business"}); err == nil {
		t.Fatal("重复集成类型必须报错")
	}
	if _, ok := registry.Get("missing"); ok {
		t.Fatal("未注册适配器不得返回成功")
	}
	if got, ok := registry.Get("business"); !ok || got != first {
		t.Fatal("重复注册不得替换原适配器")
	}
}

func TestIntegrationRegistryRejectsNilAdapter(t *testing.T) {
	registry := NewIntegrationRegistry()
	var adapter *fakeAdapter
	if err := registry.Register(adapter); err == nil {
		t.Fatal("空适配器必须报错")
	}
}
