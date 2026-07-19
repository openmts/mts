# Dashboard / 运维可商用 EARS 清单（2026-07-20 P44）

## 范围
- 部署材料包：证书/HSTS 验收命令、Nginx 样例、cron/systemd/env 样例
- 就绪中心一键复制与 Markdown 材料包下载
- 明确文案：复制/下载 ≠ 部署验收完成
- 文档同步

## 边界
- 不在 Dashboard 内自动安装证书、不启动 cron/systemd
- 不宣称边缘 HTTPS / 异地备份已完成

## EARS
- [x] EARS-FE-P44-01 WHEN 打开就绪中心 THE SYSTEM SHALL 展示部署材料包样例列表
- [x] EARS-FE-P44-02 WHEN 用户点击复制 THE SYSTEM SHALL 将对应样例写入剪贴板（失败时提示手动复制）
- [x] EARS-FE-P44-03 WHEN 用户下载材料包 THE SYSTEM SHALL 生成含全部样例的 Markdown 文件
- [x] EARS-FE-P44-04 WHEN 展示部署材料包 THE SYSTEM SHALL 明确标注部署侧人工签核仍为必做
- [x] EARS-FE-P44-05 WHEN 运行相关单元测试与商业冒烟 THE SYSTEM SHALL 覆盖材料包入口
- [x] EARS-DOC-P44-06 WHEN 更新基线 THE SYSTEM SHALL 记录 P44 与仍未完成项

## 验证
- `cd cmd/mts-dashboard && npm test && npm run build`
- `cd cmd/mts-dashboard && npm run test:e2e`
- 仓库根：`make test` / `make e2e` / `make lint`

## 可商用仍未完成（不宣称目标完成）
- 真实边缘证书/HSTS 人工验收执行
- 目标环境 cron/systemd 实装与演练归档
- 跨主机异地备份 + 告警通道真实跑通
