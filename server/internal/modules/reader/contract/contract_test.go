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

func TestReaderContractImportsOnlyStandardBoundaryTypes(t *testing.T) {
	assertReaderContractImports(t, map[string]bool{"context": true, "errors": true})
}

func TestReaderContractDTOsKeepLongIDsAsInt64(t *testing.T) {
	for name, typ := range map[string]reflect.Type{
		"Account":        reflect.TypeOf(Account{}),
		"AccountDisplay": reflect.TypeOf(AccountDisplay{}),
		"InviteRelation": reflect.TypeOf(InviteRelation{}),
	} {
		assertPortableReaderDTO(t, name, typ, map[reflect.Type]bool{})
	}
	if reflect.TypeOf(Account{}).Field(0).Type.Kind() != reflect.Int64 {
		t.Fatal("Account.ID must remain int64 inside the domain contract")
	}
}

func assertReaderContractImports(t *testing.T, allowed map[string]bool) {
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

func TestReaderContractErrorsAreStableAndSanitized(t *testing.T) {
	cause := errors.New("SELECT password_hash FROM reader_accounts at postgres://secret")
	err := Wrap(ErrUnavailable, cause)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("error classification=%v", err)
	}
	if Cause(err) != cause {
		t.Fatal("wrapped cause was not retained for provider-boundary logging")
	}
	for _, secret := range []string{"SELECT", "reader_accounts", "postgres://", "secret"} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("contract error leaked %q: %v", secret, err)
		}
	}
}

func assertPortableReaderDTO(t *testing.T, name string, typ reflect.Type, seen map[reflect.Type]bool) {
	t.Helper()
	if typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Array {
		assertPortableReaderDTO(t, name, typ.Elem(), seen)
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
		assertPortableReaderDTO(t, name+"."+typ.Field(i).Name, typ.Field(i).Type, seen)
	}
}
