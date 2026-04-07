package db

import "testing"

func TestStartTenantSyncPreventsDuplicateRuns(t *testing.T) {
	tenantID := int64(999)
	FinishTenantSync(tenantID)

	if !StartTenantSync(tenantID) {
		t.Fatal("first sync start should succeed")
	}
	if StartTenantSync(tenantID) {
		t.Fatal("second sync start should be rejected while running")
	}

	FinishTenantSync(tenantID)

	if !StartTenantSync(tenantID) {
		t.Fatal("sync should be startable again after finish")
	}
	FinishTenantSync(tenantID)
}
