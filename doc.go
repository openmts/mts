// Package mts 提供单机嵌入式时序存储引擎。
//
// MTS 面向 Go 应用内的本地时序数据存储，公开点写入、typed batch 写入、
// Builder 查询、本地元数据管理、compaction、retention 和本地降采样策略 API。
// 内部时间统一按纳秒存储，public API 可通过 TimePrecision 声明秒、毫秒、
// 微秒或纳秒输入和查询返回时间戳。
// 用户管理通过 UserManager 接口暴露，默认 LocalUserManager 提供本地用户
// CRUD 和 database 级 read/write/admin 授权。
//
// 当前公开 API 明确限定在单进程和本地数据目录内，不提供分布式查询、
// 分布式存储、外部元数据系统、SQL、InfluxQL、PromQL 或 MetricsQL parser。
package mts
