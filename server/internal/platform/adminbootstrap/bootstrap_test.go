package adminbootstrap

import (
	"context"
	"database/sql"
	"testing"
)

func TestBootstrapValidatesInputBeforeDatabaseAccess(t *testing.T) {
	tests := []struct {
		name    string
		options Options
	}{
		{name: "empty username", options: Options{Password: "long-enough-password"}},
		{name: "username whitespace", options: Options{Username: "moon book", Password: "long-enough-password"}},
		{name: "short password", options: Options{Username: "admin", Password: "too-short"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Bootstrap(context.Background(), &sql.DB{}, tt.options); err == nil {
				t.Fatal("Bootstrap returned nil error")
			}
		})
	}
}
