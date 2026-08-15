package modules_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestCompositionRoot(t *testing.T) {
	walkRuntimeModuleFiles(t, func(module, path string, file *ast.File) {
		for _, spec := range file.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil || !strings.HasPrefix(importPath, moduleImportPrefix) {
				continue
			}
			remainder := strings.TrimPrefix(importPath, moduleImportPrefix)
			parts := strings.Split(remainder, "/")
			if len(parts) >= 2 && parts[0] != module && parts[1] != "contract" {
				t.Errorf("%s: module %s must not compose module %s implementation", path, module, parts[0])
			}
		}
	})

	rootPath := filepath.Clean("../../initialize/router_biz.go")
	root, err := parser.ParseFile(token.NewFileSet(), rootPath, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	providers := map[string]bool{"reader/provider": false, "commerce/provider": false, "novel/provider": false}
	for _, spec := range root.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		for providerPath := range providers {
			if strings.HasSuffix(importPath, "/internal/modules/"+providerPath) {
				providers[providerPath] = true
			}
		}
	}
	for providerPath, imported := range providers {
		if !imported {
			t.Errorf("%s: composition root must import %s", rootPath, providerPath)
		}
	}
}

func TestProviderImplementationsAreNotComposedInsideBusinessModules(t *testing.T) {
	walkRuntimeModuleFiles(t, func(module, path string, file *ast.File) {
		aliases := map[string]string{}
		for _, spec := range file.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil || !strings.HasPrefix(importPath, moduleImportPrefix) || !strings.HasSuffix(importPath, "/provider") {
				continue
			}
			name := filepath.Base(importPath)
			if spec.Name != nil {
				name = spec.Name.Name
			}
			aliases[name] = importPath
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !strings.HasPrefix(selector.Sel.Name, "New") {
				return true
			}
			identifier, ok := selector.X.(*ast.Ident)
			if ok && aliases[identifier.Name] != "" {
				t.Errorf("%s: module %s must not construct provider %s", path, module, aliases[identifier.Name])
			}
			return true
		})
	})
}
