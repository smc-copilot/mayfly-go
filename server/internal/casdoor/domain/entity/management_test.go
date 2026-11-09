package entity

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"gorm.io/gorm/schema"
)

func TestManagementUniquenessIncludesConnection(t *testing.T) {
	cases := []struct {
		value  any
		index  string
		fields []string
	}{
		{&DirectoryObject{}, "uk_casdoor_object", []string{"connection_id", "kind", "source_id"}},
		{&ConnectionScope{}, "uk_casdoor_scope", []string{"account_id", "connection_id"}},
		{&ManagedApplication{}, "uk_casdoor_managed_app", []string{"connection_id", "application_source_id"}},
		{&Change{}, "uk_casdoor_change", []string{"connection_id", "idempotency_key"}},
	}
	for _, tc := range cases {
		parsed, err := schema.Parse(tc.value, &sync.Map{}, schema.NamingStrategy{})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, index := range parsed.ParseIndexes() {
			if index.Name != tc.index {
				continue
			}
			found = true
			if index.Class != "UNIQUE" || len(index.Fields) != len(tc.fields) {
				t.Fatalf("%s 必须是连接隔离的复合唯一索引", tc.index)
			}
			for i, field := range index.Fields {
				if field.DBName != tc.fields[i] {
					t.Fatalf("%s field %d = %s, want %s", tc.index, i, field.DBName, tc.fields[i])
				}
			}
		}
		if !found {
			t.Fatalf("缺少索引 %s", tc.index)
		}
	}
}

func TestChangeJSONDoesNotExposeInternalCommand(t *testing.T) {
	change := Change{Command: "private-command", SourceFingerprint: "internal-fingerprint"}
	data, err := json.Marshal(change)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "private-command") || strings.Contains(string(data), "internal-fingerprint") {
		t.Fatal("变更记录的内部命令和指纹不得直接返回")
	}
}
