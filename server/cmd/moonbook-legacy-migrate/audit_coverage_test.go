package main

import (
	"testing"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/legacyaudit"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/legacymigrate"
)

func TestAllMigrationStagesHaveAuditContracts(t *testing.T) {
	contracts := legacyaudit.StageAuditContracts()
	stages := append([]legacymigrate.Stage{legacymigrate.PreflightStage{}}, allMigrationStages(nil, nil, legacymigrate.NovelTXTImportsStage{}, legacymigrate.ReaderIdentityStage{})...)
	registered := make(map[string]bool, len(stages))
	for _, stage := range stages {
		name := stage.Name()
		registered[name] = true
		if len(contracts[name]) == 0 {
			t.Errorf("registered migration stage %q has no audit contract", name)
		}
	}
	for name := range contracts {
		if !registered[name] {
			t.Errorf("audit contract %q has no registered migration stage", name)
		}
	}
}
