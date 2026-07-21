# Dashboard UX P282（2026-07-21）

## 目标
- 抽取 `PasswordInputWithToggle` 统一密码可见性交互
- Account 续期/改密、Login、强制改密复用同一组件

## 验收
- [x] Account / Force / Login 密码框具备 toggle testid
- [x] commercial-smoke 覆盖 account/force/login toggle
- [x] npm test / build / e2e 通过后合入
