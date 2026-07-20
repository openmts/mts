# Dashboard / Readiness 签核引导 EARS（2026-07-20 P136）

## 范围
- 三项签核字段增加可展开填写引导（步骤 + 示例）
- 空白字段一键填入示例；已有内容不覆盖
- 顶部引导 banner 强调不计入评分

## 边界
- 不改 readiness 评分公式
- 示例文本不得包含真实密钥/Token

## EARS
- [x] EARS-FE-P136-01 WHEN 打开签核区 THE SYSTEM SHALL 展示引导 banner
- [x] EARS-FE-P136-02 WHEN 展开字段引导 THE SYSTEM SHALL 显示步骤与示例按钮
- [x] EARS-FE-P136-03 WHEN 字段为空且用户点击示例 THE SYSTEM SHALL 填入示例文案
- [x] EARS-FE-P136-04 WHEN 字段非空 THE SYSTEM SHALL 拒绝覆盖并提示
- [x] EARS-FE-P136-05 WHEN 商业冒烟运行 THE SYSTEM SHALL 覆盖 guide testid 与示例填充
- [x] EARS-DOC-P136-06 WHEN 更新基线 THE SYSTEM SHALL 记录 P136

## 实现备注
- utils/signoffGuide.ts
- testid：readiness-signoff-guide-banner / signoff-guide-{field} / signoff-guide-fill-{field}

## 验证
- npm test && npm run build && npm run test:e2e ✅
- make e2e + go test ./... ✅
