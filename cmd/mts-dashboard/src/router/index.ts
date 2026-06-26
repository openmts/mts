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
        { path: '', name: 'Overview', component: () => import('@/pages/OverviewPage.vue') },
        { path: 'databases', name: 'Databases', component: () => import('@/pages/DatabasesPage.vue') },
        { path: 'users', name: 'Users', component: () => import('@/pages/UsersPage.vue') },
        { path: 'config', name: 'Config', component: () => import('@/pages/ConfigPage.vue') },
        { path: 'operations', name: 'Operations', component: () => import('@/pages/OperationsPage.vue') },
        { path: 'downsample', name: 'Downsample', component: () => import('@/pages/DownsamplePage.vue') },
        { path: 'query', name: 'Query', component: () => import('@/pages/QueryPage.vue') },
        { path: 'audit', name: 'Audit', component: () => import('@/pages/AuditPage.vue') },
        { path: 'storage', name: 'Storage', component: () => import('@/pages/StoragePage.vue') },
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
