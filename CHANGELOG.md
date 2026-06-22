# Changelog

## Unreleased

### Added

- 增加根包 README、package godoc 和可执行示例。
- 增加 `DefaultOptions(path)`、`Options.Validate()`、`ErrInvalidOptions`。
- 增加公共错误类别 `ErrNotFound`、`ErrUnsupported`。
- 增加 QueryBuilder `TimeRangeTime(start, end)` 和聚合常量。
- 增加 MIT License、贡献说明和 AI 友好项目摘要。

### Changed

- 拆分根包公开 DTO 和 internal 转换文件，降低 API 维护成本。
- 明确 HTTP 查询服务仍属于 internal 实现，不作为当前外部稳定 API。
