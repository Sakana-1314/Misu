import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'login',
    component: () => import('@/views/LoginView.vue'),
    meta: { title: '登录', public: true },
  },
  {
    path: '/auth/callback',
    name: 'auth-callback',
    component: () => import('@/views/AuthCallbackView.vue'),
    meta: { title: '授权结果', public: true },
  },
  {
    path: '/',
    component: () => import('@/layouts/MainLayout.vue'),
    children: [
      {
        path: '',
        name: 'dashboard',
        component: () => import('@/views/DashboardView.vue'),
        meta: { title: '仪表盘' },
      },
      {
        path: 'authorizers',
        name: 'authorizers',
        component: () => import('@/views/AuthorizersView.vue'),
        meta: { title: '小程序' },
      },
      {
        path: 'authorizers/:appid',
        name: 'authorizer-detail',
        component: () => import('@/views/AuthorizerDetailView.vue'),
        meta: { title: '小程序详情' },
      },
      {
        path: 'templates',
        name: 'templates',
        component: () => import('@/views/TemplatesView.vue'),
        meta: { title: '代码模板' },
      },
      {
        path: 'audit-profiles',
        name: 'audit-profiles',
        component: () => import('@/views/AuditProfilesView.vue'),
        meta: { title: '审核配置' },
      },
      {
        path: 'logs',
        name: 'logs',
        component: () => import('@/views/LogsView.vue'),
        meta: { title: '操作日志' },
      },
    ],
  },
  {
    path: '/:pathMatch(.*)*',
    name: 'not-found',
    component: () => import('@/views/NotFoundView.vue'),
    meta: { title: '页面不存在', public: true },
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior: () => ({ top: 0 }),
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  if (to.meta.public) {
    if (to.name === 'login' && auth.isAuthenticated.value) {
      return { name: 'dashboard' }
    }
    return true
  }
  if (!auth.state.loaded) {
    await auth.fetchMe()
  }
  if (!auth.isAuthenticated.value) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  return true
})

router.afterEach((to) => {
  const title = (to.meta.title as string | undefined) || '御坂微枢'
  document.title = title === '御坂微枢' ? title : `${title} · 御坂微枢`
})

export default router
