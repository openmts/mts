# Code Review — Dashboard P222–P223（2026-07-21）

## 范围
- mutation 文案 helper + 8 页面接入
- requestTimeout + client 默认 30s / probe 5s
- apiError timeout 映射

## 结论
- 写门禁文案一致、可维护
- 普通 API 挂死风险下降；流式查询保持用户 abort 语义

## 残余
- NDJSON 仍无服务端 idle timeout（依赖用户取消/网络）
- 宣称可商用完成：否
