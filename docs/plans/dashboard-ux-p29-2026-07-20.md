# Dashboard / 运维可商用 EARS 清单（2026-07-20 P29）

## 范围
- 无障碍：跳过链接到主内容、main landmark id、侧栏/命令按钮 aria 补强
- 运维页：操作结果本地历史（session）+ JSON 导出
- 通用 download 工具纯函数可单测
- 文档与基线同步

## 边界
- 不改服务端运维 API
- 操作历史仅浏览器 sessionStorage，不跨机器
- 仍不宣称可商用完成

## EARS
- [x] EARS-FE-P29-01 WHEN 页面加载 THE SYSTEM SHALL 提供“跳到主内容”链接并锚定 main#main-content
- [x] EARS-FE-P29-02 WHEN 运维操作成功或失败 THE SYSTEM SHALL 追加本地操作历史（含 kind/message/at）
- [x] EARS-FE-P29-03 WHEN 用户导出运维结果 THE SYSTEM SHALL 下载 versioned JSON；空历史时提示
- [x] EARS-FE-P29-04 WHEN 刷新运维页 THE SYSTEM SHALL 从 sessionStorage 恢复操作历史
- [x] EARS-DOC-P29-05 WHEN 更新基线 THE SYSTEM SHALL 记录 P29 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm run test && npm run build`
- `make test` / `make e2e` / `make lint`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收
- 目标环境 cron/systemd 与跨主机备份实装
