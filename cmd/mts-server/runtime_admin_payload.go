package main

import "context"

func (r *serverRuntime) maintenanceStatsPayload() maintenanceStatsResponse {
	busy, op, started := r.adminHeavyState()
	return maintenanceStatsResponse{
		Stats:         r.maintenanceStats(),
		Path:          routeAdminStatsMaintenance,
		AdminOpBusy:   busy,
		Op:            op,
		StartedAtUnix: started,
		Last:          r.lastAdminHeavySnapshot(),
	}
}

func (r *serverRuntime) opsStatusPayload() opsStatusResponse {
	busy, op, started := r.adminHeavyState()
	return opsStatusResponse{
		Path:          routeAdminOpsStatus,
		AdminOpBusy:   busy,
		Op:            op,
		StartedAtUnix: started,
		Last:          r.lastAdminHeavySnapshot(),
	}
}

func (r *serverRuntime) adminHealthPayload() adminHealthResponse {
	busy, op, started := r.adminHeavyState()
	return adminHealthResponse{
		Health:        r.health(),
		Path:          routeAdminHealth,
		AdminOpBusy:   busy,
		Op:            op,
		StartedAtUnix: started,
		Last:          r.lastAdminHeavySnapshot(),
	}
}

func (r *serverRuntime) adminVersionPayload() versionResponse {
	busy, op, started := r.adminHeavyState()
	return versionResponse{
		Version:       version,
		Commit:        commit,
		BuiltAt:       builtAt,
		Path:          routeAdminVersion,
		AdminOpBusy:   busy,
		Op:            op,
		StartedAtUnix: started,
		Last:          r.lastAdminHeavySnapshot(),
	}
}

func (r *serverRuntime) storageMemoryPayload() storageMemoryResponse {
	busy, op, started := r.adminHeavyState()
	return storageMemoryResponse{
		Snapshot:      r.storageMemory(),
		Path:          routeAdminStatsStorageMemory,
		AdminOpBusy:   busy,
		Op:            op,
		StartedAtUnix: started,
		Last:          r.lastAdminHeavySnapshot(),
	}
}

func (r *serverRuntime) compactionStatsPayload() compactionStatsResponse {
	busy, op, started := r.adminHeavyState()
	return compactionStatsResponse{
		Stats:         r.compactionStats(),
		Path:          routeAdminStatsCompaction,
		AdminOpBusy:   busy,
		Op:            op,
		StartedAtUnix: started,
		Last:          r.lastAdminHeavySnapshot(),
	}
}

func (r *serverRuntime) attachAdminOpToSnapshots(resp storageSnapshotsResponse) storageSnapshotsResponse {
	busy, op, started := r.adminHeavyState()
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}

func (r *serverRuntime) attachAdminOpToDataSnapshots(resp storageDataSnapshotsResponse) storageDataSnapshotsResponse {
	busy, op, started := r.adminHeavyState()
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}

func (r *serverRuntime) storageExportPayload(ctx context.Context) storageExportResponse {
	busy, op, started := r.adminHeavyState()
	return storageExportResponse{
		Export:        r.storageExport(ctx),
		Path:          routeAdminStorageExport,
		AdminOpBusy:   busy,
		Op:            op,
		StartedAtUnix: started,
		Last:          r.lastAdminHeavySnapshot(),
	}
}

func (r *serverRuntime) configPayload() configResponse {
	busy, op, started := r.adminHeavyState()
	return configResponse{
		Config:        r.effectiveConfig(),
		Path:          routeAdminConfigEffective,
		AdminOpBusy:   busy,
		Op:            op,
		StartedAtUnix: started,
		Last:          r.lastAdminHeavySnapshot(),
	}
}

func (r *serverRuntime) configSchemaPayload() configSchemaResponse {
	busy, op, started := r.adminHeavyState()
	return configSchemaResponse{
		Fields:        configSchema(),
		Path:          routeAdminConfigSchema,
		AdminOpBusy:   busy,
		Op:            op,
		StartedAtUnix: started,
		Last:          r.lastAdminHeavySnapshot(),
	}
}

func (r *serverRuntime) apiSpecPayload() apiSpecResponse {
	busy, op, started := r.adminHeavyState()
	resp := apiSpec()
	resp.Path = routeAdminAPISpec
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}

func (r *serverRuntime) errorCodesPayload() errorCodesResponse {
	busy, op, started := r.adminHeavyState()
	resp := errorCodeSpecs()
	resp.Path = routeAdminErrorCodes
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}

func (r *serverRuntime) attachAdminOpToAudit(resp auditListResponse) auditListResponse {
	busy, op, started := r.adminHeavyState()
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}

func (r *serverRuntime) attachAdminOpToDownsamplePolicies(resp downsamplePoliciesResponse) downsamplePoliciesResponse {
	busy, op, started := r.adminHeavyState()
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}

func (r *serverRuntime) attachAdminOpToDownsamplePolicy(resp downsamplePolicyResponse) downsamplePolicyResponse {
	busy, op, started := r.adminHeavyState()
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}

func (r *serverRuntime) attachAdminOpToDownsampleStatuses(resp downsampleStatusesResponse) downsampleStatusesResponse {
	busy, op, started := r.adminHeavyState()
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}

func (r *serverRuntime) attachAdminOpToDownsamplePolicyStatus(resp downsamplePolicyStatusResponse) downsamplePolicyStatusResponse {
	busy, op, started := r.adminHeavyState()
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}

func (r *serverRuntime) attachAdminOpToUsers(resp usersResponse) usersResponse {
	busy, op, started := r.adminHeavyState()
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}

func (r *serverRuntime) attachAdminOpToMeasurements(resp measurementsResponse) measurementsResponse {
	busy, op, started := r.adminHeavyState()
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}

func (r *serverRuntime) attachAdminOpToDatabases(resp databasesResponse) databasesResponse {
	busy, op, started := r.adminHeavyState()
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}

func (r *serverRuntime) attachAdminOpToRetentionPolicies(resp retentionPoliciesResponse) retentionPoliciesResponse {
	busy, op, started := r.adminHeavyState()
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}

func (r *serverRuntime) attachAdminOpToPermissions(resp databasePermissionsResponse) databasePermissionsResponse {
	busy, op, started := r.adminHeavyState()
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}
