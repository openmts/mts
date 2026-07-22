# Dashboard UX P492 — 有效配置结构化摘要

## 目标
Config 有效配置从整屏 raw JSON 升级为可扫视摘要 + 折叠原始 JSON，便于运维快速核对分区与敏感键。

## 范围
- `summarizeEffectiveConfig` 纯函数 + 单测
- ConfigPage 摘要卡 testid
- 清单 / 命令面板 / commercial-smoke
- 不改服务端 config 载荷（已含 path）

## 验收
- [x] summarizeEffectiveConfig 单测
- [x] config-effective-summary / sections / leaves testid
- [x] productionChecklist `config-effective-summary`
- [x] npm test / build / commercial-smoke
- [x] 不宣称 dashboard 可商用 goal complete
