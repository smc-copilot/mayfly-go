package application

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"

	"mayfly-go/internal/casdoor/domain/integration"
)

// IntegrationRegistry 管理可选的业务资源适配器；没有适配器时仍可使用 Casdoor。
type IntegrationRegistry struct {
	mu       sync.RWMutex
	adapters map[string]integration.Adapter
}

func NewIntegrationRegistry() *IntegrationRegistry {
	return &IntegrationRegistry{adapters: make(map[string]integration.Adapter)}
}

func (r *IntegrationRegistry) Register(adapter integration.Adapter) error {
	if adapter == nil {
		return fmt.Errorf("integration adapter requires a key")
	}
	value := reflect.ValueOf(adapter)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if value.IsNil() {
			return fmt.Errorf("integration adapter requires a key")
		}
	}
	if strings.TrimSpace(adapter.Key()) == "" {
		return fmt.Errorf("integration adapter requires a key")
	}
	key := adapter.Key()
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.adapters[key]; exists {
		return fmt.Errorf("integration adapter %q already registered", key)
	}
	r.adapters[key] = adapter
	return nil
}

func (r *IntegrationRegistry) Get(key string) (integration.Adapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	adapter, ok := r.adapters[key]
	return adapter, ok
}

func (r *IntegrationRegistry) Keys() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	keys := make([]string, 0, len(r.adapters))
	for key := range r.adapters {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
