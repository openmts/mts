package main

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	mts "github.com/openmts/mts"
)

func (r *serverRuntime) handleMetrics(writer http.ResponseWriter, request *http.Request) {
	if !requireHTTPMethod(writer, request, http.MethodGet) {
		return
	}
	writer.Header().Set("Content-Type", contentTypePrometheus)
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write([]byte(r.engine.PrometheusMetrics()))
	_, _ = writer.Write([]byte(r.metrics.prometheusText()))
	_, _ = writer.Write([]byte(r.healthMetrics()))
}

func (r *serverRuntime) handleAPISpec(writer http.ResponseWriter, request *http.Request) {
	if !r.requireHTTPAdminMethod(writer, request, http.MethodGet) {
		return
	}
	writeHTTPJSON(writer, http.StatusOK, r.apiSpecPayload())
}

func (r *serverRuntime) handleErrorCodes(writer http.ResponseWriter, request *http.Request) {
	if !r.requireHTTPAdminMethod(writer, request, http.MethodGet) {
		return
	}
	writeHTTPJSON(writer, http.StatusOK, r.errorCodesPayload())
}

func (r *serverRuntime) handleValidateConfig(writer http.ResponseWriter, request *http.Request) {
	if !r.requireHTTPAdminMethod(writer, request, http.MethodPost) {
		return
	}
	var req configValidateRequest
	if err := decodeHTTPJSON(request, &req); err != nil {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
		return
	}
	resp := r.validateConfigPayload(req.Config)
	if !resp.OK {
		writeHTTPJSON(writer, http.StatusBadRequest, resp)
		return
	}
	writeHTTPJSON(writer, http.StatusOK, resp)
}

func (r *serverRuntime) handleReloadConfig(writer http.ResponseWriter, request *http.Request) {
	if !r.requireHTTPAdminMethod(writer, request, http.MethodPost) {
		return
	}
	resp, err := r.reloadConfig()
	if err != nil {
		writeAPIError(writer, err)
		return
	}
	r.audit.record(auditEvent{UserName: r.auditUser(request), Action: "reload_config"})
	writeHTTPJSON(writer, http.StatusOK, resp)
}

func (r *serverRuntime) handleStorageValidate(writer http.ResponseWriter, request *http.Request) {
	if !r.requireHTTPAdminMethod(writer, request, http.MethodPost) {
		return
	}
	writeHTTPJSON(writer, http.StatusOK, r.storageValidatePayload())
}

func (r *serverRuntime) handleStorageDataSnapshot(writer http.ResponseWriter, request *http.Request) {
	if !r.requireHTTPAdminMethod(writer, request, http.MethodPost) {
		return
	}
	var req storageDataSnapshotRequest
	if err := decodeOptionalHTTPJSON(request, &req); err != nil {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
		return
	}
	flush := true
	if req.Flush != nil {
		flush = *req.Flush
	}
	resp, err := r.storageDataSnapshot(request.Context(), flush)
	if err != nil {
		writeAPIError(writer, err)
		return
	}
	r.audit.record(auditEvent{UserName: r.auditUser(request), Action: "storage_data_snapshot", Detail: resp.Path})
	writeHTTPJSON(writer, http.StatusOK, resp)
}

func (r *serverRuntime) handleStorageRestoreDrill(writer http.ResponseWriter, request *http.Request) {
	if !r.requireHTTPAdminMethod(writer, request, http.MethodPost) {
		return
	}
	var req storageRestoreDrillRequest
	if err := decodeOptionalHTTPJSON(request, &req); err != nil {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
		return
	}
	resp, err := r.storageRestoreDrill(request.Context(), req.SourcePath)
	if err != nil {
		// 互斥冲突保留 resource_exhausted；路径校验等仍按原始错误分类
		writeAPIError(writer, err)
		return
	}
	r.audit.record(auditEvent{
		UserName: r.auditUser(request),
		Action:   "storage_restore_drill",
		Detail:   resp.Source + " -> " + resp.Target,
	})
	writeHTTPJSON(writer, http.StatusOK, resp)
}

func (r *serverRuntime) handleListStorageDataSnapshots(writer http.ResponseWriter, request *http.Request) {
	if !r.requireHTTPAdminMethod(writer, request, http.MethodGet) {
		return
	}
	resp, err := r.listDataSnapshots()
	if err != nil {
		writeAPIError(writer, newAPIError(errorCodeInternal, err.Error(), err))
		return
	}
	writeHTTPJSON(writer, http.StatusOK, r.attachAdminOpToDataSnapshots(resp))
}

func (r *serverRuntime) handleStorageSnapshot(writer http.ResponseWriter, request *http.Request) {
	if !r.requireHTTPAdminMethod(writer, request, http.MethodPost) {
		return
	}
	resp, err := r.storageSnapshot(request.Context())
	if err != nil {
		writeAPIError(writer, err)
		return
	}
	r.audit.record(auditEvent{UserName: r.auditUser(request), Action: "storage_snapshot"})
	writeHTTPJSON(writer, http.StatusOK, resp)
}

func (r *serverRuntime) handleStorageExport(writer http.ResponseWriter, request *http.Request) {
	if !r.requireHTTPAdminMethod(writer, request, http.MethodGet) {
		return
	}
	writeHTTPJSON(writer, http.StatusOK, r.storageExportPayload(request.Context()))
}

func (r *serverRuntime) healthMetrics() string {
	healthy := 0
	ready := 0
	health := r.health()
	if health.Healthy {
		healthy = 1
	}
	if health.Ready {
		ready = 1
	}
	return "# HELP mts_health_healthy Engine healthy state.\n" +
		"# TYPE mts_health_healthy gauge\n" +
		"mts_health_healthy " + intMetric(healthy) + "\n" +
		"# HELP mts_health_ready Engine ready state.\n" +
		"# TYPE mts_health_ready gauge\n" +
		"mts_health_ready " + intMetric(ready) + "\n"
}

func intMetric(value int) string {
	if value == 0 {
		return "0"
	}
	return "1"
}

func (r *serverRuntime) handleConfig(writer http.ResponseWriter, request *http.Request) {
	if !r.requireHTTPAdminMethod(writer, request, http.MethodGet) {
		return
	}
	writeHTTPJSON(writer, http.StatusOK, r.configPayload())
}

func (r *serverRuntime) handleConfigSchema(writer http.ResponseWriter, request *http.Request) {
	if !r.requireHTTPAdminMethod(writer, request, http.MethodGet) {
		return
	}
	writeHTTPJSON(writer, http.StatusOK, r.configSchemaPayload())
}

func (r *serverRuntime) handleFlush(writer http.ResponseWriter, request *http.Request) {
	if !r.requireHTTPAdminMethod(writer, request, http.MethodPost) {
		return
	}
	if err := r.flush(request.Context()); err != nil {
		writeAPIError(writer, err)
		return
	}
	r.audit.record(auditEvent{UserName: r.auditUser(request), Action: "flush"})
	writeHTTPJSON(writer, http.StatusOK, maintenanceResponse{OK: true})
}

func (r *serverRuntime) handleCompact(writer http.ResponseWriter, request *http.Request) {
	if !r.requireHTTPAdminMethod(writer, request, http.MethodPost) {
		return
	}
	result, err := r.compact(request.Context())
	if err != nil {
		writeAPIError(writer, err)
		return
	}
	r.audit.record(auditEvent{UserName: r.auditUser(request), Action: "compact"})
	writeHTTPJSON(writer, http.StatusOK, maintenanceResponse{OK: true, Result: result})
}

func (r *serverRuntime) handleApplyRetention(writer http.ResponseWriter, request *http.Request) {
	if !r.requireHTTPAdminMethod(writer, request, http.MethodPost) {
		return
	}
	var req retentionApplyRequest
	if err := decodeHTTPJSON(request, &req); err != nil {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
		return
	}
	if err := r.applyRetention(request.Context(), req); err != nil {
		writeAPIError(writer, err)
		return
	}
	r.audit.record(auditEvent{UserName: r.auditUser(request), Action: "apply_retention"})
	writeHTTPJSON(writer, http.StatusOK, okResponse{OK: true})
}

func (r *serverRuntime) handleMaintenanceErrors(writer http.ResponseWriter, request *http.Request) {
	if !r.requireHTTPAdminMethod(writer, request, http.MethodGet) {
		return
	}
	busy, op, started := r.adminHeavyState()
	writeHTTPJSON(writer, http.StatusOK, maintenanceErrorsResponse{
		Errors:        r.maintenanceErrors(request.Context()),
		AdminOpBusy:   busy,
		Op:            op,
		StartedAtUnix: started,
		Last:          r.lastAdminHeavySnapshot(),
	})
}

func (r *serverRuntime) handleStorageMemory(writer http.ResponseWriter, request *http.Request) {
	if !r.requireHTTPAdminMethod(writer, request, http.MethodGet) {
		return
	}
	writeHTTPJSON(writer, http.StatusOK, r.storageMemoryPayload())
}

func (r *serverRuntime) handleCompactionStats(writer http.ResponseWriter, request *http.Request) {
	if !r.requireHTTPAdminMethod(writer, request, http.MethodGet) {
		return
	}
	writeHTTPJSON(writer, http.StatusOK, r.compactionStatsPayload())
}

func (r *serverRuntime) handleMaintenanceStats(writer http.ResponseWriter, request *http.Request) {
	if !r.requireHTTPAdminMethod(writer, request, http.MethodGet) {
		return
	}
	writeHTTPJSON(writer, http.StatusOK, r.maintenanceStatsPayload())
}

func (r *serverRuntime) handleOpsStatus(writer http.ResponseWriter, request *http.Request) {
	if !r.requireHTTPAdminMethod(writer, request, http.MethodGet) {
		return
	}
	writeHTTPJSON(writer, http.StatusOK, r.opsStatusPayload())
}

func (r *serverRuntime) handleAdminDoctor(writer http.ResponseWriter, request *http.Request) {
	if !r.requireHTTPAdminMethod(writer, request, http.MethodGet) {
		return
	}
	resp, err := evaluateDoctor(r.currentConfig())
	if err != nil {
		writeAPIError(writer, newAPIError(errorCodeInternal, err.Error(), err))
		return
	}
	busy, op, started := r.adminHeavyState()
	resp.AdminOpBusy = busy
	resp.Op = op
	resp.StartedAtUnix = started
	resp.Last = r.lastAdminHeavySnapshot()
	writeHTTPJSON(writer, http.StatusOK, resp)
}

func (r *serverRuntime) handleAdminVersion(writer http.ResponseWriter, request *http.Request) {
	if !r.requireHTTPAdminMethod(writer, request, http.MethodGet) {
		return
	}
	writeHTTPJSON(writer, http.StatusOK, r.adminVersionPayload())
}

func (r *serverRuntime) handleAdminHealth(writer http.ResponseWriter, request *http.Request) {
	if !r.requireHTTPAdminMethod(writer, request, http.MethodGet) {
		return
	}
	writeHTTPJSON(writer, http.StatusOK, r.adminHealthPayload())
}

func (r *serverRuntime) handleDownsamplePolicies(writer http.ResponseWriter, request *http.Request) {
	if err := r.requireHTTPAdmin(request); err != nil {
		writeAPIError(writer, err)
		return
	}
	switch request.Method {
	case http.MethodPost, http.MethodPut:
		var policy mts.DownsamplePolicy
		if err := decodeHTTPJSON(request, &policy); err != nil {
			writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
			return
		}
		if err := r.engine.CreateDownsamplePolicy(request.Context(), policy); err != nil {
			writeAPIError(writer, err)
			return
		}
		r.audit.record(auditEvent{UserName: r.auditUser(request), Action: "upsert_downsample_policy", Detail: policy.Name})
		writeHTTPJSON(writer, http.StatusOK, okResponse{OK: true})
	case http.MethodGet:
		policies, err := r.engine.ListDownsamplePolicies(request.Context())
		if err != nil {
			writeAPIError(writer, err)
			return
		}
		writeHTTPJSON(writer, http.StatusOK, r.attachAdminOpToDownsamplePolicies(downsamplePoliciesResponse{Policies: policies}))
	default:
		writeAPIError(writer, newAPIError(errorCodeBadRequest, messageMethodNotAllowed, nil))
	}
}

func (r *serverRuntime) handleDownsampleStatuses(writer http.ResponseWriter, request *http.Request) {
	if !r.requireHTTPAdminMethod(writer, request, http.MethodGet) {
		return
	}
	statuses, err := r.engine.DownsamplePolicyStatuses(request.Context(), time.Now())
	if err != nil {
		writeAPIError(writer, err)
		return
	}
	writeHTTPJSON(writer, http.StatusOK, r.attachAdminOpToDownsampleStatuses(downsampleStatusesResponse{Statuses: statuses}))
}

func (r *serverRuntime) handleDownsamplePolicyResource(writer http.ResponseWriter, request *http.Request) {
	if err := r.requireHTTPAdmin(request); err != nil {
		writeAPIError(writer, err)
		return
	}
	parts := splitPath(request.URL.Path, routeAdminDownsamplePrefix)
	if len(parts) == 0 {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, "policy name is required", nil))
		return
	}
	name := parts[0]
	if len(parts) == 1 {
		if request.Method != http.MethodDelete {
			writeAPIError(writer, newAPIError(errorCodeBadRequest, messageMethodNotAllowed, nil))
			return
		}
		var req downsampleDropRequest
		_ = decodeHTTPJSON(request, &req)
		if err := r.engine.DropDownsamplePolicyWithOptions(request.Context(), name, req.Options); err != nil {
			writeAPIError(writer, err)
			return
		}
		r.audit.record(auditEvent{UserName: r.auditUser(request), Action: "drop_downsample_policy", Detail: name})
		writeHTTPJSON(writer, http.StatusOK, okResponse{OK: true})
		return
	}
	if request.Method != http.MethodPost {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, messageMethodNotAllowed, nil))
		return
	}
	r.handleDownsampleAction(writer, request, name, parts[1])
}

func (r *serverRuntime) handleDownsampleAction(
	writer http.ResponseWriter,
	request *http.Request,
	name string,
	action string,
) {
	switch action {
	case "enable":
		err := r.engine.EnableDownsamplePolicy(request.Context(), name)
		writeActionOK(writer, err)
		if err == nil {
			r.audit.record(auditEvent{UserName: r.auditUser(request), Action: "enable_downsample_policy", Detail: name})
		}
	case "disable":
		err := r.engine.DisableDownsamplePolicy(request.Context(), name)
		writeActionOK(writer, err)
		if err == nil {
			r.audit.record(auditEvent{UserName: r.auditUser(request), Action: "disable_downsample_policy", Detail: name})
		}
	case "reset":
		var req downsampleResetRequest
		if decodeActionRequest(writer, request, &req) {
			err := r.engine.ResetDownsamplePolicy(request.Context(), name, req.Reset)
			writeActionOK(writer, err)
			if err == nil {
				r.audit.record(auditEvent{UserName: r.auditUser(request), Action: "reset_downsample_policy", Detail: name})
			}
		}
	case "run":
		var req downsampleRunRequest
		if decodeActionRequest(writer, request, &req) {
			result, err := r.engine.RunDownsamplePolicy(request.Context(), name, unixSecondsOrNow(req.NowUnix))
			writeDownsampleRun(writer, result, err)
			if err == nil {
				r.audit.record(auditEvent{UserName: r.auditUser(request), Action: "run_downsample_policy", Detail: name})
			}
		}
	case "run-range":
		var req downsampleRangeRequest
		if decodeActionRequest(writer, request, &req) {
			result, err := r.engine.RunDownsamplePolicyRange(
				request.Context(),
				name,
				unixFlexible(req.StartUnix),
				unixFlexible(req.EndUnix),
				req.Options,
			)
			writeDownsampleRun(writer, result, err)
		}
	case "repair":
		var req downsampleRangeRequest
		if decodeActionRequest(writer, request, &req) {
			result, err := r.engine.RepairDownsamplePolicy(
				request.Context(),
				name,
				unixFlexible(req.StartUnix),
				unixFlexible(req.EndUnix),
			)
			writeDownsampleRun(writer, result, err)
		}
	case "dry-run":
		var req downsampleRangeRequest
		if decodeActionRequest(writer, request, &req) {
			result, err := r.engine.DryRunDownsamplePolicy(
				request.Context(),
				name,
				unixFlexible(req.StartUnix),
				unixFlexible(req.EndUnix),
			)
			if err != nil {
				writeAPIError(writer, err)
				return
			}
			writeHTTPJSON(writer, http.StatusOK, downsampleDryRunResponse{Result: result})
		}
	default:
		writeAPIError(writer, newAPIError(errorCodeNotFound, "downsample action not found", nil))
	}
}

func decodeActionRequest(writer http.ResponseWriter, request *http.Request, value any) bool {
	if err := decodeHTTPJSON(request, value); err != nil {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
		return false
	}
	return true
}

func writeActionOK(writer http.ResponseWriter, err error) {
	if err != nil {
		writeAPIError(writer, err)
		return
	}
	writeHTTPJSON(writer, http.StatusOK, okResponse{OK: true})
}

func writeDownsampleRun(writer http.ResponseWriter, result mts.DownsampleRunResult, err error) {
	if err != nil {
		writeAPIError(writer, err)
		return
	}
	writeHTTPJSON(writer, http.StatusOK, downsampleRunResponse{Result: result})
}

func unixSeconds(value int64) time.Time {
	return time.Unix(value, 0)
}

func unixFlexible(value int64) time.Time {
	if value > int64(100*365*24*time.Hour/time.Second) {
		return time.Unix(0, value)
	}
	return unixSeconds(value)
}

func unixSecondsOrNow(value int64) time.Time {
	if value == 0 {
		return time.Now()
	}
	return unixFlexible(value)
}

func timeNow() time.Time { return time.Now() }

func configSchema() []configFieldSchema {
	return []configFieldSchema{
		{Name: "data_dir", Description: "MTS 本地数据目录"},
		{Name: "http.enabled", Description: "是否启用 HTTP API"},
		{Name: "http.addr", Description: "HTTP 监听地址"},
		{Name: "http.dashboard_base", Description: "Dashboard 静态资源与 SPA 子路径，默认 /；例如 /mts/"},
		{Name: "grpc.enabled", Description: "是否启用 gRPC API"},
		{Name: "grpc.addr", Description: "gRPC 监听地址"},
		{Name: "auth.admin_token", Description: "管理面和用户面 admin token，空值表示开发兼容模式"},
		{Name: "auth.require_user", Description: "数据面是否强制用户 Bearer token 和 DB 权限校验"},
		{Name: "user.endpoint", Description: "用户模块接入地址，默认 local"},
		{Name: "user.password_auth_disabled", Description: "是否关闭密码登录和 token 校验能力"},
		{Name: "engine.default_database", Description: "默认 database"},
		{Name: "engine.default_retention_policy", Description: "默认 retention policy"},
		{Name: "engine.shard_duration", Description: "shard 时间窗口"},
		{Name: "engine.retention", Description: "默认保留时间"},
		{Name: "engine.memtable_max_samples", Description: "MemTable 样本阈值"},
		{Name: "engine.max_concurrent_compaction", Description: "跨 shard 并行 compact 上限"},
		{Name: "engine.max_concurrent_downsample", Description: "后台降采样并发上限"},
		{Name: "engine.compression.value_page_samples", Description: "values 页样本数"},
		{Name: "engine.compression.omit_write_seq", Description: "省略 per-sample writeSeq"},
		{Name: "engine.compression.zstd_level", Description: "zstd 强度 fastest|default|better|best"},
		{Name: "engine.query_page_cache.limit", Description: "查询 page 解码缓存条目上限，-1 关闭"},
		{Name: "engine.query_block_cache.limit", Description: "查询 block payload 缓存条目上限，-1 关闭"},
		{Name: "engine.storage_memory.hard_bytes_limit", Description: "存储硬内存字节上限"},
		{Name: "engine.cardinality.max_series", Description: "series 高基数硬限制"},
		{Name: "shutdown_timeout", Description: "优雅关闭超时"},
	}
}

func (r *serverRuntime) handleListAudit(writer http.ResponseWriter, request *http.Request) {
	if !r.requireHTTPAdminMethod(writer, request, http.MethodGet) {
		return
	}
	q := request.URL.Query()
	req := auditListRequest{
		UserName: q.Get("user_name"),
		Action:   q.Get("action"),
	}
	if v := q.Get("since_unix"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			req.SinceUnix = n
		}
	}
	if v := q.Get("until_unix"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			req.UntilUnix = n
		}
	}
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			req.Limit = n
		}
	}
	events := r.audit.listFiltered(req)
	writeHTTPJSON(writer, http.StatusOK, r.attachAdminOpToAudit(auditListResponse{Events: events, Total: len(events)}))
}

func (r *serverRuntime) handleListStorageSnapshots(writer http.ResponseWriter, request *http.Request) {
	if !r.requireHTTPAdminMethod(writer, request, http.MethodGet) {
		return
	}
	resp, err := r.listStorageSnapshots()
	if err != nil {
		writeAPIError(writer, newAPIError(errorCodeInternal, err.Error(), err))
		return
	}
	writeHTTPJSON(writer, http.StatusOK, r.attachAdminOpToSnapshots(resp))
}

func (r *serverRuntime) handleDeleteStorageSnapshot(writer http.ResponseWriter, request *http.Request) {
	if !r.requireHTTPAdminMethod(writer, request, http.MethodDelete) {
		return
	}
	name := strings.TrimSpace(request.URL.Query().Get("name"))
	if name == "" {
		// allow path suffix style via X or body? keep query for simplicity
		writeAPIError(writer, newAPIError(errorCodeBadRequest, "name is required", nil))
		return
	}
	if err := r.deleteStorageSnapshot(name); err != nil {
		if os.IsNotExist(err) {
			writeAPIError(writer, newAPIError(errorCodeNotFound, err.Error(), err))
			return
		}
		writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
		return
	}
	r.audit.record(auditEvent{UserName: r.auditUser(request), Action: "delete_storage_snapshot", Detail: name})
	writeHTTPJSON(writer, http.StatusOK, okResponse{OK: true})
}

func (r *serverRuntime) handleStorageSnapshots(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		r.handleListStorageSnapshots(writer, request)
	case http.MethodDelete:
		r.handleDeleteStorageSnapshot(writer, request)
	default:
		writer.Header().Set("Allow", "GET, DELETE")
		writeAPIError(writer, newAPIError(errorCodeBadRequest, "method not allowed", nil))
	}
}
