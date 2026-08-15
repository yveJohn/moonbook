package contract

import (
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestCommerceContractImportsOnlyStandardBoundaryTypes(t *testing.T) {
	assertCommerceContractImports(t, map[string]bool{"context": true, "errors": true, "time": true})
}

func TestCommerceContractDTOsKeepLongIDsAsInt64(t *testing.T) {
	for name, typ := range map[string]reflect.Type{
		"ReaderSearchProjection": reflect.TypeOf(ReaderSearchProjection{}),
		"AccessRequest":          reflect.TypeOf(AccessRequest{}),
		"PurchaseOrder":          reflect.TypeOf(PurchaseOrder{}),
		"RechargeOrder":          reflect.TypeOf(RechargeOrder{}),
		"Wallet":                 reflect.TypeOf(Wallet{}),
	} {
		assertPortableCommerceDTO(t, name, typ, map[reflect.Type]bool{})
	}
	if reflect.TypeOf(ReaderSearchProjection{}).Field(0).Type.Kind() != reflect.Int64 {
		t.Fatal("ReaderSearchProjection.ReaderID must remain int64")
	}
}

func assertCommerceContractImports(t *testing.T, allowed map[string]bool) {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range file.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil || !allowed[path] {
				t.Errorf("%s imports boundary-forbidden package %q", entry.Name(), path)
			}
		}
	}
}

func TestCommerceContractErrorsAreStableAndSanitized(t *testing.T) {
	cause := errors.New("duplicate key on reader_wallets using postgres://secret")
	err := Wrap(ErrUnavailable, cause)
	if !errors.Is(err, ErrUnavailable) || Cause(err) != cause {
		t.Fatalf("classification=%v cause=%v", err, Cause(err))
	}
	for _, secret := range []string{"duplicate key", "reader_wallets", "postgres://", "secret"} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("contract error leaked %q: %v", secret, err)
		}
	}
}

func assertPortableCommerceDTO(t *testing.T, name string, typ reflect.Type, seen map[reflect.Type]bool) {
	t.Helper()
	if typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Array {
		assertPortableCommerceDTO(t, name, typ.Elem(), seen)
		return
	}
	if typ.Kind() != reflect.Struct || seen[typ] {
		return
	}
	seen[typ] = true
	if path := typ.PkgPath(); strings.Contains(path, "/gin-gonic/") || strings.Contains(path, "/model/") || strings.Contains(path, "/service/") || path == "database/sql" {
		t.Fatalf("%s contains implementation type %s", name, typ)
	}
	for i := 0; i < typ.NumField(); i++ {
		assertPortableCommerceDTO(t, name+"."+typ.Field(i).Name, typ.Field(i).Type, seen)
	}
}
