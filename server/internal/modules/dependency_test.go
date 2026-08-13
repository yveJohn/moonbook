package modules_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const moduleImportPrefix = "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/"

func TestModuleDependencyRules(t *testing.T) {
	root := "."
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		module := entry.Name()
		err := filepath.WalkDir(filepath.Join(root, module), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, spec := range file.Imports {
				checkImport(t, module, path, spec)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func checkImport(t *testing.T, module, path string, spec *ast.ImportSpec) {
	t.Helper()
	importPath, err := strconv.Unquote(spec.Path.Value)
	if err != nil {
		t.Fatalf("%s: invalid import: %v", path, err)
	}
	for _, forbidden := range []string{
		"github.com/flipped-aurora/gin-vue-admin/server/global",
		"github.com/flipped-aurora/gin-vue-admin/server/model/",
		"github.com/flipped-aurora/gin-vue-admin/server/service/",
	} {
		if importPath == strings.TrimSuffix(forbidden, "/") || strings.HasPrefix(importPath, forbidden) {
			t.Errorf("%s: module %s must not import GVA implementation package %s", path, module, importPath)
		}
	}
	if !strings.HasPrefix(importPath, moduleImportPrefix) {
		return
	}
	remainder := strings.TrimPrefix(importPath, moduleImportPrefix)
	other := strings.SplitN(remainder, "/", 2)[0]
	contractPath := other + "/contract"
	usesContract := remainder == contractPath || strings.HasPrefix(remainder, contractPath+"/")
	if other != module && !usesContract {
		t.Errorf("%s: module %s may only use module %s through its contract package", path, module, other)
	}
}
