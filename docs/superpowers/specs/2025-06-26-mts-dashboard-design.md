# MTS Dashboard 前端管理页面设计方案

日期: 2025-06-26

## 目标

为 mts-server 构建一个 Vue 3 + TypeScript + shadcn-vue 前端管理页面（mts-dashboard），嵌入 Go 二进制，用户通过浏览器访问 mts-server 的 HTTP 地址即可管理整个 MTS 服务。

## 架构

```
┌─────────────────────────────────────────────────┐
│                mts-server (Go)                   │
│  ┌──────────┐  ┌──────────┐  ┌────────────────┐ │
│  │ /healthz  │  │ /api/v1/*│  │ /              │ │
│  │ /readyz   │  │ (40+ API)│  │ (Dashboard SPA)│ │
│  │ /metrics  │  │          │  │ embed.FS       │ │
│  └──────────┘  └──────────┘  └────────────────┘ │
└─────────────────────────────────────────────────┘
```

- 前端: Vue 3 + TypeScript + shadcn-vue + Vite，产出纯静态 SPA
- 嵌入: Go `//go:embed cmd/mts-dashboard/dist` 打包进二进制
- 路由: `/` 根路径挂载 Dashboard，现有 `/api/*` 不变
- SPA fallback: 非 API/非静态资源路径返回 `index.html`

## 前端路由与页面（9 个页面）

| 路由 | 页面 | 功能 |
|------|------|------|
| `/login` | 登录 | 账号密码表单（预填假值），点击进入（认证接口预留） |
| `/` | 仪表盘概览 | 服务健康、存储内存、压缩统计 |
| `/databases` | 数据库管理 | 创建/删除数据库、保留策略 CRUD |
| `/users` | 用户管理 | 用户 CRUD、数据库权限分配 |
| `/config` | 配置管理 | 查看/验证/重载配置 |
| `/operations` | 运维操作 | Flush、Compact、保留策略应用、维护错误 |
| `/downsample` | 降采样管理 | 策略 CRUD、启停、状态查询 |
| `/query` | 数据查询 | 行式/列式查询、流式查询、EXPLAIN |
| `/audit` | 审计日志 | 按用户查看审计事件 |
| `/storage` | 存储快照 | 快照创建、导出、验证 |

## 组件树

```
App.vue
├── LoginPage.vue              # /login
└── DashboardLayout.vue        # 认证后布局
    ├── SidebarNav.vue          # 左侧导航
    ├── TopBar.vue              # 页面标题
    └── <RouterView>
        ├── OverviewPage.vue
        ├── DatabasesPage.vue
        ├── UsersPage.vue
        ├── ConfigPage.vue
        ├── OperationsPage.vue
        ├── DownsamplePage.vue
        ├── QueryPage.vue
        ├── AuditPage.vue
        └── StoragePage.vue
```

## 构建流程

```
make build
  ├── cd cmd/mts-dashboard && npm ci && npm run build → dist/
  ├── cd cmd/mts-server && go build -o mts-server .
  └── 产物: mts-server (单二进制含前端)
```

## API 通信

- 前端通过 admin_token 调用后端 API
- 登录后 token 存储于内存（localStorage 预留，后续对接真实认证）
- 统一 HTTP client 封装，自动附加认证头

## 边界

- 前端不引入额外后端依赖（仅 Go 标准库 `embed`）
- 前端构建产物 < 2MB（gzip 后）
- 不修改现有 API 接口
- 认证模块预留接口位置，密码体系后续完善
