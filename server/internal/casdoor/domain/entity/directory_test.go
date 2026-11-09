package entity

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDirectoryEntitiesUseIndependentTables(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"connection", (&Connection{}).TableName(), "t_casdoor_connection"},
		{"application", (&Application{}).TableName(), "t_casdoor_application"},
		{"user", (&User{}).TableName(), "t_casdoor_user"},
		{"relation", (&AuthRelation{}).TableName(), "t_casdoor_auth_relation"},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s table = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}

func TestConnectionJSONDoesNotExposeCredential(t *testing.T) {
	connection := Connection{ClientSecretEnc: "secret-value", Certificate: "private-material"}
	data, err := json.Marshal(connection)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret-value") || strings.Contains(string(data), "private-material") {
		t.Fatal("连接响应不得包含凭证或证书材料")
	}
}
