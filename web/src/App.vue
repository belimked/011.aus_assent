<template>
  <el-config-provider :locale="epLocale">
  <router-view v-if="route.meta.bare" />
  <div v-else class="shell" :class="{ collapsed: ui.collapsed }">
    <!-- 侧边栏 -->
    <aside class="side">
      <div class="brand" @click="router.push('/dashboard')">
        <div class="logo"><svg viewBox="0 0 32 32"><path d="M6 16l10-9 10 9v10h-7v-7h-6v7H6z" fill="#c9a227" /></svg></div>
        <div v-if="!ui.collapsed" class="bt">
          <div class="serif b1">Place Sunnybank</div>
          <div class="b2">{{ t('业主信息采集平台') }}</div>
        </div>
      </div>
      <nav>
        <template v-for="g in navGroups" :key="g.t">
          <div v-if="!ui.collapsed && g.items.length" class="ng">{{ t(g.t) }}</div>
          <router-link v-for="r in g.items" :key="r.path" :to="r.path" class="ni" :title="t(r.meta.title)">
            <el-icon :size="18"><component :is="r.meta.icon" /></el-icon>
            <span v-if="!ui.collapsed">{{ t(r.meta.title) }}</span>
            <el-badge v-if="badge(r.path) && !ui.collapsed" :value="badge(r.path)" class="nb" :type="r.path === '/tasks' ? 'warning' : 'danger'" />
          </router-link>
        </template>
      </nav>
      <div class="side-foot" v-if="!ui.collapsed">
        <div class="sync"><span class="dot" :style="{ background: state.live ? '#22c55e' : '#f59e0b' }" :class="{ pulse: state.live }" /> {{ t('Rex 同步') }} {{ state.live ? t('实时') : t('已暂停') }}</div>
        <div class="small" style="opacity:.6">{{ t('最后同步 {time} · 配额 {n}/3600', { time: fmtTime(state.lastSync), n: state.quota }) }}</div>
      </div>
      <button class="collapse" @click="ui.collapsed = !ui.collapsed"><el-icon><component :is="ui.collapsed ? 'DArrowRight' : 'DArrowLeft'" /></el-icon></button>
    </aside>

    <!-- 主区 -->
    <div class="main">
      <header class="top">
        <div class="crumb">
          <el-icon :size="18" color="#8792a2"><component :is="route.meta.icon" /></el-icon>
          <b>{{ t(route.meta.title) }}</b>
        </div>
        <div class="search" @click="ui.search = true">
          <el-icon><Search /></el-icon><span>{{ t('搜索地址、业主、电话…') }}</span><span class="kbd">Ctrl K</span>
        </div>
        <div class="sp" />
        <el-tooltip :content="t('打开全屏数据驾驶舱（适合会议室大屏）')">
          <el-button class="gold-btn" :icon="DataBoard" @click="router.push('/cockpit')">{{ t('驾驶舱') }}</el-button>
        </el-tooltip>
        <el-dropdown trigger="click" @command="setLocale">
          <el-button class="lang" :icon="Switch">{{ LOCALES.find(l => l.k === locale).l }}</el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item v-for="l in LOCALES" :key="l.k" :command="l.k"><div class="row" style="min-width:120px"><span class="lshort">{{ l.short }}</span>{{ l.l }}<div class="grow" /><el-icon v-if="l.k === locale" color="#16a34a"><Check /></el-icon></div></el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
        <el-popover placement="bottom-end" :width="360" trigger="click">
          <template #reference>
            <el-badge :value="unread" :hidden="!unread" class="bell"><el-button circle :icon="Bell" /></el-badge>
          </template>
          <div class="row" style="margin-bottom:8px"><b>{{ t('通知') }}</b><div class="sp grow" /><el-button link size="small" @click="state.notifications.forEach(n => n.read = true)">{{ t('全部已读') }}</el-button></div>
          <el-scrollbar max-height="380px">
            <div v-for="n in state.notifications" :key="n.id" class="noti" :class="{ unread: !n.read }" @click="n.read = true; n.link && router.push(n.link)">
              <span class="dot" :style="{ background: { warning: '#f59e0b', success: '#16a34a', info: '#2563eb' }[n.type] }" />
              <div class="grow"><div class="nt">{{ t(n.title) }}</div><div class="small muted">{{ t(n.body, n.p) }}</div><div class="small muted" style="margin-top:2px">{{ relTime(n.at) }}</div></div>
            </div>
            <el-empty v-if="!state.notifications.length" :description="t('暂无通知')" :image-size="60" />
          </el-scrollbar>
        </el-popover>
        <el-dropdown trigger="click" @command="switchUser">
          <div class="me">
            <Avatar :id="me.id" :size="32" />
            <div class="mt"><div class="mn">{{ me.name }}</div><div class="small muted">{{ t(me.roleLabel) }} · {{ t(me.team) }}</div></div>
            <el-icon><ArrowDown /></el-icon>
          </div>
          <template #dropdown>
            <el-dropdown-menu>
              <div class="small muted" style="padding:6px 16px">{{ t('演示：切换登录身份（模拟 M365 SSO）') }}</div>
              <el-dropdown-item v-for="u in USERS" :key="u.id" :command="u.id" :disabled="u.status !== 'active'">
                <div class="row"><Avatar :id="u.id" :size="22" /><span>{{ u.name }}</span><span class="small muted">{{ t(u.roleLabel) }}</span><el-icon v-if="u.id === state.me" color="#16a34a"><Check /></el-icon></div>
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </header>
      <main class="content no-copy">
        <router-view v-slot="{ Component }"><keep-alive include="MapView"><component :is="Component" /></keep-alive></router-view>
      </main>
    </div>
    <div v-if="state.settings.watermark" class="watermark" :style="{ backgroundImage: wm }" />
  </div>

  <PropertyDrawer />
  <CaptureDialog />
  <SearchDialog />
  </el-config-provider>
</template>

<script setup>
import { computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Bell, DataBoard, Switch } from '@element-plus/icons-vue'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import zhHk from 'element-plus/es/locale/lang/zh-hk'
import en from 'element-plus/es/locale/lang/en'
import { t, locale, LOCALES, setLocale } from './i18n'
import { routes } from './router'
import { state, me, USERS, fmtTime, relTime, now } from './data/store'
import { ui } from './data/ui'
import Avatar from './components/Avatar.vue'
import PropertyDrawer from './components/PropertyDrawer.vue'
import CaptureDialog from './components/CaptureDialog.vue'
import SearchDialog from './components/SearchDialog.vue'

const route = useRoute(), router = useRouter()
const epLocale = computed(() => ({ 'zh-CN': zhCn, 'zh-HK': zhHk, en })[locale.value])
watch(() => [locale.value, route.meta.title], () => { document.title = `${t(route.meta.title || '工作台')} · Place Sunnybank` }, { immediate: true })
const allowed = r => !r.meta.roles || r.meta.roles.includes(me.value.role)
const visible = computed(() => routes.filter(r => r.meta && !r.meta.bare && allowed(r)))
const navGroups = computed(() => [
  { t: '工作', items: visible.value.filter(r => ['/dashboard', '/map', '/properties', '/ofi', '/tasks'].includes(r.path)) },
  { t: '管理', items: visible.value.filter(r => ['/review', '/team', '/audit', '/export', '/settings'].includes(r.path)) },
])
const myTasks = computed(() => state.tasks.filter(t => !t.done && t.due < new Date(now().getTime() + 864e5) && (me.value.role === 'admin' || t.agent === me.value.id || me.value.role === 'leader')).length)
const badge = p => p === '/review' ? state.reviews.length : p === '/tasks' ? myTasks.value : 0
const unread = computed(() => state.notifications.filter(n => !n.read).length)
const wm = computed(() => `url("data:image/svg+xml;utf8,${encodeURIComponent(`<svg xmlns='http://www.w3.org/2000/svg' width='340' height='200'><text x='20' y='110' transform='rotate(-18 170 100)' font-size='15' font-family='sans-serif' fill='#000'>${me.value.name} · ${new Date().toLocaleDateString('en-AU')} · ${t('机密')}</text></svg>`)}")`)

function switchUser(id) {
  state.me = id
  ElMessage.success(t('已切换为 {name}（{role}）', { name: me.value.name, role: t(me.value.roleLabel) }))
  if (!allowed(route)) router.push('/dashboard')
}
function onKey(e) {
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') { e.preventDefault(); ui.search = true }
}
onMounted(() => window.addEventListener('keydown', onKey))
onBeforeUnmount(() => window.removeEventListener('keydown', onKey))
</script>

<style scoped>
.shell { display: flex; height: 100vh; }
.side { width: 232px; flex: none; background: linear-gradient(180deg, var(--navy) 0%, #0b2135 100%); color: #d6deea; display: flex; flex-direction: column; position: relative; transition: width .2s; }
.collapsed .side { width: 68px; }
.brand { display: flex; align-items: center; gap: 10px; padding: 18px 16px 16px; cursor: pointer; border-bottom: 1px solid rgba(255, 255, 255, .07); }
.logo { width: 36px; height: 36px; border-radius: 9px; background: rgba(201, 162, 39, .14); border: 1px solid rgba(201, 162, 39, .45); display: grid; place-items: center; flex: none; }
.logo svg { width: 22px; }
.b1 { color: #fff; font-size: 16px; font-weight: 700; letter-spacing: .3px; }
.b2 { font-size: 11px; color: var(--gold-2); letter-spacing: 1px; margin-top: 1px; }
nav { flex: 1; overflow: auto; padding: 8px 10px; }
.ng { font-size: 11px; letter-spacing: 1.5px; color: #6f87a3; padding: 14px 10px 6px; }
.ni { display: flex; align-items: center; gap: 11px; padding: 9px 11px; border-radius: 9px; text-decoration: none; color: #c4cfdd; font-size: 14px; margin-bottom: 2px; position: relative; }
.collapsed .ni { justify-content: center; }
.ni:hover { background: rgba(255, 255, 255, .06); color: #fff; }
.ni.router-link-active { background: linear-gradient(90deg, rgba(201, 162, 39, .22), rgba(201, 162, 39, .05)); color: #fff; box-shadow: inset 3px 0 0 var(--gold); }
.ni.router-link-active .el-icon { color: var(--gold-2); }
.nb { margin-left: auto; }
.nb :deep(.el-badge__content) { border: 0; }
.side-foot { padding: 12px 18px 46px; font-size: 12px; border-top: 1px solid rgba(255, 255, 255, .07); }
.sync { display: flex; align-items: center; gap: 7px; margin-bottom: 3px; color: #e2e8f0; }
.collapse { position: absolute; bottom: 10px; right: 12px; background: rgba(255, 255, 255, .06); border: 0; color: #9fb0c4; border-radius: 7px; width: 28px; height: 26px; cursor: pointer; }
.main { flex: 1; min-width: 0; display: flex; flex-direction: column; }
.top { height: 60px; flex: none; display: flex; align-items: center; gap: 14px; padding: 0 22px; background: #fff; border-bottom: 1px solid var(--line); }
.crumb { display: flex; align-items: center; gap: 8px; font-size: 15px; min-width: 120px; }
.search { display: flex; align-items: center; gap: 8px; width: 340px; padding: 7px 12px; border: 1px solid var(--line); background: #f8f9fb; border-radius: 10px; color: #98a2b3; font-size: 13px; cursor: text; }
.search span:nth-child(2) { flex: 1; }
.sp { flex: 1; }
.bell :deep(.el-badge__content) { top: 6px; right: 8px; }
.me { display: flex; align-items: center; gap: 9px; cursor: pointer; padding: 4px 6px; border-radius: 10px; }
.me:hover { background: #f4f5f7; }
.mn { font-weight: 650; font-size: 14px; line-height: 1.2; }
.content { flex: 1; overflow: auto; }
.noti { display: flex; gap: 10px; padding: 10px 6px; border-radius: 8px; cursor: pointer; align-items: flex-start; }
.noti .dot { margin-top: 6px; }
.noti:hover { background: #f6f7f9; }
.noti.unread .nt { font-weight: 700; }
.nt { font-size: 13px; }
.lang { font-weight: 600; }
.lshort { display: inline-grid; place-items: center; width: 26px; height: 20px; border-radius: 5px; background: #eef2f6; font-size: 11px; font-weight: 700; color: var(--navy); }
</style>
