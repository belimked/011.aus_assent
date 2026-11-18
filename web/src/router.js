import { createRouter, createWebHashHistory } from 'vue-router'
import { token, identity, restore } from './api'
export const routes = [
 { path: '/dashboard', component: () => import('./views/AdminDashboard.vue'), meta: { title: '工作台', icon: 'Odometer' } },
 { path: '/properties', component: () => import('./views/AdminProperties.vue'), meta: { title: '房产列表', icon: 'OfficeBuilding' } },
 { path: '/map', component: () => import('./views/AdminMap.vue'), meta: { title: '区域地图', icon: 'MapLocation' } },
 { path: '/team', component: () => import('./views/AdminTables.vue'), props: { page: 'team' }, meta: { title: '团队与权限', icon: 'UserFilled' } },
 { path: '/review', component: () => import('./views/AdminTables.vue'), props: { page: 'review' }, meta: { title: '审核队列', icon: 'Stamp' } },
 { path: '/audit', component: () => import('./views/AdminTables.vue'), props: { page: 'audit' }, meta: { title: '审计日志', icon: 'Tickets' } },
 { path: '/rex', component: () => import('./views/AdminTables.vue'), props: { page: 'rex' }, meta: { title: 'Rex 队列', icon: 'Connection' } },
 { path: '/market', component: () => import('./views/AdminTables.vue'), props: { page: 'market' }, meta: { title: '市场数据', icon: 'Upload' } },
 { path: '/data', component: () => import('./views/AdminTables.vue'), props: { page: 'data' }, meta: { title: '数据浏览', icon: 'Grid' } },
]
const router = createRouter({ history: createWebHashHistory('/admin/'), routes: [{ path: '/', redirect: '/dashboard' }, { path: '/login', component: () => import('./views/AdminLogin.vue'), meta: { bare: true, title: '登录' } }, ...routes, {path: '/:pathMatch(.*)*', redirect: '/dashboard'}] })
router.beforeEach(async to => { if (to.path === '/login') return true; if (!token.value || (!identity.value && !await restore())) return '/login'; return true })
export default router
