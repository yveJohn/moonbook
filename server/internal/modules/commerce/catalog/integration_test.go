//go:build integration

package catalog

import "testing"

// Database-backed coverage is run by the repository integration suite when
// the project PostgreSQL container is available. Unit tests keep the decision
// matrix deterministic and do not require infrastructure.
func TestCatalogIntegrationPlaceholder(t *testing.T) {
	t.Skip("requires project PostgreSQL integration harness")
}
