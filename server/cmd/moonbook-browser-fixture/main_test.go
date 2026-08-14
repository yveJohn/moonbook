package main

import "testing"

func TestConfigValidateRequiresExplicitLocalTarget(t *testing.T) {
	valid := config{
		dsn:      "postgres://moonbook:secret@127.0.0.1:25488/moonbook?sslmode=disable",
		endpoint: "127.0.0.1:29488", accessKey: "moonbook", secretKey: "secret",
		bucket: "moonbook-content", confirmation: fixtureConfirmation,
	}
	if err := valid.validate(); err != nil {
		t.Fatalf("valid local target rejected: %v", err)
	}
	checks := []struct {
		name string
		edit func(*config)
	}{
		{"confirmation", func(c *config) { c.confirmation = "" }},
		{"database host", func(c *config) { c.dsn = "postgres://moonbook:secret@db.example.com/moonbook" }},
		{"MinIO host", func(c *config) { c.endpoint = "minio.example.com:9000" }},
		{"missing secret", func(c *config) { c.secretKey = "" }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			candidate := valid
			check.edit(&candidate)
			if err := candidate.validate(); err == nil {
				t.Fatal("unsafe fixture target accepted")
			}
		})
	}
}
