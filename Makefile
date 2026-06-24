SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c

PROJECT_NAME ?= github.com/openmts/mts
GOSUMDB := $(or $(MTS_GOSUMDB),sum.golang.org)
export GOSUMDB
GO ?= env GOSUMDB=$(GOSUMDB) go
COUNT ?= 1
TEST_TIMEOUT ?= 10m
TEST_WALL_TIMEOUT ?= 600s
SCENARIO_TIMEOUT ?= 15m
FMT_TIMEOUT ?= 300s
LINT_TIMEOUT ?= 720s
COVERAGE_MIN ?= 90.0
COVERAGE_PACKAGE_TIMEOUT ?= 300s
CI_TIMEOUT ?= 1800s

STORAGE_POINTS ?= 100000
STORAGE_BATCH_SIZE ?= 4096
STORAGE_COMPRESSION ?= snappy
STORAGE_DURABILITY ?= buffered
SOAK_DURATION ?= 30s
SOAK_SEED ?= 7
PPROF_POINTS ?= 10000
PPROF_SERIES ?= 100
PPROF_QUERY_REPEAT ?= 5
DOWNSAMPLE_POINTS ?= 100000
DOWNSAMPLE_SERIES ?= 100
DOWNSAMPLE_POLICIES ?= 1

CORE_PACKAGES = $(shell $(GO) list ./... | grep -v '/tests/' | grep -v '/internal/bench')
GO_TEST = timeout $(TEST_WALL_TIMEOUT) $(GO) test -count=$(COUNT) -timeout $(TEST_TIMEOUT)
SCENARIO_GO_TEST = timeout $(SCENARIO_TIMEOUT) $(GO) test -count=$(COUNT) -timeout $(TEST_TIMEOUT)

.DEFAULT_GOAL := help

.PHONY: help
help: ## 显示 Makefile 目标
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_.-]+:.*## / {printf "  %-26s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@printf "\n常用场景示例:\n"
	@printf "  make unit                  生产包单元测试\n"
	@printf "  make e2e-public-api        公开 API 端到端用例\n"
	@printf "  make fault-matrix          存储故障矩阵\n"
	@printf "  make storage-100k          100K 存储写查压缩场景\n"
	@printf "  make bench-query           查询迭代器性能基准\n"
	@printf "  make pprof-storage         存储 pprof smoke 场景\n"

.PHONY: fmt
fmt: ## 格式化 Go 代码
	timeout $(FMT_TIMEOUT) goimports-reviser -project-name $(PROJECT_NAME) -recursive -format -rm-unused .

.PHONY: lint
lint: ## 运行 golangci-lint
	timeout $(LINT_TIMEOUT) golangci-lint run ./...

.PHONY: unit
unit: ## 运行生产包单元测试
	$(GO_TEST) $(CORE_PACKAGES)

.PHONY: test
test: ## 运行全部 Go 测试
	$(GO_TEST) ./...

.PHONY: coverage
coverage: ## 检查生产包覆盖率不低于 COVERAGE_MIN
	@tmp_dir="$$(mktemp -d "$${TMPDIR:-/tmp}/mts-coverage.XXXXXX")"; \
	chmod 0700 "$$tmp_dir"; \
	trap 'rm -rf "$$tmp_dir"' EXIT; \
	failed=0; \
	for pkg in $(CORE_PACKAGES); do \
		profile="$$tmp_dir/$$(echo "$$pkg" | tr '/.' '__').cover"; \
		if ! output="$$(timeout $(COVERAGE_PACKAGE_TIMEOUT) $(GO) test "$$pkg" -coverprofile="$$profile" -count=$(COUNT) -timeout 5m 2>&1)"; then \
			printf '%s\n' "$$output"; \
			exit 1; \
		fi; \
		chmod 0600 "$$profile"; \
		printf '%s\n' "$$output"; \
		coverage="$$($(GO) tool cover -func="$$profile" | awk '/^total:/ {gsub("%", "", $$3); print $$3}')"; \
		if ! awk -v got="$$coverage" -v min="$(COVERAGE_MIN)" 'BEGIN { exit !(got + 0 >= min + 0) }'; then \
			printf 'coverage below threshold: package=%s got=%s%% min=%s%%\n' "$$pkg" "$$coverage" "$(COVERAGE_MIN)"; \
			failed=1; \
		fi; \
	done; \
	exit "$$failed"

.PHONY: e2e
e2e: ## 运行全部 e2e 用例
	$(SCENARIO_GO_TEST) ./tests/e2e/...

.PHONY: e2e-simple e2e-public-api e2e-wal e2e-flush e2e-no-json e2e-retention e2e-compaction e2e-query-pruning e2e-query-window e2e-streaming e2e-read-amplification e2e-service e2e-format e2e-downsample
e2e-simple: ## 运行 simple_integrity e2e
	$(SCENARIO_GO_TEST) ./tests/e2e/simple_integrity
e2e-public-api: ## 运行 public_api_workflow e2e
	$(SCENARIO_GO_TEST) ./tests/e2e/public_api_workflow
e2e-wal: ## 运行 wal_recovery e2e
	$(SCENARIO_GO_TEST) ./tests/e2e/wal_recovery
e2e-flush: ## 运行 flush_manifest_recovery e2e
	$(SCENARIO_GO_TEST) ./tests/e2e/flush_manifest_recovery
e2e-no-json: ## 运行 no_json_storage e2e
	$(SCENARIO_GO_TEST) ./tests/e2e/no_json_storage
e2e-retention: ## 运行 retention e2e
	$(SCENARIO_GO_TEST) ./tests/e2e/retention
e2e-compaction: ## 运行 compaction_integrity e2e
	$(SCENARIO_GO_TEST) ./tests/e2e/compaction_integrity
e2e-query-pruning: ## 运行 query_pruning e2e
	$(SCENARIO_GO_TEST) ./tests/e2e/query_pruning
e2e-query-window: ## 运行 query_aggregate_window e2e
	$(SCENARIO_GO_TEST) ./tests/e2e/query_aggregate_window
e2e-streaming: ## 运行 streaming_query e2e
	$(SCENARIO_GO_TEST) ./tests/e2e/streaming_query
e2e-read-amplification: ## 运行 read_amplification e2e
	$(SCENARIO_GO_TEST) ./tests/e2e/read_amplification
e2e-service: ## 运行 service_ops e2e
	$(SCENARIO_GO_TEST) ./tests/e2e/service_ops
e2e-format: ## 运行 format_governance e2e
	$(SCENARIO_GO_TEST) ./tests/e2e/format_governance
e2e-downsample: ## 运行 downsample_policy e2e
	$(SCENARIO_GO_TEST) ./tests/e2e/downsample_policy

.PHONY: simple-integrity public-api-workflow wal-recovery flush-manifest-recovery no-json-storage retention compaction-integrity query-pruning query-aggregate-window streaming-query read-amplification service-ops format-governance downsample-e2e
simple-integrity: e2e-simple ## e2e-simple 的短别名
public-api-workflow: e2e-public-api ## e2e-public-api 的短别名
wal-recovery: e2e-wal ## e2e-wal 的短别名
flush-manifest-recovery: e2e-flush ## e2e-flush 的短别名
no-json-storage: e2e-no-json ## e2e-no-json 的短别名
retention: e2e-retention ## e2e-retention 的短别名
compaction-integrity: e2e-compaction ## e2e-compaction 的短别名
query-pruning: e2e-query-pruning ## e2e-query-pruning 的短别名
query-aggregate-window: e2e-query-window ## e2e-query-window 的短别名
streaming-query: e2e-streaming ## e2e-streaming 的短别名
read-amplification: e2e-read-amplification ## e2e-read-amplification 的短别名
service-ops: e2e-service ## e2e-service 的短别名
format-governance: e2e-format ## e2e-format 的短别名
downsample-e2e: e2e-downsample ## e2e-downsample 的短别名

.PHONY: fault fault-matrix fault-downsample
fault: ## 运行全部 fault 用例
	$(SCENARIO_GO_TEST) ./tests/fault/...
fault-matrix: ## 运行存储故障矩阵
	$(SCENARIO_GO_TEST) ./tests/fault/storage_fault_matrix
fault-downsample: ## 运行降采样故障用例
	$(SCENARIO_GO_TEST) ./tests/fault/downsample_policy

.PHONY: scale scale-storage scale-downsample storage-100k storage-1m storage-10m storage-matrix storage-soak
scale: ## 运行全部 scale 测试包
	$(SCENARIO_GO_TEST) ./tests/scale/...
scale-storage: ## 运行可调规模存储场景，默认 100K
	@umask 077; timeout $(SCENARIO_TIMEOUT) $(GO) run ./tests/scale/storage_10m \
		-profile quick \
		-mode write-query-compact \
		-points $(STORAGE_POINTS) \
		-batch-size $(STORAGE_BATCH_SIZE) \
		-compression-algorithm $(STORAGE_COMPRESSION) \
		-durability $(STORAGE_DURABILITY)
storage-100k: scale-storage ## 运行 100K 存储写查压缩场景
storage-1m: ## 运行 1M 存储写查压缩场景
	@$(MAKE) scale-storage STORAGE_POINTS=1000000
storage-10m: ## 运行 10M 存储写查压缩场景
	@$(MAKE) scale-storage STORAGE_POINTS=10000000 SCENARIO_TIMEOUT=20m
storage-matrix: ## 运行小规模存储矩阵
	@umask 077; timeout $(SCENARIO_TIMEOUT) $(GO) run ./tests/scale/storage_matrix \
		-sizes 100k \
		-compressions off,snappy,zstd \
		-durabilities buffered,write-sync \
		-case-timeout 5m
storage-soak: ## 运行 30s 存储长稳 smoke
	@umask 077; timeout $(SCENARIO_TIMEOUT) $(GO) run ./tests/scale/storage_soak \
		-seed $(SOAK_SEED) \
		-duration $(SOAK_DURATION) \
		-report-interval 5s
scale-downsample: ## 运行降采样规模化场景
	@umask 077; timeout $(SCENARIO_TIMEOUT) $(GO) run ./tests/scale/downsample_policy \
		-points $(DOWNSAMPLE_POINTS) \
		-series $(DOWNSAMPLE_SERIES) \
		-policy-count $(DOWNSAMPLE_POLICIES)

.PHONY: pprof pprof-storage
pprof: ## 运行全部 pprof 测试包
	$(SCENARIO_GO_TEST) ./tests/pprof/...
pprof-storage: ## 运行存储 pprof smoke 场景，不落 profile 文件
	@umask 077; timeout $(SCENARIO_TIMEOUT) $(GO) run ./tests/pprof/storage_engine \
		-mode query \
		-points $(PPROF_POINTS) \
		-series $(PPROF_SERIES) \
		-query-repeat $(PPROF_QUERY_REPEAT)

.PHONY: bench bench-query
bench: ## 运行存储基准 gate
	timeout $(SCENARIO_TIMEOUT) bash scripts/storage_benchmark_gate.sh
bench-query: ## 运行查询迭代器基准 smoke
	timeout $(SCENARIO_TIMEOUT) $(GO) test ./internal/bench \
		-run '^$$' \
		-bench 'BenchmarkEngineQuery(Row|Column)Iterator/points=1000$$' \
		-benchmem \
		-count=$(COUNT) \
		-timeout $(TEST_TIMEOUT)

.PHONY: ci gate commercial
ci: ## 运行完整商用门禁脚本
	timeout $(CI_TIMEOUT) bash scripts/ci_gate.sh
gate: ci ## ci 的别名
commercial: ci ## ci 的别名

.PHONY: clean-artifacts
clean-artifacts: ## 清理测试和 profile 临时产物
	find . -type f \( \
		-name '*.test' -o \
		-name '*.prof' -o \
		-name '*.pprof' -o \
		-name 'coverage.out' -o \
		-name '*.coverprofile' \
	\) -not -path './.git/*' -print -delete
