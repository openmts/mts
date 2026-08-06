package main

func (r *serverRuntime) attachAdminOpToFields(resp fieldsResponse) fieldsResponse {
	busy, op, started := r.adminHeavyState()
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}

func (r *serverRuntime) attachAdminOpToSeries(resp seriesResponse) seriesResponse {
	busy, op, started := r.adminHeavyState()
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}

func (r *serverRuntime) queryStatsPayload() queryStatsResponse {
	busy, op, started := r.adminHeavyState()
	return queryStatsResponse{
		Stats:         r.queryStats(),
		Path:          routeDataQueryStats,
		AdminOpBusy:   busy,
		Op:            op,
		StartedAtUnix: started,
		Last:          r.lastAdminHeavySnapshot(),
	}
}

func (r *serverRuntime) attachAdminOpToSession(resp sessionResponse) sessionResponse {
	busy, op, started := r.adminHeavyState()
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}

func (r *serverRuntime) storageValidatePayload() storageValidateResponse {
	busy, op, started := r.adminHeavyState()
	resp := r.storageValidate()
	resp.Path = routeAdminStorageValidate
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}

func (r *serverRuntime) attachAdminOpToMaintenance(resp maintenanceResponse) maintenanceResponse {
	busy, op, started := r.adminHeavyState()
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}

func (r *serverRuntime) attachAdminOpToStorageSnapshot(resp storageSnapshotResponse) storageSnapshotResponse {
	busy, op, started := r.adminHeavyState()
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}

func (r *serverRuntime) attachAdminOpToDataSnapshot(resp storageDataSnapshotResponse) storageDataSnapshotResponse {
	busy, op, started := r.adminHeavyState()
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}

func (r *serverRuntime) attachAdminOpToRestoreDrill(resp storageRestoreDrillResponse) storageRestoreDrillResponse {
	busy, op, started := r.adminHeavyState()
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}

func (r *serverRuntime) attachAdminOpToDelete(resp deleteResponse) deleteResponse {
	busy, op, started := r.adminHeavyState()
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}

func (r *serverRuntime) streamEndRecord(format string, recordCount int, database, measurement string) streamRecord {
	busy, op, started := r.adminHeavyState()
	stats := r.queryStats()
	return streamRecord{
		Type:          streamTypeEnd,
		Stats:         &stats,
		Path:          routeDataQueryStream,
		Format:        format,
		RecordCount:   recordCount,
		Database:      database,
		Measurement:   measurement,
		AdminOpBusy:   busy,
		Op:            op,
		StartedAtUnix: started,
		Last:          r.lastAdminHeavySnapshot(),
	}
}

func (r *serverRuntime) attachAdminOpToOK(resp okResponse) okResponse {
	busy, op, started := r.adminHeavyState()
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}

func (r *serverRuntime) attachAdminOpToSetPassword(resp setPasswordResponse) setPasswordResponse {
	busy, op, started := r.adminHeavyState()
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}

func (r *serverRuntime) attachAdminOpToChangePassword(resp changePasswordResponse) changePasswordResponse {
	busy, op, started := r.adminHeavyState()
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}

func (r *serverRuntime) attachAdminOpToBatch(resp batchMutationResponse) batchMutationResponse {
	busy, op, started := r.adminHeavyState()
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}

func (r *serverRuntime) attachAdminOpToReload(resp reloadConfigResponse) reloadConfigResponse {
	busy, op, started := r.adminHeavyState()
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}

func (r *serverRuntime) attachAdminOpToDownsampleRun(resp downsampleRunResponse) downsampleRunResponse {
	busy, op, started := r.adminHeavyState()
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}

func (r *serverRuntime) attachAdminOpToDownsampleDryRun(resp downsampleDryRunResponse) downsampleDryRunResponse {
	busy, op, started := r.adminHeavyState()
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}

func (r *serverRuntime) attachAdminOpToWrite(resp writeResponse) writeResponse {
	busy, op, started := r.adminHeavyState()
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}

func (r *serverRuntime) attachAdminOpToQueryRows(resp queryRowsResponse) queryRowsResponse {
	busy, op, started := r.adminHeavyState()
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}

func (r *serverRuntime) attachAdminOpToQueryColumns(resp queryColumnsResponse) queryColumnsResponse {
	busy, op, started := r.adminHeavyState()
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}

func (r *serverRuntime) attachAdminOpToQueryExplain(resp queryExplainResponse) queryExplainResponse {
	busy, op, started := r.adminHeavyState()
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}

func (r *serverRuntime) dataContractPayload() dataContractResponse {
	cfg := r.currentConfig()
	resp := dataContractResponse{
		Version:           1,
		Path:              routeDataContract,
		MaxWritePoints:    cfg.Limits.MaxWritePoints,
		DefaultQueryLimit: cfg.Limits.DefaultQueryLimit,
		MaxQueryLimit:     cfg.Limits.MaxQueryLimit,
		Features: []dataContractFeature{
			{ID: "write_accepted_points", Path: routeDataWrite, Description: "write responses include points/path/mode/database/retention_policy", Enabled: true},
			{ID: "write_response_mode", Path: routeDataWrite, Description: "write responses include mode (points|typed|points_typed), database and retention_policy", Enabled: true},
			{ID: "write_response_retention", Path: routeDataWrite, Description: "write responses include retention_policy when single-policy batch", Enabled: true},
			{ID: "query_result_meta", Path: routeDataQueryRows, Description: "query rows/columns/explain include path/count/database/measurement/admin_op", Enabled: true},
			{ID: "query_stats_path", Path: routeDataQueryStats, Description: "GET query/stats includes path and admin_op", Enabled: true},
			{ID: "query_stream_end_meta", Path: routeDataQueryStream, Description: "stream end frame includes path/format/record_count/database/measurement/admin_op", Enabled: true},
			{ID: "delete_response_meta", Path: routeDataDelete, Description: "delete response includes path/database/measurement/admin_op", Enabled: true},
			{ID: "data_limits", Path: routeDataLimits, Description: "GET data/limits exposes write/query caps", Enabled: true},
			{ID: "meta_list_path", Path: routeDataDatabases, Description: "meta list responses include path/database/measurement scope", Enabled: true},
		},
	}
	return r.attachAdminOpToDataContract(resp)
}

func (r *serverRuntime) attachAdminOpToDataContract(resp dataContractResponse) dataContractResponse {
	busy, op, started := r.adminHeavyState()
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}

func (r *serverRuntime) dataLimitsPayload() dataLimitsResponse {
	cfg := r.currentConfig()
	resp := dataLimitsResponse{
		MaxWritePoints:    cfg.Limits.MaxWritePoints,
		DefaultQueryLimit: cfg.Limits.DefaultQueryLimit,
		MaxQueryLimit:     cfg.Limits.MaxQueryLimit,
		Path:              routeDataLimits,
	}
	return r.attachAdminOpToDataLimits(resp)
}

func (r *serverRuntime) attachAdminOpToDataLimits(resp dataLimitsResponse) dataLimitsResponse {
	busy, op, started := r.adminHeavyState()
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}

func (r *serverRuntime) attachAdminOpToConfigValidate(resp configValidateResponse) configValidateResponse {
	busy, op, started := r.adminHeavyState()
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	return resp
}
