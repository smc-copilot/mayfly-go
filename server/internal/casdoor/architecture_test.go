package casdoor_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCasdoorCoreDoesNotDependOnBusinessModules(t *testing.T) {
	if _, err := os.Stat(filepath.Join("init", "init.go")); err != nil {
		t.Fatalf("独立 Casdoor 模块必须提供初始化入口: %v", err)
	}
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imported := range file.Imports {
			if strings.Contains(imported.Path.Value, "mayfly-go/internal/octop") {
				t.Errorf("%s 不能依赖 Octop: %s", path, imported.Path.Value)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
