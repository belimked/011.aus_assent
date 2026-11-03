import { createRouter, createWebHashHistory } from 'vue-router'

const routes = [
  { path: '/', redirect: '/dashboard' },
  { path: '/dashboard', component: () => import('./views/Dashboard.vue'), meta: { title: '工作台', icon: 'Odometer' } },
  { path: '/cockpit', component: () => import('./views/Cockpit.vue'), meta: { title: '数据驾驶舱', icon: 'DataBoard', bare: true } },
  { path: '/map', component: () => import('./views/MapView.vue'), meta: { title: '区域地图', icon: 'MapLocation' } },
  { path: '/properties', component: () => import('./views/Properties.vue'), meta: { title: '房产列表', icon: 'OfficeBuilding' } },
  { path: '/ofi', component: () => import('./views/Ofi.vue'), meta: { title: '开放参观', icon: 'Key' } },
  { path: '/tasks', component: () => import('./views/Tasks.vue'), meta: { title: '跟进任务', icon: 'Calendar' } },
  { path: '/review', component: () => import('./views/Review.vue'), meta: { title: '审核队列', icon: 'Stamp', roles: ['admin', 'leader'] } },
  { path: '/team', component: () => import('./views/Team.vue'), meta: { title: '团队与权限', icon: 'UserFilled', roles: ['admin', 'leader'] } },
  { path: '/audit', component: () => import('./views/Audit.vue'), meta: { title: '审计日志', icon: 'Tickets', roles: ['admin'] } },
  { path: '/export', component: () => import('./views/Export.vue'), meta: { title: '数据导出', icon: 'Download', roles: ['admin'] } },
  { path: '/settings', component: () => import('./views/Settings.vue'), meta: { title: '系统设置', icon: 'Setting', roles: ['admin'] } },
]
export { routes }
const router = createRouter({ history: createWebHashHistory(), routes })
// 按角色拦截：无权限的页面直接回工作台（防止手输 URL 绕过侧边栏）
router.beforeEach(async to => {
  if (!to.meta?.roles) return true
  const { me } = await import('./data/store')
  if (to.meta.roles.includes(me.value.role)) return true
  const { ElMessage } = await import('element-plus')
  const { t } = await import('./i18n')
  ElMessage.warning(t('当前身份（{role}）无权访问「{page}」', { role: t(me.value.roleLabel), page: t(to.meta.title) }))
  return '/dashboard'
})
export default router
