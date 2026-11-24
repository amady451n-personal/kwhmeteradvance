import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      name: 'beranda',
      component: () => import('../views/DashboardView.vue'),
      meta: { title: 'Beranda' },
    },
    {
      path: '/riwayat',
      name: 'riwayat',
      component: () => import('../views/ReadingsView.vue'),
      meta: { title: 'Riwayat' },
    },
    {
      path: '/token',
      name: 'token',
      component: () => import('../views/TokensView.vue'),
      meta: { title: 'Token' },
    },
    {
      path: '/laporan',
      name: 'laporan',
      component: () => import('../views/ReportsView.vue'),
      meta: { title: 'Laporan' },
    },
    {
      path: '/pengaturan',
      name: 'pengaturan',
      component: () => import('../views/SettingsView.vue'),
      meta: { title: 'Pengaturan' },
    },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
  scrollBehavior: () => ({ top: 0 }),
})

router.afterEach((to) => {
  const title = to.meta.title as string | undefined
  document.title = title ? `${title} · meteran` : 'meteran'
})

export default router
