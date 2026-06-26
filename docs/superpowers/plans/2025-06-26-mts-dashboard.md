# MTS Dashboard 前端管理页面实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 mts-server 构建 Vue 3 + TypeScript + shadcn-vue 前端管理页面，通过 Go embed 嵌入二进制，用户通过浏览器访问 HTTP 地址即可管理 MTS 服务。

**Architecture:** 前端 SPA（Vite 构建）产物置于 `cmd/mts-server/dashboard-dist/`，Go 编译时 `//go:embed` 打包进二进制；`http.ServeMux` 的 `/` 路径注册 SPA handler，非 API 路径 fallback 到 `index.html`。

**Tech Stack:** Vue 3.5, TypeScript, Vite 7, shadcn-vue, Tailwind CSS 4, Vue Router 4, lucide-vue-next, Go 1.26 embed

---

## 文件结构规划

```
cmd/mts-dashboard/
├── package.json                     # 前端依赖声明
├── tsconfig.json                    # TS 配置总入口
├── tsconfig.app.json                # 应用 TS 配置
├── vite.config.ts                   # Vite 配置（输出到 ../mts-server/dashboard-dist）
├── index.html                       # SPA 入口 HTML
├── src/
│   ├── main.ts                      # Vue 应用入口
│   ├── App.vue                      # 根组件（仅 RouterView）
│   ├── index.css                    # Tailwind 基础样式
│   ├── router/index.ts              # 路由定义
│   ├── api/client.ts                # HTTP API 客户端封装
│   ├── composables/useAuth.ts       # 认证状态管理（stub）
│   ├── layouts/DashboardLayout.vue  # 主布局（Sidebar + TopBar + RouterView）
│   ├── pages/
│   │   ├── LoginPage.vue            # 登录页
│   │   ├── OverviewPage.vue         # 仪表盘概览
│   │   ├── DatabasesPage.vue        # 数据库管理
│   │   ├── UsersPage.vue            # 用户管理
│   │   ├── ConfigPage.vue           # 配置管理
│   │   ├── OperationsPage.vue       # 运维操作
│   │   ├── DownsamplePage.vue       # 降采样管理
│   │   ├── QueryPage.vue            # 数据查询
│   │   ├── AuditPage.vue            # 审计日志
│   │   └── StoragePage.vue          # 存储快照
│   ├── components/
│   │   ├── SidebarNav.vue           # 侧边导航
│   │   └── TopBar.vue               # 顶栏
│   └── lib/utils.ts                 # shadcn-vue 工具函数

cmd/mts-server/
├── dashboard.go                     # [NEW] embed + SPA file server
├── http.go                          # [MODIFY] 注册 SPA handler

Makefile                              # [MODIFY] 添加 frontend 构建目标
```

---

### Task 1: 前端工程脚手架

**Files:**
- Create: `cmd/mts-dashboard/package.json`
- Create: `cmd/mts-dashboard/tsconfig.json`
- Create: `cmd/mts-dashboard/tsconfig.app.json`
- Create: `cmd/mts-dashboard/vite.config.ts`
- Create: `cmd/mts-dashboard/index.html`
- Create: `cmd/mts-dashboard/src/index.css`
- Create: `cmd/mts-dashboard/src/lib/utils.ts`
- Create: `cmd/mts-dashboard/src/main.ts`
- Create: `cmd/mts-dashboard/src/App.vue`

- [ ] **Step 1: 创建 package.json**

```json
{
  "name": "mts-dashboard",
  "version": "0.0.0",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vue-tsc -b && vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "vue": "^3.5.21",
    "vue-router": "^4.6.4",
    "lucide-vue-next": "^0.561.0",
    "class-variance-authority": "^0.7.1",
    "clsx": "^2.1.1",
    "tailwind-merge": "^3.4.0",
    "radix-vue": "^1.9.17"
  },
  "devDependencies": {
    "@vitejs/plugin-vue": "^5.2.4",
    "typescript": "~5.9.3",
    "vite": "^7.2.7",
    "vue-tsc": "^3.1.8",
    "tailwindcss": "^4.1.18",
    "@tailwindcss/vite": "^4.1.18"
  }
}
```

- [ ] **Step 2: 创建 tsconfig.json**

```json
{
  "files": [],
  "references": [
    { "path": "./tsconfig.app.json" }
  ]
}
```

- [ ] **Step 3: 创建 tsconfig.app.json**

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "moduleResolution": "bundler",
    "jsx": "preserve",
    "jsxImportSource": "vue",
    "strict": true,
    "noEmit": true,
    "skipLibCheck": true,
    "isolatedModules": true,
    "baseUrl": ".",
    "paths": {
      "@/*": ["./src/*"]
    }
  },
  "include": ["src/**/*.ts", "src/**/*.vue"]
}
```

- [ ] **Step 4: 创建 vite.config.ts**

```typescript
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { resolve } from 'path'

export default defineConfig({
  plugins: [vue(), tailwindcss()],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
    },
  },
  build: {
    outDir: resolve(__dirname, '../mts-server/dashboard-dist'),
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/api': 'http://127.0.0.1:8086',
      '/healthz': 'http://127.0.0.1:8086',
      '/readyz': 'http://127.0.0.1:8086',
      '/metrics': 'http://127.0.0.1:8086',
    },
  },
})
```

- [ ] **Step 5: 创建 index.html**

```html
<!DOCTYPE html>
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>MTS Dashboard</title>
  </head>
  <body>
    <div id="app"></div>
    <script type="module" src="/src/main.ts"></script>
  </body>
</html>
```

- [ ] **Step 6: 创建 src/index.css**

```css
@import 'tailwindcss';
```

- [ ] **Step 7: 创建 src/lib/utils.ts**

```typescript
import { type ClassValue, clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}
```

- [ ] **Step 8: 创建 src/main.ts**

```typescript
import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import './index.css'

const app = createApp(App)
app.use(router)
app.mount('#app')
```

- [ ] **Step 9: 创建 src/App.vue**

```vue
<script setup lang="ts">
</script>

<template>
  <RouterView />
</template>
```

- [ ] **Step 10: 安装依赖**

Run: `cd cmd/mts-dashboard && npm install`

Expected: node_modules 目录创建成功，无报错。

- [ ] **Step 11: 验证构建**

Run: `cd cmd/mts-dashboard && npm run build`

Expected: `cmd/mts-server/dashboard-dist/` 目录生成，包含 `index.html` 和 `assets/`。

---

### Task 2: Vue Router 与核心组件

**Files:**
- Create: `cmd/mts-dashboard/src/router/index.ts`
- Create: `cmd/mts-dashboard/src/api/client.ts`
- Create: `cmd/mts-dashboard/src/composables/useAuth.ts`
- Create: `cmd/mts-dashboard/src/layouts/DashboardLayout.vue`
- Create: `cmd/mts-dashboard/src/components/SidebarNav.vue`
- Create: `cmd/mts-dashboard/src/components/TopBar.vue`

- [ ] **Step 1: 创建 Vue Router**

File: `cmd/mts-dashboard/src/router/index.ts`

```typescript
import { createRouter, createWebHistory } from 'vue-router'
import { useAuth } from '@/composables/useAuth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'Login',
      component: () => import('@/pages/LoginPage.vue'),
    },
    {
      path: '/',
      component: () => import('@/layouts/DashboardLayout.vue'),
      children: [
        {
          path: '',
          name: 'Overview',
          component: () => import('@/pages/OverviewPage.vue'),
        },
        {
          path: 'databases',
          name: 'Databases',
          component: () => import('@/pages/DatabasesPage.vue'),
        },
        {
          path: 'users',
          name: 'Users',
          component: () => import('@/pages/UsersPage.vue'),
        },
        {
          path: 'config',
          name: 'Config',
          component: () => import('@/pages/ConfigPage.vue'),
        },
        {
          path: 'operations',
          name: 'Operations',
          component: () => import('@/pages/OperationsPage.vue'),
        },
        {
          path: 'downsample',
          name: 'Downsample',
          component: () => import('@/pages/DownsamplePage.vue'),
        },
        {
          path: 'query',
          name: 'Query',
          component: () => import('@/pages/QueryPage.vue'),
        },
        {
          path: 'audit',
          name: 'Audit',
          component: () => import('@/pages/AuditPage.vue'),
        },
        {
          path: 'storage',
          name: 'Storage',
          component: () => import('@/pages/StoragePage.vue'),
        },
      ],
    },
  ],
})

router.beforeEach((to) => {
  const { isAuthenticated } = useAuth()
  if (to.name !== 'Login' && !isAuthenticated.value) {
    return { name: 'Login' }
  }
})

export default router
```

- [ ] **Step 2: 创建 API Client**

File: `cmd/mts-dashboard/src/api/client.ts`

```typescript
const API_BASE = ''

interface APIError {
  ok: boolean
  code: string
  message: string
  error?: string
}

class APIClientError extends Error {
  code: string
  status: number

  constructor(status: number, code: string, message: string) {
    super(message)
    this.name = 'APIClientError'
    this.code = code
    this.status = status
  }
}

let adminToken = ''

export function setAdminToken(token: string) {
  adminToken = token
}

export function getAdminToken(): string {
  return adminToken
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string>),
  }
  if (adminToken) {
    headers['X-MTS-Admin-Token'] = adminToken
  }
  const response = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
  })
  if (!response.ok) {
    let err: APIError = { ok: false, code: 'internal', message: response.statusText }
    try {
      err = await response.json()
    } catch (_) {
      // 响应体非 JSON，使用默认错误
    }
    throw new APIClientError(response.status, err.code, err.message)
  }
  return response.json()
}

export function apiGet<T>(path: string): Promise<T> {
  return request<T>(path)
}

export function apiPost<T>(path: string, body?: unknown): Promise<T> {
  return request<T>(path, {
    method: 'POST',
    body: body ? JSON.stringify(body) : undefined,
  })
}

export function apiPut<T>(path: string, body?: unknown): Promise<T> {
  return request<T>(path, {
    method: 'PUT',
    body: body ? JSON.stringify(body) : undefined,
  })
}

export function apiDelete<T>(path: string): Promise<T> {
  return request<T>(path, { method: 'DELETE' })
}

export { APIClientError }
```

- [ ] **Step 3: 创建 useAuth composable**

File: `cmd/mts-dashboard/src/composables/useAuth.ts`

```typescript
import { ref } from 'vue'
import { setAdminToken, getAdminToken } from '@/api/client'

const isAuthenticated = ref(!!getAdminToken())
const currentUser = ref('')

export function useAuth() {
  function login(username: string, password: string) {
    // stub: 预留认证接口，当前直接放行
    void password
    setAdminToken(username)
    currentUser.value = username
    isAuthenticated.value = true
  }

  function logout() {
    setAdminToken('')
    currentUser.value = ''
    isAuthenticated.value = false
  }

  return { isAuthenticated, currentUser, login, logout }
}
```

- [ ] **Step 4: 创建 DashboardLayout**

File: `cmd/mts-dashboard/src/layouts/DashboardLayout.vue`

```vue
<script setup lang="ts">
import SidebarNav from '@/components/SidebarNav.vue'
import TopBar from '@/components/TopBar.vue'
</script>

<template>
  <div class="flex h-screen bg-slate-50">
    <SidebarNav />
    <div class="flex flex-1 flex-col overflow-hidden">
      <TopBar />
      <main class="flex-1 overflow-auto p-6">
        <RouterView />
      </main>
    </div>
  </div>
</template>
```

- [ ] **Step 5: 创建 SidebarNav**

File: `cmd/mts-dashboard/src/components/SidebarNav.vue`

```vue
<script setup lang="ts">
import { useRoute } from 'vue-router'
import {
  LayoutDashboard,
  Database,
  Users,
  Settings,
  Wrench,
  ArrowDownUp,
  Search,
  ScrollText,
  HardDrive,
} from 'lucide-vue-next'

const route = useRoute()

const navItems = [
  { to: '/', label: '仪表盘', icon: LayoutDashboard },
  { to: '/databases', label: '数据库', icon: Database },
  { to: '/users', label: '用户', icon: Users },
  { to: '/config', label: '配置', icon: Settings },
  { to: '/operations', label: '运维', icon: Wrench },
  { to: '/downsample', label: '降采样', icon: ArrowDownUp },
  { to: '/query', label: '查询', icon: Search },
  { to: '/audit', label: '审计', icon: ScrollText },
  { to: '/storage', label: '存储', icon: HardDrive },
]

function isActive(path: string): boolean {
  if (path === '/') return route.path === '/'
  return route.path.startsWith(path)
}
</script>

<template>
  <aside class="flex w-56 flex-col border-r border-slate-200 bg-white">
    <div class="flex h-14 items-center border-b border-slate-200 px-4">
      <span class="text-lg font-semibold text-slate-800">MTS Dashboard</span>
    </div>
    <nav class="flex-1 space-y-1 overflow-auto p-3">
      <RouterLink
        v-for="item in navItems"
        :key="item.to"
        :to="item.to"
        :class="[
          'flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors',
          isActive(item.to)
            ? 'bg-slate-100 text-slate-900 font-medium'
            : 'text-slate-600 hover:bg-slate-50 hover:text-slate-900',
        ]"
      >
        <component :is="item.icon" class="h-4 w-4" />
        {{ item.label }}
      </RouterLink>
    </nav>
  </aside>
</template>
```

- [ ] **Step 6: 创建 TopBar**

File: `cmd/mts-dashboard/src/components/TopBar.vue`

```vue
<script setup lang="ts">
import { useRoute } from 'vue-router'
import { useAuth } from '@/composables/useAuth'
import { computed } from 'vue'

const route = useRoute()
const { currentUser, logout } = useAuth()

const routeLabels: Record<string, string> = {
  Overview: '仪表盘',
  Databases: '数据库管理',
  Users: '用户管理',
  Config: '配置管理',
  Operations: '运维操作',
  Downsample: '降采样管理',
  Query: '数据查询',
  Audit: '审计日志',
  Storage: '存储快照',
}

const pageTitle = computed(() => {
  const name = route.name as string | undefined
  return name ? (routeLabels[name] ?? name) : ''
})
</script>

<template>
  <header class="flex h-14 items-center justify-between border-b border-slate-200 bg-white px-6">
    <h1 class="text-lg font-medium text-slate-800">{{ pageTitle }}</h1>
    <div class="flex items-center gap-4 text-sm text-slate-600">
      <span>{{ currentUser }}</span>
      <button
        class="rounded px-2 py-1 text-slate-500 hover:bg-slate-100 hover:text-slate-700"
        @click="logout"
      >
        退出
      </button>
    </div>
  </header>
</template>
```

- [ ] **Step 7: 验证路由**

Run: `cd cmd/mts-dashboard && npm run build`

Expected: 构建成功，无 TS 类型错误。

---

### Task 3: 登录页

**Files:**
- Create: `cmd/mts-dashboard/src/pages/LoginPage.vue`

- [ ] **Step 1: 创建 LoginPage**

File: `cmd/mts-dashboard/src/pages/LoginPage.vue`

```vue
<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuth } from '@/composables/useAuth'
import { Server } from 'lucide-vue-next'

const router = useRouter()
const { login } = useAuth()

const username = ref('admin')
const password = ref('admin')
const loading = ref(false)
const error = ref('')

async function handleLogin() {
  error.value = ''
  loading.value = true
  try {
    login(username.value, password.value)
    await router.push('/')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="flex min-h-screen items-center justify-center bg-slate-100">
    <div class="w-full max-w-sm rounded-xl bg-white p-8 shadow-sm">
      <div class="mb-6 flex flex-col items-center gap-2">
        <Server class="h-10 w-10 text-slate-700" />
        <h1 class="text-xl font-semibold text-slate-800">MTS Dashboard</h1>
        <p class="text-sm text-slate-500">时序数据库管理平台</p>
      </div>

      <form class="space-y-4" @submit.prevent="handleLogin">
        <div>
          <label class="mb-1 block text-sm font-medium text-slate-700" for="username">
            用户名
          </label>
          <input
            id="username"
            v-model="username"
            type="text"
            class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none focus:ring-1 focus:ring-slate-500"
            placeholder="请输入用户名"
          />
        </div>

        <div>
          <label class="mb-1 block text-sm font-medium text-slate-700" for="password">
            密码
          </label>
          <input
            id="password"
            v-model="password"
            type="password"
            class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none focus:ring-1 focus:ring-slate-500"
            placeholder="请输入密码"
          />
        </div>

        <p v-if="error" class="text-sm text-red-600">{{ error }}</p>

        <button
          type="submit"
          :disabled="loading"
          class="w-full rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50"
        >
          {{ loading ? '登录中...' : '登录' }}
        </button>
      </form>
    </div>
  </div>
</template>
```

- [ ] **Step 2: 验证构建**

Run: `cd cmd/mts-dashboard && npm run build`

Expected: 构建成功。

---

### Task 4: 仪表盘概览页

**Files:**
- Create: `cmd/mts-dashboard/src/pages/OverviewPage.vue`

- [ ] **Step 1: 创建 OverviewPage**

File: `cmd/mts-dashboard/src/pages/OverviewPage.vue`

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { apiGet } from '@/api/client'
import { Activity, Cpu, HardDrive, Server } from 'lucide-vue-next'

interface HealthResponse {
  healthy: boolean
  ready: boolean
  reasons: string[]
}

interface StorageMemoryResponse {
  snapshot: {
    current_bytes: number
    memtable_bytes: number
    wal_bytes: number
    query_bytes: number
    compaction_bytes: number
  }
}

interface CompactionStatsResponse {
  stats: {
    active: number
    backlog: number
    total: number
    success: number
    failure: number
    last_error: string
  }
}

const healthy = ref<boolean | null>(null)
const ready = ref<boolean | null>(null)
const healthReasons = ref<string[]>([])
const memorySnapshot = ref<StorageMemoryResponse['snapshot'] | null>(null)
const compactionStats = ref<CompactionStatsResponse['stats'] | null>(null)
const loadError = ref('')

function formatBytes(bytes: number): string {
  if (bytes >= 1 << 30) return (bytes / (1 << 30)).toFixed(1) + ' GB'
  if (bytes >= 1 << 20) return (bytes / (1 << 20)).toFixed(1) + ' MB'
  if (bytes >= 1 << 10) return (bytes / (1 << 10)).toFixed(1) + ' KB'
  return bytes + ' B'
}

onMounted(async () => {
  try {
    const [healthData, memData, compData] = await Promise.all([
      apiGet<HealthResponse>('/healthz'),
      apiGet<StorageMemoryResponse>('/api/v1/admin/stats/storage-memory'),
      apiGet<CompactionStatsResponse>('/api/v1/admin/stats/compaction'),
    ])
    healthy.value = healthData.healthy
    ready.value = healthData.ready
    healthReasons.value = healthData.reasons ?? []
    memorySnapshot.value = memData.snapshot
    compactionStats.value = compData.stats
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : '加载失败'
  }
})
</script>

<template>
  <div>
    <p v-if="loadError" class="mb-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">
      {{ loadError }}
    </p>

    <!-- 健康状态卡片 -->
    <div class="mb-6 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
      <div class="rounded-xl border border-slate-200 bg-white p-4">
        <div class="flex items-center gap-3">
          <Server class="h-5 w-5 text-slate-500" />
          <div>
            <p class="text-xs text-slate-500">服务状态</p>
            <p v-if="healthy === null" class="text-sm text-slate-400">加载中...</p>
            <p v-else :class="healthy ? 'text-green-600' : 'text-red-600'" class="text-sm font-medium">
              {{ healthy ? '健康' : '异常' }}
            </p>
          </div>
        </div>
      </div>

      <div class="rounded-xl border border-slate-200 bg-white p-4">
        <div class="flex items-center gap-3">
          <Activity class="h-5 w-5 text-slate-500" />
          <div>
            <p class="text-xs text-slate-500">就绪状态</p>
            <p v-if="ready === null" class="text-sm text-slate-400">加载中...</p>
            <p v-else :class="ready ? 'text-green-600' : 'text-yellow-600'" class="text-sm font-medium">
              {{ ready ? '就绪' : '未就绪' }}
            </p>
          </div>
        </div>
      </div>

      <div class="rounded-xl border border-slate-200 bg-white p-4">
        <div class="flex items-center gap-3">
          <Cpu class="h-5 w-5 text-slate-500" />
          <div>
            <p class="text-xs text-slate-500">存储内存</p>
            <p v-if="!memorySnapshot" class="text-sm text-slate-400">加载中...</p>
            <p v-else class="text-sm font-medium text-slate-700">
              {{ formatBytes(memorySnapshot.current_bytes) }}
            </p>
          </div>
        </div>
      </div>

      <div class="rounded-xl border border-slate-200 bg-white p-4">
        <div class="flex items-center gap-3">
          <HardDrive class="h-5 w-5 text-slate-500" />
          <div>
            <p class="text-xs text-slate-500">压缩任务</p>
            <p v-if="!compactionStats" class="text-sm text-slate-400">加载中...</p>
            <p v-else class="text-sm font-medium text-slate-700">
              {{ compactionStats.active }} 活跃 / {{ compactionStats.total }} 总计
            </p>
          </div>
        </div>
      </div>
    </div>

    <!-- 内存详情 -->
    <div v-if="memorySnapshot" class="mb-6 rounded-xl border border-slate-200 bg-white p-6">
      <h2 class="mb-4 text-sm font-semibold text-slate-800">存储内存详情</h2>
      <div class="grid grid-cols-2 gap-4 sm:grid-cols-4">
        <div><span class="text-xs text-slate-500">MemTable</span><p class="text-sm font-medium">{{ formatBytes(memorySnapshot.memtable_bytes) }}</p></div>
        <div><span class="text-xs text-slate-500">WAL</span><p class="text-sm font-medium">{{ formatBytes(memorySnapshot.wal_bytes) }}</p></div>
        <div><span class="text-xs text-slate-500">查询</span><p class="text-sm font-medium">{{ formatBytes(memorySnapshot.query_bytes) }}</p></div>
        <div><span class="text-xs text-slate-500">压缩</span><p class="text-sm font-medium">{{ formatBytes(memorySnapshot.compaction_bytes) }}</p></div>
      </div>
    </div>

    <!-- 压缩统计 -->
    <div v-if="compactionStats" class="rounded-xl border border-slate-200 bg-white p-6">
      <h2 class="mb-4 text-sm font-semibold text-slate-800">Compaction 统计</h2>
      <div class="grid grid-cols-2 gap-4 sm:grid-cols-4">
        <div><span class="text-xs text-slate-500">成功</span><p class="text-sm font-medium text-green-600">{{ compactionStats.success }}</p></div>
        <div><span class="text-xs text-slate-500">失败</span><p class="text-sm font-medium text-red-600">{{ compactionStats.failure }}</p></div>
        <div><span class="text-xs text-slate-500">积压</span><p class="text-sm font-medium text-yellow-600">{{ compactionStats.backlog }}</p></div>
        <div><span class="text-xs text-slate-500">活跃</span><p class="text-sm font-medium text-blue-600">{{ compactionStats.active }}</p></div>
      </div>
      <p v-if="compactionStats.last_error" class="mt-3 text-xs text-red-500">最近错误: {{ compactionStats.last_error }}</p>
    </div>

    <!-- 健康原因 -->
    <div v-if="healthReasons.length" class="mt-6 rounded-xl border border-slate-200 bg-white p-6">
      <h2 class="mb-2 text-sm font-semibold text-slate-800">健康检查详情</h2>
      <ul class="list-inside list-disc text-sm text-slate-600">
        <li v-for="reason in healthReasons" :key="reason">{{ reason }}</li>
      </ul>
    </div>
  </div>
</template>
```

- [ ] **Step 2: 验证构建**

Run: `cd cmd/mts-dashboard && npm run build`

Expected: 构建成功。

---

### Task 5: 数据库管理页

**Files:**
- Create: `cmd/mts-dashboard/src/pages/DatabasesPage.vue`

- [ ] **Step 1: 创建 DatabasesPage**

File: `cmd/mts-dashboard/src/pages/DatabasesPage.vue`

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { apiGet, apiPost, apiDelete } from '@/api/client'
import { Plus, Trash2, ChevronDown, ChevronRight } from 'lucide-vue-next'

interface MeasurementResponse { measurements: string[] }
interface RetentionPolicy { name: string; duration: number }
interface RetentionPoliciesResponse { policies: RetentionPolicy[] }

interface DatabaseEntry {
  name: string
  measurements: string[]
  retentionPolicies: RetentionPolicy[]
  expanded: boolean
  loading: boolean
}

const databases = ref<DatabaseEntry[]>([])
const newDbName = ref('')
const loadError = ref('')
const actionError = ref('')

onMounted(async () => {
  try {
    const data = await apiGet<MeasurementResponse>('/api/v1/data/databases/')
    databases.value = (data.measurements ?? []).map((name) => ({
      name,
      measurements: [],
      retentionPolicies: [],
      expanded: false,
      loading: false,
    }))
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : '加载失败'
  }
})

async function toggleExpand(db: DatabaseEntry) {
  db.expanded = !db.expanded
  if (db.expanded && !db.measurements.length && !db.retentionPolicies.length) {
    db.loading = true
    try {
      const [measData, rpData] = await Promise.all([
        apiGet<MeasurementResponse>(`/api/v1/data/databases/${db.name}/measurements`),
        apiGet<RetentionPoliciesResponse>(`/api/v1/admin/databases/${db.name}/retention-policies`),
      ])
      db.measurements = measData.measurements ?? []
      db.retentionPolicies = rpData.policies ?? []
    } catch (e) {
      actionError.value = e instanceof Error ? e.message : '加载详情失败'
    } finally {
      db.loading = false
    }
  }
}

async function createDatabase() {
  if (!newDbName.value.trim()) return
  actionError.value = ''
  try {
    await apiPost('/api/v1/admin/databases', { name: newDbName.value.trim() })
    databases.value.push({
      name: newDbName.value.trim(),
      measurements: [],
      retentionPolicies: [],
      expanded: false,
      loading: false,
    })
    newDbName.value = ''
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '创建失败'
  }
}

async function deleteDatabase(name: string) {
  if (!confirm(`确定删除数据库 ${name}？此操作不可逆。`)) return
  actionError.value = ''
  try {
    await apiDelete(`/api/v1/admin/databases/${name}`)
    databases.value = databases.value.filter((d) => d.name !== name)
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '删除失败'
  }
}
</script>

<template>
  <div>
    <p v-if="loadError" class="mb-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">{{ loadError }}</p>
    <p v-if="actionError" class="mb-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">{{ actionError }}</p>

    <div class="mb-4 flex gap-2">
      <input
        v-model="newDbName"
        type="text"
        placeholder="新数据库名称"
        class="w-64 rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none focus:ring-1 focus:ring-slate-500"
        @keyup.enter="createDatabase"
      />
      <button
        class="inline-flex items-center gap-1 rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700"
        @click="createDatabase"
      >
        <Plus class="h-4 w-4" /> 创建
      </button>
    </div>

    <div class="rounded-xl border border-slate-200 bg-white">
      <div v-if="!databases.length" class="p-6 text-center text-sm text-slate-400">
        暂无数据库
      </div>
      <div
        v-for="db in databases"
        :key="db.name"
        class="border-b border-slate-100 last:border-b-0"
      >
        <div
          class="flex items-center justify-between px-4 py-3 hover:bg-slate-50 cursor-pointer"
          @click="toggleExpand(db)"
        >
          <div class="flex items-center gap-2">
            <component :is="db.expanded ? ChevronDown : ChevronRight" class="h-4 w-4 text-slate-400" />
            <span class="text-sm font-medium text-slate-700">{{ db.name }}</span>
          </div>
          <button
            class="rounded p-1 text-slate-400 hover:bg-red-50 hover:text-red-600"
            @click.stop="deleteDatabase(db.name)"
          >
            <Trash2 class="h-4 w-4" />
          </button>
        </div>
        <div v-if="db.expanded" class="border-t border-slate-100 bg-slate-50 px-10 py-3">
          <p v-if="db.loading" class="text-sm text-slate-400">加载中...</p>
          <template v-else>
            <div v-if="db.measurements.length" class="mb-2">
              <p class="mb-1 text-xs font-medium text-slate-500">Measurements</p>
              <div class="flex flex-wrap gap-1">
                <span
                  v-for="m in db.measurements"
                  :key="m"
                  class="rounded bg-slate-200 px-2 py-0.5 text-xs text-slate-600"
                >
                  {{ m }}
                </span>
              </div>
            </div>
            <div v-if="db.retentionPolicies.length">
              <p class="mb-1 text-xs font-medium text-slate-500">保留策略</p>
              <div class="flex flex-wrap gap-1">
                <span
                  v-for="rp in db.retentionPolicies"
                  :key="rp.name"
                  class="rounded bg-blue-100 px-2 py-0.5 text-xs text-blue-700"
                >
                  {{ rp.name }} ({{ (rp.duration / 1e9 / 3600).toFixed(0) }}h)
                </span>
              </div>
            </div>
            <p v-if="!db.measurements.length && !db.retentionPolicies.length" class="text-sm text-slate-400">
              暂无详情
            </p>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>
```

- [ ] **Step 2: 验证构建**

Run: `cd cmd/mts-dashboard && npm run build`

Expected: 构建成功。

---

### Task 6: 用户管理页

**Files:**
- Create: `cmd/mts-dashboard/src/pages/UsersPage.vue`

- [ ] **Step 1: 创建 UsersPage**

File: `cmd/mts-dashboard/src/pages/UsersPage.vue`

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { apiGet, apiPost, apiPut, apiDelete } from '@/api/client'
import { Plus, Trash2, Shield, X } from 'lucide-vue-next'

interface User {
  name: string
  display_name?: string
  disabled?: boolean
}

interface UsersResponse { users: User[] }
interface UserResponse { user: User }
interface DatabaseGrant { database: string; permission: string }
interface PermissionsResponse { grants: DatabaseGrant[] }

const users = ref<User[]>([])
const loadError = ref('')
const actionError = ref('')
const showCreate = ref(false)
const newUser = ref({ name: '', display_name: '' })
const selectedUser = ref<User | null>(null)
const userGrants = ref<DatabaseGrant[]>([])
const grantDb = ref('')
const grantPerm = ref<'read' | 'write' | 'admin'>('read')

onMounted(loadUsers)

async function loadUsers() {
  try {
    const data = await apiGet<UsersResponse>('/api/v1/users')
    users.value = data.users ?? []
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : '加载失败'
  }
}

async function createUser() {
  if (!newUser.value.name.trim()) return
  actionError.value = ''
  try {
    await apiPost('/api/v1/users', { name: newUser.value.name.trim(), display_name: newUser.value.display_name })
    showCreate.value = false
    newUser.value = { name: '', display_name: '' }
    await loadUsers()
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '创建失败'
  }
}

async function deleteUser(name: string) {
  if (!confirm(`确定删除用户 ${name}？`)) return
  try {
    await apiDelete(`/api/v1/users/${name}`)
    await loadUsers()
    if (selectedUser.value?.name === name) selectedUser.value = null
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '删除失败'
  }
}

async function toggleDisable(user: User) {
  try {
    await apiPut(`/api/v1/users/${user.name}`, { ...user, disabled: !user.disabled })
    await loadUsers()
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '操作失败'
  }
}

async function selectUser(user: User) {
  selectedUser.value = user
  try {
    const data = await apiGet<PermissionsResponse>(`/api/v1/users/${user.name}/database-permissions`)
    userGrants.value = data.grants ?? []
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '加载权限失败'
  }
}

async function grantPermission() {
  if (!grantDb.value.trim() || !selectedUser.value) return
  try {
    await apiPut(`/api/v1/users/${selectedUser.value.name}/database-permissions/${grantDb.value.trim()}/${grantPerm.value}`)
    await selectUser(selectedUser.value)
    grantDb.value = ''
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '授权失败'
  }
}

async function revokePermission(database: string, permission: string) {
  if (!selectedUser.value) return
  try {
    await apiDelete(`/api/v1/users/${selectedUser.value.name}/database-permissions/${database}/${permission}`)
    await selectUser(selectedUser.value)
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '撤销失败'
  }
}
</script>

<template>
  <div class="flex gap-6">
    <!-- 用户列表 -->
    <div class="w-80 shrink-0">
      <p v-if="loadError" class="mb-3 rounded-lg border border-red-200 bg-red-50 p-2 text-xs text-red-700">{{ loadError }}</p>
      <p v-if="actionError" class="mb-3 rounded-lg border border-red-200 bg-red-50 p-2 text-xs text-red-700">{{ actionError }}</p>

      <div class="mb-3 flex gap-2">
        <button
          class="inline-flex items-center gap-1 rounded-lg bg-slate-800 px-3 py-1.5 text-xs font-medium text-white hover:bg-slate-700"
          @click="showCreate = true"
        >
          <Plus class="h-3 w-3" /> 新建用户
        </button>
      </div>

      <div class="rounded-xl border border-slate-200 bg-white">
        <div v-if="!users.length" class="p-4 text-center text-xs text-slate-400">暂无用户</div>
        <div
          v-for="user in users"
          :key="user.name"
          :class="[
            'flex items-center justify-between border-b border-slate-100 px-4 py-2.5 last:border-b-0 cursor-pointer hover:bg-slate-50',
            selectedUser?.name === user.name ? 'bg-slate-50' : '',
          ]"
          @click="selectUser(user)"
        >
          <div>
            <p class="text-sm font-medium text-slate-700">{{ user.display_name || user.name }}</p>
            <p v-if="user.disabled" class="text-xs text-red-500">已禁用</p>
          </div>
          <div class="flex items-center gap-1">
            <button
              class="rounded p-0.5 text-xs text-slate-400 hover:text-slate-600"
              :title="user.disabled ? '启用' : '禁用'"
              @click.stop="toggleDisable(user)"
            >
              {{ user.disabled ? '启用' : '禁用' }}
            </button>
            <button
              class="rounded p-0.5 text-slate-400 hover:text-red-600"
              @click.stop="deleteUser(user.name)"
            >
              <Trash2 class="h-3.5 w-3.5" />
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- 权限面板 -->
    <div class="flex-1">
      <div v-if="!selectedUser" class="flex h-48 items-center justify-center rounded-xl border border-dashed border-slate-300 bg-white text-sm text-slate-400">
        选择左侧用户查看数据库权限
      </div>
      <div v-else class="rounded-xl border border-slate-200 bg-white p-6">
        <h3 class="mb-4 text-sm font-semibold text-slate-800">
          {{ selectedUser.display_name || selectedUser.name }} 的数据库权限
        </h3>

        <div v-if="!userGrants.length" class="mb-4 text-xs text-slate-400">暂无权限</div>
        <div v-else class="mb-4 space-y-1">
          <div
            v-for="grant in userGrants"
            :key="`${grant.database}-${grant.permission}`"
            class="flex items-center justify-between rounded bg-slate-50 px-3 py-1.5"
          >
            <span class="text-sm text-slate-700">
              <span class="font-medium">{{ grant.database }}</span>
              <span class="ml-2 rounded bg-slate-200 px-1.5 py-0.5 text-xs text-slate-600">{{ grant.permission }}</span>
            </span>
            <button class="text-slate-400 hover:text-red-600" @click="revokePermission(grant.database, grant.permission)">
              <X class="h-3.5 w-3.5" />
            </button>
          </div>
        </div>

        <div class="flex gap-2">
          <input
            v-model="grantDb"
            type="text"
            placeholder="数据库名"
            class="w-40 rounded-lg border border-slate-300 px-3 py-1.5 text-xs focus:border-slate-500 focus:outline-none"
          />
          <select
            v-model="grantPerm"
            class="rounded-lg border border-slate-300 px-2 py-1.5 text-xs focus:border-slate-500 focus:outline-none"
          >
            <option value="read">read</option>
            <option value="write">write</option>
            <option value="admin">admin</option>
          </select>
          <button
            class="inline-flex items-center gap-1 rounded-lg bg-slate-800 px-3 py-1.5 text-xs font-medium text-white hover:bg-slate-700"
            @click="grantPermission"
          >
            <Shield class="h-3 w-3" /> 授权
          </button>
        </div>
      </div>
    </div>
  </div>

  <!-- 创建用户对话框 -->
  <div v-if="showCreate" class="fixed inset-0 z-50 flex items-center justify-center bg-black/30" @click.self="showCreate = false">
    <div class="w-80 rounded-xl bg-white p-6 shadow-lg">
      <h3 class="mb-4 text-sm font-semibold text-slate-800">创建用户</h3>
      <div class="space-y-3">
        <input
          v-model="newUser.name"
          type="text"
          placeholder="用户名 (必填)"
          class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none"
          @keyup.enter="createUser"
        />
        <input
          v-model="newUser.display_name"
          type="text"
          placeholder="显示名 (可选)"
          class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-slate-500 focus:outline-none"
        />
      </div>
      <div class="mt-4 flex justify-end gap-2">
        <button class="rounded-lg px-4 py-2 text-sm text-slate-600 hover:bg-slate-100" @click="showCreate = false">
          取消
        </button>
        <button class="rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700" @click="createUser">
          创建
        </button>
      </div>
    </div>
  </div>
</template>
```

- [ ] **Step 2: 验证构建**

Run: `cd cmd/mts-dashboard && npm run build`

Expected: 构建成功。

---

### Task 7: 配置管理页

**Files:**
- Create: `cmd/mts-dashboard/src/pages/ConfigPage.vue`

- [ ] **Step 1: 创建 ConfigPage**

File: `cmd/mts-dashboard/src/pages/ConfigPage.vue`

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { apiGet, apiPost } from '@/api/client'
import { RefreshCw, CheckCircle, AlertCircle } from 'lucide-vue-next'

interface ConfigResponse { config: Record<string, unknown> }
interface ValidateResponse { ok: boolean; error?: string }
interface ReloadResponse { ok: boolean; fields: string[] }
interface ErrorCodeSpec { code: string; http_status: number; grpc_code: string; description: string }
interface ErrorCodesResponse { codes: ErrorCodeSpec[] }

const config = ref<Record<string, unknown> | null>(null)
const validateResult = ref<ValidateResponse | null>(null)
const reloadResult = ref<ReloadResponse | null>(null)
const errorCodes = ref<ErrorCodeSpec[]>([])
const loadError = ref('')
const actionError = ref('')

onMounted(async () => {
  try {
    const [cfgData, ecData] = await Promise.all([
      apiGet<ConfigResponse>('/api/v1/admin/config/effective'),
      apiGet<ErrorCodesResponse>('/api/v1/admin/error-codes'),
    ])
    config.value = cfgData.config
    errorCodes.value = ecData.codes ?? []
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : '加载失败'
  }
})

async function handleValidate() {
  actionError.value = ''
  validateResult.value = null
  try {
    validateResult.value = await apiPost<ValidateResponse>('/api/v1/admin/config/validate', { config: config.value })
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '验证失败'
  }
}

async function handleReload() {
  actionError.value = ''
  reloadResult.value = null
  try {
    reloadResult.value = await apiPost<ReloadResponse>('/api/v1/admin/config/reload')
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '重载失败'
  }
}

function statusLabel(httpStatus: number): string {
  if (httpStatus >= 200 && httpStatus < 300) return '成功'
  if (httpStatus >= 400 && httpStatus < 500) return '客户端错误'
  if (httpStatus >= 500) return '服务端错误'
  return ''
}
</script>

<template>
  <div class="space-y-6">
    <p v-if="loadError" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">{{ loadError }}</p>
    <p v-if="actionError" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">{{ actionError }}</p>

    <!-- 操作按钮 -->
    <div class="flex gap-3">
      <button
        class="inline-flex items-center gap-2 rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
        @click="handleValidate"
      >
        <CheckCircle class="h-4 w-4" /> 验证配置
      </button>
      <button
        class="inline-flex items-center gap-2 rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700"
        @click="handleReload"
      >
        <RefreshCw class="h-4 w-4" /> 热重载
      </button>
    </div>

    <!-- 验证/重载结果 -->
    <div v-if="validateResult" :class="validateResult.ok ? 'border-green-200 bg-green-50' : 'border-red-200 bg-red-50'" class="rounded-lg border p-3">
      <p v-if="validateResult.ok" class="text-sm text-green-700">配置验证通过</p>
      <p v-else class="text-sm text-red-700">配置验证失败: {{ validateResult.error }}</p>
    </div>
    <div v-if="reloadResult" class="rounded-lg border border-green-200 bg-green-50 p-3">
      <p class="text-sm text-green-700">
        配置已重载
        <span v-if="reloadResult.fields?.length">，变更字段: {{ reloadResult.fields.join(', ') }}</span>
      </p>
    </div>

    <!-- 当前有效配置 -->
    <div v-if="config" class="rounded-xl border border-slate-200 bg-white p-6">
      <h2 class="mb-4 text-sm font-semibold text-slate-800">有效配置</h2>
      <pre class="overflow-auto rounded-lg bg-slate-900 p-4 text-xs text-green-400 max-h-96">{{ JSON.stringify(config, null, 2) }}</pre>
    </div>

    <!-- 错误码契约 -->
    <div class="rounded-xl border border-slate-200 bg-white p-6">
      <h2 class="mb-4 text-sm font-semibold text-slate-800">错误码契约</h2>
      <div v-if="!errorCodes.length" class="text-sm text-slate-400">暂无数据</div>
      <table v-else class="w-full text-sm">
        <thead>
          <tr class="border-b border-slate-200 text-left">
            <th class="pb-2 text-xs font-medium text-slate-500">Code</th>
            <th class="pb-2 text-xs font-medium text-slate-500">HTTP</th>
            <th class="pb-2 text-xs font-medium text-slate-500">gRPC</th>
            <th class="pb-2 text-xs font-medium text-slate-500">说明</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="ec in errorCodes" :key="ec.code" class="border-b border-slate-100 last:border-b-0">
            <td class="py-2 font-mono text-xs text-slate-700">{{ ec.code }}</td>
            <td class="py-2">
              <span class="rounded bg-slate-100 px-1.5 py-0.5 text-xs text-slate-600">{{ ec.http_status }} {{ statusLabel(ec.http_status) }}</span>
            </td>
            <td class="py-2 font-mono text-xs text-slate-600">{{ ec.grpc_code }}</td>
            <td class="py-2 text-xs text-slate-600">{{ ec.description }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
```

- [ ] **Step 2: 验证构建**

Run: `cd cmd/mts-dashboard && npm run build`

Expected: 构建成功。

---

### Task 8: 运维操作页

**Files:**
- Create: `cmd/mts-dashboard/src/pages/OperationsPage.vue`

- [ ] **Step 1: 创建 OperationsPage**

File: `cmd/mts-dashboard/src/pages/OperationsPage.vue`

```vue
<script setup lang="ts">
import { ref } from 'vue'
import { apiPost, apiGet } from '@/api/client'
import { HardDrive, ArrowDownWideNarrow, Archive } from 'lucide-vue-next'

interface OpResult { ok: boolean; result?: Record<string, unknown> }
interface MaintenanceErrorsResponse { errors: string[] }

const flushResult = ref<string>('')
const compactResult = ref<string>('')
const retentionResult = ref<string>('')
const maintenanceErrors = ref<string[]>([])
const actionLoading = ref('')

async function doFlush() {
  actionLoading.value = 'flush'
  try {
    const res = await apiPost<OpResult>('/api/v1/admin/flush')
    flushResult.value = res.ok ? 'Flush 执行成功' : 'Flush 执行失败'
  } catch (e) {
    flushResult.value = `失败: ${e instanceof Error ? e.message : '未知错误'}`
  } finally {
    actionLoading.value = ''
  }
}

async function doCompact() {
  actionLoading.value = 'compact'
  try {
    const res = await apiPost<OpResult>('/api/v1/admin/compact')
    compactResult.value = res.ok ? 'Compact 执行成功' : 'Compact 执行失败'
  } catch (e) {
    compactResult.value = `失败: ${e instanceof Error ? e.message : '未知错误'}`
  } finally {
    actionLoading.value = ''
  }
}

async function doApplyRetention() {
  actionLoading.value = 'retention'
  try {
    const res = await apiPost<OpResult>('/api/v1/admin/retention/apply', { now_unix_nanos: 0 })
    retentionResult.value = res.ok ? '保留策略应用成功' : '保留策略应用失败'
  } catch (e) {
    retentionResult.value = `失败: ${e instanceof Error ? e.message : '未知错误'}`
  } finally {
    actionLoading.value = ''
  }
}

async function loadMaintenanceErrors() {
  actionLoading.value = 'errors'
  try {
    const res = await apiGet<MaintenanceErrorsResponse>('/api/v1/admin/maintenance/errors')
    maintenanceErrors.value = res.errors ?? []
  } catch (e) {
    maintenanceErrors.value = [`加载失败: ${e instanceof Error ? e.message : '未知错误'}`]
  } finally {
    actionLoading.value = ''
  }
}
</script>

<template>
  <div class="space-y-6">
    <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
      <!-- Flush -->
      <div class="rounded-xl border border-slate-200 bg-white p-6">
        <div class="mb-3 flex items-center gap-2">
          <HardDrive class="h-5 w-5 text-slate-500" />
          <h3 class="text-sm font-semibold text-slate-800">Flush</h3>
        </div>
        <p class="mb-4 text-xs text-slate-500">将 MemTable 数据刷写到 SSTable</p>
        <button
          :disabled="actionLoading === 'flush'"
          class="w-full rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50"
          @click="doFlush"
        >
          {{ actionLoading === 'flush' ? '执行中...' : '执行 Flush' }}
        </button>
        <p v-if="flushResult" class="mt-2 text-xs" :class="flushResult.includes('失败') ? 'text-red-600' : 'text-green-600'">
          {{ flushResult }}
        </p>
      </div>

      <!-- Compact -->
      <div class="rounded-xl border border-slate-200 bg-white p-6">
        <div class="mb-3 flex items-center gap-2">
          <ArrowDownWideNarrow class="h-5 w-5 text-slate-500" />
          <h3 class="text-sm font-semibold text-slate-800">Compact</h3>
        </div>
        <p class="mb-4 text-xs text-slate-500">触发 Compaction 合并 SSTable 文件</p>
        <button
          :disabled="actionLoading === 'compact'"
          class="w-full rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50"
          @click="doCompact"
        >
          {{ actionLoading === 'compact' ? '执行中...' : '执行 Compact' }}
        </button>
        <p v-if="compactResult" class="mt-2 text-xs" :class="compactResult.includes('失败') ? 'text-red-600' : 'text-green-600'">
          {{ compactResult }}
        </p>
      </div>

      <!-- Retention -->
      <div class="rounded-xl border border-slate-200 bg-white p-6">
        <div class="mb-3 flex items-center gap-2">
          <Archive class="h-5 w-5 text-slate-500" />
          <h3 class="text-sm font-semibold text-slate-800">保留策略</h3>
        </div>
        <p class="mb-4 text-xs text-slate-500">应用保留策略删除过期数据</p>
        <button
          :disabled="actionLoading === 'retention'"
          class="w-full rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50"
          @click="doApplyRetention"
        >
          {{ actionLoading === 'retention' ? '执行中...' : '应用保留策略' }}
        </button>
        <p v-if="retentionResult" class="mt-2 text-xs" :class="retentionResult.includes('失败') ? 'text-red-600' : 'text-green-600'">
          {{ retentionResult }}
        </p>
      </div>
    </div>

    <!-- 维护错误 -->
    <div class="rounded-xl border border-slate-200 bg-white p-6">
      <div class="mb-4 flex items-center justify-between">
        <h3 class="text-sm font-semibold text-slate-800">维护错误</h3>
        <button
          :disabled="actionLoading === 'errors'"
          class="rounded-lg bg-slate-100 px-3 py-1.5 text-xs text-slate-600 hover:bg-slate-200"
          @click="loadMaintenanceErrors"
        >
          {{ actionLoading === 'errors' ? '加载中...' : '刷新' }}
        </button>
      </div>
      <div v-if="!maintenanceErrors.length" class="text-sm text-slate-400">
        暂无维护错误（点击刷新加载）
      </div>
      <ul v-else class="space-y-1">
        <li v-for="(err, idx) in maintenanceErrors" :key="idx" class="rounded bg-red-50 px-3 py-1.5 text-xs text-red-700">
          {{ err }}
        </li>
      </ul>
    </div>
  </div>
</template>
```

- [ ] **Step 2: 验证构建**

Run: `cd cmd/mts-dashboard && npm run build`

Expected: 构建成功。

---

### Task 9: 降采样管理页

**Files:**
- Create: `cmd/mts-dashboard/src/pages/DownsamplePage.vue`

- [ ] **Step 1: 创建 DownsamplePage**

File: `cmd/mts-dashboard/src/pages/DownsamplePage.vue`

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { apiGet, apiPost, apiPut, apiDelete } from '@/api/client'
import { Plus, Trash2, Play, Pause, RefreshCw } from 'lucide-vue-next'

interface DownsamplePolicy {
  name: string
  source_database: string
  source_retention: string
  source_measurement: string
  target_database: string
  target_retention: string
  target_measurement: string
  interval: number
  functions: { function: string; field: string; as: string }[]
  group_by_tags: string[]
  enabled: boolean
}

interface DownsampleStatus {
  policy_name: string
  completed_until_unix: number
  last_run_unix: number
  last_error: string
}

interface PoliciesResponse { policies: DownsamplePolicy[] }
interface StatusesResponse { statuses: DownsampleStatus[] }

const policies = ref<DownsamplePolicy[]>([])
const statuses = ref<DownsampleStatus[]>([])
const loadError = ref('')
const actionError = ref('')
const showCreate = ref(false)
const newPolicy = ref<DownsamplePolicy>({
  name: '',
  source_database: '',
  source_retention: 'autogen',
  source_measurement: '',
  target_database: '',
  target_retention: 'autogen',
  target_measurement: '',
  interval: 60000000000,
  functions: [{ function: 'mean', field: 'value', as: 'mean_value' }],
  group_by_tags: [],
  enabled: true,
})

onMounted(loadData)

async function loadData() {
  try {
    const [polData, statData] = await Promise.all([
      apiGet<PoliciesResponse>('/api/v1/admin/downsample/policies'),
      apiGet<StatusesResponse>('/api/v1/admin/downsample/statuses'),
    ])
    policies.value = polData.policies ?? []
    statuses.value = statData.statuses ?? []
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : '加载失败'
  }
}

async function createPolicy() {
  if (!newPolicy.value.name.trim()) return
  actionError.value = ''
  try {
    await apiPost('/api/v1/admin/downsample/policies', { name: newPolicy.value.name, ...newPolicy.value })
    showCreate.value = false
    await loadData()
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '创建失败'
  }
}

async function deletePolicy(name: string) {
  if (!confirm(`确定删除降采样策略 ${name}？`)) return
  try {
    const opts = prompt('清理目标数据？(y/n)', 'n') === 'y' ? { cleanup_target: true } : {}
    await apiDelete(`/api/v1/admin/downsample/policies/${name}`)
    await loadData()
    void opts
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '删除失败'
  }
}

async function togglePolicy(policy: DownsamplePolicy) {
  const action = policy.enabled ? 'pause' : 'resume'
  try {
    await apiPost(`/api/v1/admin/downsample/policies/${policy.name}/${action}`)
    await loadData()
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '操作失败'
  }
}

function formatDuration(ns: number): string {
  if (ns >= 3600e9) return (ns / 3600e9).toFixed(1) + 'h'
  if (ns >= 60e9) return (ns / 60e9).toFixed(0) + 'm'
  if (ns >= 1e9) return (ns / 1e9).toFixed(0) + 's'
  return ns + 'ns'
}

function formatUnix(ts: number): string {
  if (!ts) return '-'
  return new Date(ts * 1000).toLocaleString()
}

function getStatus(name: string): DownsampleStatus | undefined {
  return statuses.value.find((s) => s.policy_name === name)
}
</script>

<template>
  <div class="space-y-6">
    <p v-if="loadError" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">{{ loadError }}</p>
    <p v-if="actionError" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">{{ actionError }}</p>

    <div class="flex gap-2">
      <button class="inline-flex items-center gap-1 rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700" @click="showCreate = true">
        <Plus class="h-4 w-4" /> 创建策略
      </button>
      <button class="inline-flex items-center gap-1 rounded-lg border border-slate-300 px-4 py-2 text-sm text-slate-600 hover:bg-slate-50" @click="loadData">
        <RefreshCw class="h-4 w-4" /> 刷新
      </button>
    </div>

    <!-- 策略列表 -->
    <div class="rounded-xl border border-slate-200 bg-white">
      <div v-if="!policies.length" class="p-6 text-center text-sm text-slate-400">暂无降采样策略</div>
      <table v-else class="w-full text-sm">
        <thead>
          <tr class="border-b border-slate-200 text-left">
            <th class="px-4 py-3 text-xs font-medium text-slate-500">名称</th>
            <th class="px-4 py-3 text-xs font-medium text-slate-500">源 → 目标</th>
            <th class="px-4 py-3 text-xs font-medium text-slate-500">间隔</th>
            <th class="px-4 py-3 text-xs font-medium text-slate-500">状态</th>
            <th class="px-4 py-3 text-xs font-medium text-slate-500">进度</th>
            <th class="px-4 py-3 text-xs font-medium text-slate-500">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="policy in policies" :key="policy.name" class="border-b border-slate-100 last:border-b-0">
            <td class="px-4 py-3 font-medium text-slate-700">{{ policy.name }}</td>
            <td class="px-4 py-3 text-slate-600">
              {{ policy.source_database }}/{{ policy.source_measurement }}
              → {{ policy.target_database }}/{{ policy.target_measurement }}
            </td>
            <td class="px-4 py-3 text-slate-600">{{ formatDuration(policy.interval) }}</td>
            <td class="px-4 py-3">
              <span :class="policy.enabled ? 'bg-green-100 text-green-700' : 'bg-slate-100 text-slate-500'" class="rounded px-2 py-0.5 text-xs font-medium">
                {{ policy.enabled ? '运行中' : '已暂停' }}
              </span>
            </td>
            <td class="px-4 py-3 text-xs text-slate-500">
              <template v-if="getStatus(policy.name)">
                完成于 {{ formatUnix(getStatus(policy.name)!.completed_until_unix) }}
              </template>
              <template v-else>-</template>
            </td>
            <td class="px-4 py-3">
              <div class="flex items-center gap-1">
                <button class="rounded p-1 text-slate-400 hover:text-slate-600" :title="policy.enabled ? '暂停' : '恢复'" @click="togglePolicy(policy)">
                  <component :is="policy.enabled ? Pause : Play" class="h-4 w-4" />
                </button>
                <button class="rounded p-1 text-slate-400 hover:text-red-600" @click="deletePolicy(policy.name)">
                  <Trash2 class="h-4 w-4" />
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- 创建策略对话框 -->
    <div v-if="showCreate" class="fixed inset-0 z-50 flex items-center justify-center bg-black/30" @click.self="showCreate = false">
      <div class="w-[480px] max-h-[80vh] overflow-auto rounded-xl bg-white p-6 shadow-lg">
        <h3 class="mb-4 text-sm font-semibold text-slate-800">创建降采样策略</h3>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="mb-1 block text-xs text-slate-500">名称</label>
            <input v-model="newPolicy.name" class="w-full rounded border border-slate-300 px-2 py-1.5 text-xs" />
          </div>
          <div>
            <label class="mb-1 block text-xs text-slate-500">源数据库</label>
            <input v-model="newPolicy.source_database" class="w-full rounded border border-slate-300 px-2 py-1.5 text-xs" />
          </div>
          <div>
            <label class="mb-1 block text-xs text-slate-500">源 Measurement</label>
            <input v-model="newPolicy.source_measurement" class="w-full rounded border border-slate-300 px-2 py-1.5 text-xs" />
          </div>
          <div>
            <label class="mb-1 block text-xs text-slate-500">目标数据库</label>
            <input v-model="newPolicy.target_database" class="w-full rounded border border-slate-300 px-2 py-1.5 text-xs" />
          </div>
          <div>
            <label class="mb-1 block text-xs text-slate-500">目标 Measurement</label>
            <input v-model="newPolicy.target_measurement" class="w-full rounded border border-slate-300 px-2 py-1.5 text-xs" />
          </div>
          <div>
            <label class="mb-1 block text-xs text-slate-500">间隔 (纳秒)</label>
            <input v-model.number="newPolicy.interval" type="number" class="w-full rounded border border-slate-300 px-2 py-1.5 text-xs" />
          </div>
        </div>
        <div class="mt-4 flex justify-end gap-2">
          <button class="rounded-lg px-4 py-2 text-sm text-slate-600 hover:bg-slate-100" @click="showCreate = false">取消</button>
          <button class="rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700" @click="createPolicy">创建</button>
        </div>
      </div>
    </div>
  </div>
</template>
```

- [ ] **Step 2: 验证构建**

Run: `cd cmd/mts-dashboard && npm run build`

Expected: 构建成功。

---

### Task 10: 数据查询页

**Files:**
- Create: `cmd/mts-dashboard/src/pages/QueryPage.vue`

- [ ] **Step 1: 创建 QueryPage**

File: `cmd/mts-dashboard/src/pages/QueryPage.vue`

```vue
<script setup lang="ts">
import { ref } from 'vue'
import { apiPost, apiGet } from '@/api/client'
import { Search } from 'lucide-vue-next'

interface QueryResultRow {
  series_id: number
  measurement: string
  tags: Record<string, string>
  timestamp: number
  fields: Record<string, unknown>
}

interface QueryStatsData {
  candidate_shards: number
  shards_scanned: number
  shards_skipped: number
  parts_scanned: number
  parts_skipped: number
  samples_read: number
  samples_returned: number
  duration_nanos: number
  errors: number
}

interface RowsResponse { rows: QueryResultRow[] }
interface StatsResponse { stats: QueryStatsData }

const queryForm = ref({
  database: '',
  measurement: '',
  start_unix_nanos: '',
  end_unix_nanos: '',
  fields: '',
  limit: '100',
})

const queryMode = ref<'rows' | 'columns' | 'explain' | 'stream'>('rows')
const rows = ref<QueryResultRow[]>([])
const queryStats = ref<QueryStatsData | null>(null)
const rawOutput = ref('')
const actionError = ref('')
const loading = ref(false)

async function executeQuery() {
  actionError.value = ''
  loading.value = true
  rows.value = []
  queryStats.value = null
  rawOutput.value = ''

  const query: Record<string, unknown> = {}
  if (queryForm.value.database) query.database = queryForm.value.database
  if (queryForm.value.measurement) query.measurement = queryForm.value.measurement
  if (queryForm.value.start_unix_nanos) query.start_unix_nanos = parseInt(queryForm.value.start_unix_nanos)
  if (queryForm.value.end_unix_nanos) query.end_unix_nanos = parseInt(queryForm.value.end_unix_nanos)
  if (queryForm.value.fields) query.fields = queryForm.value.fields.split(',').map((s) => s.trim())
  if (queryForm.value.limit) query.limit = parseInt(queryForm.value.limit)

  try {
    if (queryMode.value === 'rows') {
      const data = await apiPost<RowsResponse>('/api/v1/data/query/rows', { query })
      rows.value = data.rows ?? []
    } else if (queryMode.value === 'explain') {
      const data = await apiPost<{ result: { columns: unknown[]; explain: Record<string, unknown>; stats: QueryStatsData } }>('/api/v1/data/query/explain', { query })
      rows.value = (data.result?.columns as QueryResultRow[]) ?? []
      queryStats.value = data.result?.stats ?? null
    } else if (queryMode.value === 'columns') {
      const data = await apiPost<{ columns: unknown[] }>('/api/v1/data/query/columns', { query })
      rawOutput.value = JSON.stringify(data.columns, null, 2)
    } else if (queryMode.value === 'stream') {
      const token = (await import('@/api/client')).getAdminToken()
      const resp = await fetch('/api/v1/data/query/stream', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-MTS-Admin-Token': token,
        },
        body: JSON.stringify({ query }),
      })
      rawOutput.value = await resp.text()
    }
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '查询失败'
  } finally {
    loading.value = false
  }

  // 获取查询统计
  try {
    const statsData = await apiGet<StatsResponse>('/api/v1/data/query/stats')
    queryStats.value = statsData.stats
  } catch (_) {
    // 统计获取失败不影响主流程
  }
}

function formatTimestamp(ns: number): string {
  return new Date(ns / 1e6).toISOString()
}
</script>

<template>
  <div class="space-y-6">
    <p v-if="actionError" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">{{ actionError }}</p>

    <!-- 查询表单 -->
    <div class="rounded-xl border border-slate-200 bg-white p-6">
      <h3 class="mb-4 text-sm font-semibold text-slate-800">查询条件</h3>
      <div class="mb-4 flex gap-3">
        <select v-model="queryMode" class="rounded-lg border border-slate-300 px-3 py-2 text-sm">
          <option value="rows">行式查询</option>
          <option value="columns">列式查询</option>
          <option value="explain">EXPLAIN</option>
          <option value="stream">流式查询</option>
        </select>
      </div>
      <div class="grid grid-cols-2 gap-3 lg:grid-cols-3">
        <div>
          <label class="mb-1 block text-xs text-slate-500">数据库</label>
          <input v-model="queryForm.database" placeholder="default" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm" />
        </div>
        <div>
          <label class="mb-1 block text-xs text-slate-500">Measurement</label>
          <input v-model="queryForm.measurement" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm" />
        </div>
        <div>
          <label class="mb-1 block text-xs text-slate-500">开始时间 (纳秒)</label>
          <input v-model="queryForm.start_unix_nanos" placeholder="0" type="number" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm" />
        </div>
        <div>
          <label class="mb-1 block text-xs text-slate-500">结束时间 (纳秒)</label>
          <input v-model="queryForm.end_unix_nanos" placeholder="now" type="number" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm" />
        </div>
        <div>
          <label class="mb-1 block text-xs text-slate-500">字段 (逗号分隔)</label>
          <input v-model="queryForm.fields" placeholder="value" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm" />
        </div>
        <div>
          <label class="mb-1 block text-xs text-slate-500">Limit</label>
          <input v-model="queryForm.limit" type="number" class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm" />
        </div>
      </div>
      <button
        :disabled="loading"
        class="mt-4 inline-flex items-center gap-2 rounded-lg bg-slate-800 px-6 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50"
        @click="executeQuery"
      >
        <Search class="h-4 w-4" />
        {{ loading ? '查询中...' : '执行查询' }}
      </button>
    </div>

    <!-- 查询统计 -->
    <div v-if="queryStats" class="rounded-xl border border-slate-200 bg-white p-6">
      <h3 class="mb-3 text-sm font-semibold text-slate-800">查询统计</h3>
      <div class="grid grid-cols-3 gap-3 sm:grid-cols-5">
        <div><span class="text-xs text-slate-500">扫描 Shards</span><p class="text-sm font-medium">{{ queryStats.shards_scanned }}</p></div>
        <div><span class="text-xs text-slate-500">跳过 Shards</span><p class="text-sm font-medium">{{ queryStats.shards_skipped }}</p></div>
        <div><span class="text-xs text-slate-500">读取样本</span><p class="text-sm font-medium">{{ queryStats.samples_read }}</p></div>
        <div><span class="text-xs text-slate-500">返回样本</span><p class="text-sm font-medium">{{ queryStats.samples_returned }}</p></div>
        <div><span class="text-xs text-slate-500">耗时</span><p class="text-sm font-medium">{{ (queryStats.duration_nanos / 1e6).toFixed(2) }}ms</p></div>
      </div>
    </div>

    <!-- 查询结果 -->
    <div v-if="rows.length" class="rounded-xl border border-slate-200 bg-white p-6">
      <h3 class="mb-3 text-sm font-semibold text-slate-800">结果 ({{ rows.length }} 行)</h3>
      <div class="overflow-auto max-h-96">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-slate-200 text-left">
              <th class="pb-2 pr-4 text-xs font-medium text-slate-500">时间</th>
              <th class="pb-2 pr-4 text-xs font-medium text-slate-500">Measurement</th>
              <th class="pb-2 pr-4 text-xs font-medium text-slate-500">Tags</th>
              <th class="pb-2 text-xs font-medium text-slate-500">Fields</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(row, idx) in rows" :key="idx" class="border-b border-slate-100 last:border-b-0">
              <td class="py-2 pr-4 font-mono text-xs text-slate-600">{{ formatTimestamp(row.timestamp) }}</td>
              <td class="py-2 pr-4 text-xs text-slate-600">{{ row.measurement }}</td>
              <td class="py-2 pr-4 text-xs text-slate-500">{{ JSON.stringify(row.tags) }}</td>
              <td class="py-2 text-xs text-slate-600">{{ JSON.stringify(row.fields) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- 原始输出 -->
    <div v-if="rawOutput" class="rounded-xl border border-slate-200 bg-white p-6">
      <h3 class="mb-3 text-sm font-semibold text-slate-800">原始输出</h3>
      <pre class="overflow-auto rounded-lg bg-slate-900 p-4 text-xs text-green-400 max-h-96">{{ rawOutput }}</pre>
    </div>
  </div>
</template>
```

- [ ] **Step 2: 验证构建**

Run: `cd cmd/mts-dashboard && npm run build`

Expected: 构建成功。

---

### Task 11: 审计日志页

**Files:**
- Create: `cmd/mts-dashboard/src/pages/AuditPage.vue`

- [ ] **Step 1: 创建 AuditPage**

File: `cmd/mts-dashboard/src/pages/AuditPage.vue`

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { apiGet } from '@/api/client'
import { ScrollText } from 'lucide-vue-next'

interface User { name: string; display_name?: string }
interface UsersResponse { users: User[] }
interface AuditEvent {
  timestamp: string
  user: string
  action: string
  detail: string
}

const users = ref<User[]>([])
const selectedUser = ref('')
const auditEvents = ref<AuditEvent[]>([])
const loadError = ref('')
const loading = ref(false)

onMounted(async () => {
  try {
    const data = await apiGet<UsersResponse>('/api/v1/users')
    users.value = data.users ?? []
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : '加载用户失败'
  }
})

async function loadAudit() {
  if (!selectedUser.value) return
  loading.value = true
  try {
    const data = await apiGet<AuditEvent[]>(`/api/v1/users/${selectedUser.value}/audit`)
    auditEvents.value = Array.isArray(data) ? data : []
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : '加载审计日志失败'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="space-y-6">
    <p v-if="loadError" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">{{ loadError }}</p>

    <div class="flex items-end gap-3">
      <div>
        <label class="mb-1 block text-xs text-slate-500">选择用户</label>
        <select v-model="selectedUser" class="w-48 rounded-lg border border-slate-300 px-3 py-2 text-sm" @change="loadAudit">
          <option value="">-- 选择用户 --</option>
          <option v-for="user in users" :key="user.name" :value="user.name">
            {{ user.display_name || user.name }}
          </option>
        </select>
      </div>
      <button
        :disabled="!selectedUser || loading"
        class="rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50"
        @click="loadAudit"
      >
        {{ loading ? '加载中...' : '查询' }}
      </button>
    </div>

    <div class="rounded-xl border border-slate-200 bg-white">
      <div v-if="!auditEvents.length && selectedUser" class="p-6 text-center text-sm text-slate-400">
        {{ loading ? '加载中...' : '暂无审计记录' }}
      </div>
      <div v-else-if="!selectedUser" class="p-6 text-center text-sm text-slate-400">
        请选择用户查看审计日志
      </div>
      <table v-else class="w-full text-sm">
        <thead>
          <tr class="border-b border-slate-200 text-left">
            <th class="px-4 py-3 text-xs font-medium text-slate-500">时间</th>
            <th class="px-4 py-3 text-xs font-medium text-slate-500">操作</th>
            <th class="px-4 py-3 text-xs font-medium text-slate-500">详情</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(evt, idx) in auditEvents" :key="idx" class="border-b border-slate-100 last:border-b-0">
            <td class="px-4 py-3 text-xs text-slate-600">{{ evt.timestamp }}</td>
            <td class="px-4 py-3 text-xs font-medium text-slate-700">{{ evt.action }}</td>
            <td class="px-4 py-3 text-xs text-slate-500">{{ evt.detail }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
```

- [ ] **Step 2: 验证构建**

Run: `cd cmd/mts-dashboard && npm run build`

Expected: 构建成功。

---

### Task 12: 存储快照页

**Files:**
- Create: `cmd/mts-dashboard/src/pages/StoragePage.vue`

- [ ] **Step 1: 创建 StoragePage**

File: `cmd/mts-dashboard/src/pages/StoragePage.vue`

```vue
<script setup lang="ts">
import { ref } from 'vue'
import { apiPost, apiGet } from '@/api/client'
import { CheckCircle, Camera, Download, AlertCircle } from 'lucide-vue-next'

interface ValidateResponse { ok: boolean; data_dir: string; health: Record<string, unknown> }
interface SnapshotResponse { ok: boolean; path: string }
interface ExportData { generated_at: string; config: Record<string, unknown>; health: Record<string, unknown> }
interface ExportResponse { export: ExportData }

const validateResult = ref<ValidateResponse | null>(null)
const snapshotResult = ref<SnapshotResponse | null>(null)
const exportData = ref<ExportData | null>(null)
const actionError = ref('')
const loading = ref('')

async function doValidate() {
  loading.value = 'validate'
  actionError.value = ''
  try {
    validateResult.value = await apiPost<ValidateResponse>('/api/v1/admin/storage/validate')
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '验证失败'
  } finally {
    loading.value = ''
  }
}

async function doSnapshot() {
  loading.value = 'snapshot'
  actionError.value = ''
  try {
    snapshotResult.value = await apiPost<SnapshotResponse>('/api/v1/admin/storage/snapshot')
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '快照失败'
  } finally {
    loading.value = ''
  }
}

async function doExport() {
  loading.value = 'export'
  actionError.value = ''
  try {
    const data = await apiGet<ExportResponse>('/api/v1/admin/storage/export')
    exportData.value = data.export
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : '导出失败'
  } finally {
    loading.value = ''
  }
}
</script>

<template>
  <div class="space-y-6">
    <p v-if="actionError" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">{{ actionError }}</p>

    <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
      <!-- 验证 -->
      <div class="rounded-xl border border-slate-200 bg-white p-6">
        <div class="mb-3 flex items-center gap-2">
          <CheckCircle class="h-5 w-5 text-slate-500" />
          <h3 class="text-sm font-semibold text-slate-800">存储验证</h3>
        </div>
        <p class="mb-4 text-xs text-slate-500">验证文件结构完整性</p>
        <button
          :disabled="loading === 'validate'"
          class="w-full rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50"
          @click="doValidate"
        >
          {{ loading === 'validate' ? '验证中...' : '执行验证' }}
        </button>
      </div>

      <!-- 快照 -->
      <div class="rounded-xl border border-slate-200 bg-white p-6">
        <div class="mb-3 flex items-center gap-2">
          <Camera class="h-5 w-5 text-slate-500" />
          <h3 class="text-sm font-semibold text-slate-800">存储快照</h3>
        </div>
        <p class="mb-4 text-xs text-slate-500">创建当前存储一致性快照</p>
        <button
          :disabled="loading === 'snapshot'"
          class="w-full rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50"
          @click="doSnapshot"
        >
          {{ loading === 'snapshot' ? '快照中...' : '创建快照' }}
        </button>
      </div>

      <!-- 导出 -->
      <div class="rounded-xl border border-slate-200 bg-white p-6">
        <div class="mb-3 flex items-center gap-2">
          <Download class="h-5 w-5 text-slate-500" />
          <h3 class="text-sm font-semibold text-slate-800">配置导出</h3>
        </div>
        <p class="mb-4 text-xs text-slate-500">导出当前配置和健康快照</p>
        <button
          :disabled="loading === 'export'"
          class="w-full rounded-lg bg-slate-800 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50"
          @click="doExport"
        >
          {{ loading === 'export' ? '导出中...' : '导出配置' }}
        </button>
      </div>
    </div>

    <!-- 验证结果 -->
    <div v-if="validateResult" class="rounded-xl border border-slate-200 bg-white p-6">
      <h3 class="mb-3 text-sm font-semibold text-slate-800">验证结果</h3>
      <div class="mb-2 flex items-center gap-2">
        <component :is="validateResult.ok ? CheckCircle : AlertCircle" :class="validateResult.ok ? 'text-green-600' : 'text-red-600'" class="h-5 w-5" />
        <span :class="validateResult.ok ? 'text-green-700' : 'text-red-700'" class="text-sm font-medium">
          {{ validateResult.ok ? '验证通过' : '验证失败' }}
        </span>
      </div>
      <p class="text-xs text-slate-500">数据目录: {{ validateResult.data_dir }}</p>
      <pre class="mt-2 overflow-auto rounded-lg bg-slate-900 p-3 text-xs text-green-400 max-h-48">{{ JSON.stringify(validateResult.health, null, 2) }}</pre>
    </div>

    <!-- 快照结果 -->
    <div v-if="snapshotResult" class="rounded-xl border border-slate-200 bg-white p-6">
      <h3 class="mb-2 text-sm font-semibold text-slate-800">快照结果</h3>
      <p :class="snapshotResult.ok ? 'text-green-700' : 'text-red-700'" class="text-sm">
        {{ snapshotResult.ok ? '快照创建成功' : '快照创建失败' }}
      </p>
      <p v-if="snapshotResult.path" class="mt-1 text-xs text-slate-500">路径: {{ snapshotResult.path }}</p>
    </div>

    <!-- 导出结果 -->
    <div v-if="exportData" class="rounded-xl border border-slate-200 bg-white p-6">
      <h3 class="mb-2 text-sm font-semibold text-slate-800">配置导出</h3>
      <p class="mb-2 text-xs text-slate-500">生成时间: {{ exportData.generated_at }}</p>
      <pre class="overflow-auto rounded-lg bg-slate-900 p-4 text-xs text-green-400 max-h-96">{{ JSON.stringify(exportData, null, 2) }}</pre>
    </div>
  </div>
</template>
```

- [ ] **Step 2: 验证构建**

Run: `cd cmd/mts-dashboard && npm run build`

Expected: 构建成功。

---

### Task 13: Go Embed 集成

**Files:**
- Create: `cmd/mts-server/dashboard.go`
- Modify: `cmd/mts-server/http.go:13-52`

- [ ] **Step 1: 创建 dashboard.go**

File: `cmd/mts-server/dashboard.go`

```go
package main

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed dashboard-dist
var dashboardFS embed.FS

func (r *serverRuntime) dashboardHandler() http.Handler {
	sub, err := fs.Sub(dashboardFS, "dashboard-dist")
	if err != nil {
		// 开发模式未构建前端时返回空 handler
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("dashboard not built"))
		})
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// 尝试直接服务文件
		path := strings.TrimPrefix(req.URL.Path, "/")
		f, err := sub.Open(path)
		if err != nil {
			// 文件不存在 -> SPA fallback: 返回 index.html
			req.URL.Path = "/"
		} else {
			_ = f.Close()
		}
		// 静态资源设置缓存头
		if strings.HasPrefix(req.URL.Path, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		fileServer.ServeHTTP(w, req)
	})
}
```

- [ ] **Step 2: 修改 http.go 注册 SPA handler**

在 `httpHandler()` 方法中，所有 API 路由注册完后、`return` 之前，添加 dashboard handler 注册。

File: `cmd/mts-server/http.go`，在 `r.mountPprof(mux)` 之后、`return r.wrapHTTP(mux)` 之前，添加：

```go
	mux.HandleFunc("/", r.dashboardHandler().ServeHTTP)
```

修改后的 `httpHandler()` 方法末尾为:

```go
func (r *serverRuntime) httpHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", r.handleHealth)
	mux.HandleFunc("/readyz", r.handleHealth)
	mux.HandleFunc("/metrics", r.handleMetrics)
	mux.HandleFunc("/api/v1/data/write", r.handleWrite)
	mux.HandleFunc("/api/v1/data/write/typed", r.handleWriteTyped)
	mux.HandleFunc("/api/v1/data/query/rows", r.handleQueryRows)
	mux.HandleFunc("/api/v1/data/query/columns", r.handleQueryColumns)
	mux.HandleFunc("/api/v1/data/query/explain", r.handleQueryExplain)
	mux.HandleFunc("/api/v1/data/query/stream", r.handleQueryStream)
	mux.HandleFunc("/api/v1/data/query/stats", r.handleQueryStats)
	mux.HandleFunc("/api/v1/data/databases/", r.handleDataDatabase)
	mux.HandleFunc("/api/v1/users", r.handleUsers)
	mux.HandleFunc("/api/v1/users/", r.handleUserResource)
	mux.HandleFunc("/api/v1/authz/database/check", r.handleAuthzDatabaseCheck)
	mux.HandleFunc("/api/v1/admin/databases", r.handleAdminDatabases)
	mux.HandleFunc("/api/v1/admin/databases/", r.handleAdminDatabaseResource)
	mux.HandleFunc("/api/v1/admin/config", r.handleConfig)
	mux.HandleFunc("/api/v1/admin/config/effective", r.handleConfig)
	mux.HandleFunc("/api/v1/admin/config/schema", r.handleConfigSchema)
	mux.HandleFunc("/api/v1/admin/flush", r.handleFlush)
	mux.HandleFunc("/api/v1/admin/compact", r.handleCompact)
	mux.HandleFunc("/api/v1/admin/retention/apply", r.handleApplyRetention)
	mux.HandleFunc("/api/v1/admin/maintenance/errors", r.handleMaintenanceErrors)
	mux.HandleFunc("/api/v1/admin/stats/storage-memory", r.handleStorageMemory)
	mux.HandleFunc("/api/v1/admin/stats/compaction", r.handleCompactionStats)
	mux.HandleFunc("/api/v1/admin/health", r.handleAdminHealth)
	mux.HandleFunc("/api/v1/admin/downsample/policies", r.handleDownsamplePolicies)
	mux.HandleFunc("/api/v1/admin/downsample/policies/", r.handleDownsamplePolicyResource)
	mux.HandleFunc("/api/v1/admin/downsample/statuses", r.handleDownsampleStatuses)
	mux.HandleFunc("/api/v1/admin/api-spec", r.handleAPISpec)
	mux.HandleFunc("/api/v1/admin/error-codes", r.handleErrorCodes)
	mux.HandleFunc("/api/v1/admin/config/validate", r.handleValidateConfig)
	mux.HandleFunc("/api/v1/admin/config/reload", r.handleReloadConfig)
	mux.HandleFunc("/api/v1/admin/storage/validate", r.handleStorageValidate)
	mux.HandleFunc("/api/v1/admin/storage/snapshot", r.handleStorageSnapshot)
	mux.HandleFunc("/api/v1/admin/storage/export", r.handleStorageExport)
	r.mountPprof(mux)
	mux.HandleFunc("/", r.dashboardHandler().ServeHTTP)
	return r.wrapHTTP(mux)
}
```

- [ ] **Step 3: 添加 .gitignore 规则**

在项目根目录的 `.gitignore` 中添加前端构建产物忽略规则。

```gitignore
# mts-dashboard
cmd/mts-server/dashboard-dist/
cmd/mts-dashboard/node_modules/
```

- [ ] **Step 4: 构建验证**

先构建前端，再构建后端:

```bash
cd cmd/mts-dashboard && npm ci && npm run build && cd ../.. && go build ./cmd/mts-server
```

Expected: 编译通过，生成 `mts-server` 二进制。

- [ ] **Step 5: 运行验证**

启动 mts-server:

```bash
./mts-server serve --config configs/mts-server.yaml
```

访问 `http://127.0.0.1:8086/`，预期看到 Dashboard 登录页面。

---

### Task 14: Makefile 集成

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: 添加前端构建目标到 Makefile**

在 Makefile 的 `.PHONY: fmt` 之后、`.PHONY: lint` 之前插入:

```makefile
.PHONY: dashboard
dashboard: ## 构建前端 Dashboard 并嵌入
	cd cmd/mts-dashboard && npm ci && npm run build
```

同时在 `clean-artifacts` 目标中添加 dashboard 构建产物的清理。修改 clean-artifacts 目标:

```makefile
.PHONY: clean-artifacts
clean-artifacts: ## 清理测试和 profile 临时产物
	find . -type f \( \
		-name '*.test' -o \
		-name '*.prof' -o \
		-name '*.pprof' -o \
		-name 'coverage.out' -o \
		-name '*.coverprofile' \
	\) -not -path './.git/*' -print -delete
	rm -rf cmd/mts-server/dashboard-dist
```

- [ ] **Step 2: 验证 make dashboard**

Run: `make dashboard`

Expected: 前端构建成功，`cmd/mts-server/dashboard-dist/` 目录生成。

- [ ] **Step 3: 完整构建验证**

```bash
make dashboard && go build -o mts-server ./cmd/mts-server
```

Expected: 编译通过。

---

### Task 15: 端到端验证

- [ ] **Step 1: 启动 mts-server**

```bash
./mts-server serve --config configs/mts-server.yaml
```

- [ ] **Step 2: 浏览器访问验证**

访问 `http://127.0.0.1:8086/`:
- 预期跳转到 `/login`
- 登录页显示预填的用户名和密码
- 点击登录后进入仪表盘概览页面
- 左侧导航栏有 9 个菜单项
- 可以正常浏览各管理页面

- [ ] **Step 3: API 路由不受影响验证**

```bash
curl http://127.0.0.1:8086/healthz
curl http://127.0.0.1:8086/api/v1/admin/config/effective
```

Expected: 现有的 API 端点正常响应，不受 Dashboard 影响。

- [ ] **Step 4: SPA fallback 验证**

访问 `http://127.0.0.1:8086/databases` (前端路由)，预期返回 `index.html`（由 Vue Router 接管），而不是 404。

- [ ] **Step 5: 清理**

```bash
rm -f mts-server
```

---

### Task 16: Golangci-lint 与格式化

- [ ] **Step 1: 运行 golangci-lint**

```bash
cd cmd/mts-server && golangci-lint run ./...
```

Expected: 无新增 lint 错误。

- [ ] **Step 2: 运行 goimports-reviser**

```bash
goimports-reviser -project-name github.com/openmts/mts -recursive -format -rm-unused ./cmd/mts-server
```

Expected: 代码格式化完成。
