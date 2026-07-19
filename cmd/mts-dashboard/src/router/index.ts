import { createRouter, createWebHistory } from 'vue-router'
import { useAuth } from '@/composables/useAuth'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/login',
      name: 'Login',
      component: () => import('@/pages/LoginPage.vue'),
      meta: { public: true },
    },
    {
      path: '/',
      component: () => import('@/layouts/DashboardLayout.vue'),
      children: [
        { path: '', name: 'Overview', component: () => import('@/pages/OverviewPage.vue') },
        { path: 'databases', name: 'Databases', component: () => import('@/pages/DatabasesPage.vue'), meta: { admin: true } },
        { path: 'users', name: 'Users', component: () => import('@/pages/UsersPage.vue') },
        { path: 'config', name: 'Config', component: () => import('@/pages/ConfigPage.vue'), meta: { admin: true } },
        { path: 'operations', name: 'Operations', component: () => import('@/pages/OperationsPage.vue'), meta: { admin: true } },
        { path: 'downsample', name: 'Downsample', component: () => import('@/pages/DownsamplePage.vue'), meta: { admin: true } },
        { path: 'query', name: 'Query', component: () => import('@/pages/QueryPage.vue') },
        { path: 'audit', name: 'Audit', component: () => import('@/pages/AuditPage.vue'), meta: { admin: true } },
        { path: 'storage', name: 'Storage', component: () => import('@/pages/StoragePage.vue'), meta: { admin: true } },
        { path: 'write', name: 'Write', component: () => import('@/pages/WritePage.vue') },
        { path: ':pathMatch(.*)*', name: 'NotFound', component: () => import('@/pages/NotFoundPage.vue') },
      ],
    },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

export { sanitizeRedirect } from '@/utils/redirect'

router.beforeEach((to) => {
  const { ensureSession, isAuthenticated, isAdmin } = useAuth()
  const ok = ensureSession()

  if (to.name === 'Login') {
    if (ok && isAuthenticated.value) return { name: 'Overview' }
    return true
  }

  if (!ok || !isAuthenticated.value) {
    return { name: 'Login', query: { redirect: to.fullPath } }
  }

  if (to.meta.admin && !isAdmin.value) {
    // 允许进入路由，由页面显示权限空态；也可硬跳 Overview。这里保留进入以显示 PermissionDenied。
    return true
  }

  return true
})

export default router
