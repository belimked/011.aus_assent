<template>
  <div class="page">
    <div class="page-head">
      <div><h1>{{ t('团队与权限') }}</h1><div class="sub">{{ t('登录方式：Microsoft 365（Entra ID）· 撤销访问 1 分钟内生效') }}</div></div>
      <div class="sp" />
      <el-segmented v-model="period" :options="PERIODS.map(p => ({ label: t(p.l), value: p.k }))" />
    </div>
    <el-alert v-for="n in alerts" :key="n.id" :title="t(n.title)" :description="t(n.body, n.p)" type="warning" show-icon style="margin-bottom:12px" @close="n.read = true">
      <template #default><el-button size="small" style="margin-top:6px" @click="pick(n.u)">{{ t('查看该经纪人') }}</el-button></template>
    </el-alert>
    <div class="tm">
      <div class="ul">
        <div v-for="u in list" :key="u.id" class="uc card" :class="{ on: sel?.id === u.id, off: u.status !== 'active' }" @click="pick(u.id)">
          <Avatar :id="u.id" :size="44" />
          <div class="grow"><div class="row"><b>{{ u.name }}</b><el-tag size="small" effect="plain">{{ t(u.roleLabel) }}</el-tag></div><div class="small muted">{{ t(u.team) }} · {{ t(u.device) }}</div>
            <div class="small" :style="{ color: u.status === 'active' ? '#16a34a' : '#dc2626' }">● {{ u.status === 'active' ? t(u.last) : t('已撤销访问') }}</div></div>
          <div class="mini" v-if="stat(u.id)"><b class="serif">{{ stat(u.id).knocks }}</b><span>{{ t('拜访') }}</span></div>
        </div>
      </div>
      <div v-if="sel" class="card ud">
        <div class="uh">
          <Avatar :id="sel.id" :size="64" />
          <div class="grow"><div class="serif nm">{{ sel.name }}</div><div class="muted">{{ t(sel.roleLabel) }} · {{ t(sel.team) }} · {{ sel.name.toLowerCase().replace(' ', '.') }}@placesunnybank.com.au</div></div>
          <el-button v-if="sel.status === 'active' && sel.id !== 'ping' && me.role === 'admin'" type="danger" plain :icon="Lock" @click="revoke">{{ t('撤销访问') }}</el-button>
          <el-button v-else-if="sel.status !== 'active'" type="success" plain @click="restore">{{ t('恢复访问') }}</el-button>
        </div>
        <div class="stats" v-if="stat(sel.id)">
          <div v-for="k in kk" :key="k.k"><div class="small muted">{{ t(k.l) }}</div><b class="serif">{{ stat(sel.id)[k.k] }}{{ k.u || '' }}</b></div>
        </div>
        <div class="two">
          <div><h4>{{ t('能力雷达') }}</h4><v-chart :option="radar" style="height:250px" autoresize /></div>
          <div><h4>{{ t('最近活动') }}</h4>
            <el-timeline class="tl">
              <el-timeline-item v-for="a in recent" :key="a.id" :timestamp="relTime(a.at)" :color="a.result === '预约估价' ? '#c9a227' : ''" size="small">{{ t(ACT[a.type].label) }} {{ a.propId ? propById(a.propId).addr : '' }} · {{ t(a.result) }}</el-timeline-item>
            </el-timeline>
          </div>
        </div>
        <h4>{{ t('权限') }}</h4>
        <div class="perm">
          <div v-for="p in PERMS" :key="p.k" class="pr"><div class="grow"><b>{{ t(p.l) }}</b><div class="small muted">{{ p.k === 'tp' ? state.settings.provider + ' · ' : '' }}{{ t(p.d) }}</div></div><el-switch :model-value="p.f(sel)" :disabled="me.role !== 'admin' || sel.id === 'ping'" @change="ElMessage.success(t('已更新，同步到 Entra 组'))" /></div>
        </div>
      </div>
    </div>

    <el-dialog v-model="rv.show" :title="t('撤销访问')" width="460px" :close-on-click-modal="false">
      <el-steps direction="vertical" :active="rv.step" finish-status="success">
        <el-step :title="t('停用 Entra 会话')" :description="t('该账号所有 token 立即失效')" />
        <el-step :title="t('补发手机离线队列')" :description="t('等待 {name} 手机上 2 条待发送记录写入 Rex', { name: sel?.name })" />
        <el-step :title="t('清除手机缓存')" :description="t('删除本地房产与联系人缓存')" />
        <el-step :title="t('锁定 App')" :description="t('下次打开显示「访问已撤销」')" />
      </el-steps>
      <template #footer><el-button type="primary" :disabled="rv.step < 4" @click="rv.show = false">{{ rv.step < 4 ? t('处理中… {n}s', { n: rv.sec }) : t('完成') }}</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Lock } from '@element-plus/icons-vue'
import VChart from '../components/chart'
import Avatar from '../components/Avatar.vue'
import { state, me, USERS, ACT, scopeAgents, userById, propById, relTime, audit, notify } from '../data/store'
import { PERIODS, leaderboard } from '../data/metrics'
import { t, tx, locale } from '../i18n'

const route = useRoute()
const period = ref('30d'), selId = ref(route.query.u || null)
const list = computed(() => me.value.role === 'admin' ? USERS : USERS.filter(u => u.team === me.value.team))
const sel = computed(() => list.value.find(u => u.id === selId.value) || list.value[0])
const pick = id => selId.value = id
const lb = computed(() => (state.acts.length, leaderboard(period.value, USERS.filter(u => u.id !== 'ping').map(u => u.id))))
const stat = id => lb.value.find(x => x.id === id)
const kk = [{ k: 'knocks', l: '上门拜访' }, { k: 'talks', l: '有效交谈' }, { k: 'rate', l: '交谈率', u: '%' }, { k: 'captured', l: '录入 Rex' }, { k: 'appraisals', l: '预约估价' }, { k: 'listings', l: '签约' }]
const alerts = computed(() => state.notifications.filter(n => n.type === 'warning' && !n.read && me.value.role === 'admin').map(n => ({ ...n, u: 'jason', id: n.id, read: false, _n: n })))
const recent = computed(() => state.acts.filter(a => a.agent === sel.value.id).slice(-8).reverse())
const radar = computed(() => {
  const mx = k => Math.max(1, ...lb.value.map(x => x[k])), s = stat(sel.value.id) || {}
  const ks = ['knocks', 'rate', 'captured', 'appraisals', 'linked']
  return { radar: { indicator: [{ name: t('拜访量') }, { name: t('交谈率') }, { name: t('录入') }, { name: t('估价') }, { name: t('新关联') }].map((x, i) => ({ ...x, max: mx(ks[i]) })), radius: '65%', splitArea: { areaStyle: { color: ['#fbfaf7', '#f5f3ee'] } }, axisName: { color: '#475467', fontSize: 11 } },
    series: [{ type: 'radar', data: [{ value: ks.map(k => s[k] || 0), areaStyle: { color: 'rgba(201,162,39,.25)' }, lineStyle: { color: '#c9a227' }, itemStyle: { color: '#c9a227' } }] }] }
})
const PERMS = [
  { k: 'map', l: '查看区域地图', d: '按 Rex 可见范围着色', f: () => true },
  { k: 'tp', l: '查看第三方业主名', d: '本人账户', f: u => u.role !== 'assistant' },
  { k: 'team', l: '查看团队数据', d: '团队负责人可见本团队', f: u => ['leader', 'admin'].includes(u.role) },
  { k: 'export', l: '导出数据', d: '仅管理员，所有导出留痕', f: u => u.role === 'admin' },
]
const rv = reactive({ show: false, step: 0, sec: 0 })
async function revoke() {
  await ElMessageBox.confirm(t('撤销后 {name} 将在 1 分钟内无法访问任何数据，手机缓存会被清除。确认？', { name: sel.value.name }), t('撤销访问'), { type: 'warning', confirmButtonText: t('立即撤销'), confirmButtonClass: 'el-button--danger' }).catch(() => { throw 0 })
  rv.show = true; rv.step = 0; rv.sec = 0
  const u = sel.value
  const iv = setInterval(() => { rv.sec++; if (rv.sec % 2 === 0) rv.step++; if (rv.step >= 4) { clearInterval(iv); u.status = 'revoked'; audit('撤销访问', u.name, `用时 ${rv.sec}s`); notify('访问已撤销', '{name} 的会话与手机缓存已清除', 'info', undefined, { name: u.name }) } }, 600)
}
function restore() { sel.value.status = 'active'; audit('恢复访问', sel.value.name, ''); ElMessage.success(t('已恢复')) }
</script>

<style scoped>
.tm { display: grid; grid-template-columns: 340px 1fr; gap: 16px; align-items: start; }
.ul { display: flex; flex-direction: column; gap: 10px; }
.uc { display: flex; gap: 12px; align-items: center; padding: 12px; cursor: pointer; border-width: 1.5px; }
.uc.on { border-color: var(--gold); }
.uc.off { opacity: .55; }
.mini { text-align: center; } .mini b { display: block; font-size: 20px; } .mini span { font-size: 11px; color: var(--mut); }
.ud { padding: 20px 24px; }
.uh { display: flex; gap: 16px; align-items: center; }
.nm { font-size: 26px; font-weight: 700; }
.stats { display: grid; grid-template-columns: repeat(6, minmax(0, 1fr)); gap: 10px; margin: 20px 0; background: #f8f7f3; border-radius: 12px; padding: 14px; }
.stats b { font-size: 24px; }
.two { display: grid; grid-template-columns: 1fr 1fr; gap: 20px; }
h4 { margin: 10px 0; font-size: 14px; }
.tl { max-height: 250px; overflow: auto; padding: 4px; }
.perm { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; }
.pr .grow { min-width: 0; }
.pr { display: flex; align-items: center; gap: 10px; border: 1px solid var(--line); border-radius: 10px; padding: 10px 12px; }
</style>
