import { createRouter, createWebHistory } from 'vue-router'
import { useAuth } from '@/composables/useAuth'
import { setRouteLoading } from '@/composables/useGlobalLoading'
import { allowNavigationWhenDirty, anyRouteDirty } from '@/utils/routeDirty'
import { messages, type Locale } from '@/i18n/messages'

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
      path: '/force-change-password',
      name: 'ForceChangePassword',
      component: () => import('@/pages/ForceChangePasswordPage.vue'),
      meta: { public: true, forceChange: true },
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
        { path: 'api-spec', name: 'ApiSpec', component: () => import('@/pages/ApiSpecPage.vue'), meta: { admin: true } },
        { path: 'storage', name: 'Storage', component: () => import('@/pages/StoragePage.vue'), meta: { admin: true } },
        { path: 'ops/readiness', name: 'Readiness', component: () => import('@/pages/ReadinessPage.vue'), meta: { admin: true } },
        { path: 'about', name: 'About', component: () => import('@/pages/AboutPage.vue') },
        { path: 'account', name: 'Account', component: () => import('@/pages/AccountPage.vue') },
        { path: 'write', name: 'Write', component: () => import('@/pages/WritePage.vue') },
        { path: 'access', name: 'AccessMatrix', component: () => import('@/pages/AccessMatrixPage.vue') },
        { path: 'access/grants', name: 'AccessGrants', component: () => import('@/pages/AccessGrantsPage.vue'), meta: { admin: true } },
        { path: 'observability/metrics', name: 'Metrics', component: () => import('@/pages/MetricsPage.vue'), meta: { admin: true } },
        { path: ':pathMatch(.*)*', name: 'NotFound', component: () => import('@/pages/NotFoundPage.vue') },
      ],
    },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

export { sanitizeRedirect } from '@/utils/redirect'

router.beforeEach((to, from) => {
  setRouteLoading(true)
  // 脏表单离开确认（查询/写入等页注册 checker）
  if (from.matched.length && to.fullPath !== from.fullPath && anyRouteDirty()) {
    let loc: Locale = 'zh'
    try {
      const v = localStorage.getItem('mts_locale')
      if (v === 'zh' || v === 'en') loc = v
    } catch { /* ignore */ }
    const leave = allowNavigationWhenDirty(
      true,
      messages[loc].unsavedLeaveConfirm,
    )
    if (!leave) {
      setRouteLoading(false)
      return false
    }
  }
  const { ensureSession, isAuthenticated, isAdmin, mustChangePassword } = useAuth()
  const ok = ensureSession()

  if (to.name === 'Login') {
    if (ok && isAuthenticated.value) {
      if (mustChangePassword.value) return { name: 'ForceChangePassword' }
      return { name: 'Overview' }
    }
    return true
  }

  if (to.name === 'ForceChangePassword') {
    if (!ok || !isAuthenticated.value) {
      return { name: 'Login', query: { reason: 'auth' } }
    }
    if (!mustChangePassword.value) return { name: 'Overview' }
    return true
  }

  if (!ok || !isAuthenticated.value) {
    return { name: 'Login', query: { redirect: to.fullPath, reason: 'auth' } }
  }

  if (mustChangePassword.value) {
    return { name: 'ForceChangePassword' }
  }

  if (to.meta.admin && !isAdmin.value) {
    // 允许进入路由，由页面显示权限空态
    return true
  }

  return true
})

router.afterEach(() => {
  setRouteLoading(false)
})

router.onError(() => {
  setRouteLoading(false)
})

export default router
