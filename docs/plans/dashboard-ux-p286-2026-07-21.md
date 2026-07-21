# Dashboard UX P286（2026-07-21）

## 目标
- 会话 warn 阶段展示非阻断横幅与剩余时间，引导续期
- critical 仍保持写禁用横幅

## 验收
- [x] `session-warn-banner` / remaining / renew
- [x] warn 不禁用写；critical 仍禁用
- [x] commercial-smoke 覆盖 warn→续期
- [x] npm test / build / e2e 通过后合入
