<template>
  <div class="page">
    <!-- 问候 + 控制 -->
    <div class="hello">
      <div>
        <div class="serif hi">{{ greet }}{{ t('，') }}{{ me.name.split(' ')[0] }}</div>
        <div class="muted">{{ todayStr }} · {{ t('布里斯班') }} · <b style="color:#b45309">{{ todo.length }}</b> {{ t('项跟进待办，') }}<b style="color:#0e7490">{{ todayOfi.length }}</b> {{ t('场开放参观') }}</div>
      </div>
      <div class="sp" />
      <el-segmented v-if="me.role !== 'agent'" v-model="scope" :options="scopeOpts" />
      <el-segmented v-model="period" :options="PERIODS.map(p => ({ label: t(p.l), value: p.k }))" />
      <el-tooltip :content="t('实时模拟：外勤经纪人的新记录每几秒推送一次')"><el-switch v-model="state.live" inline-prompt :active-text="t('实时')" :inactive-text="t('暂停')" style="--el-switch-on-color:#16a34a" /></el-tooltip>
    </div>

    <!-- KPI -->
    <div class="kpis">
      <KpiCard :label="t('上门拜访')" icon="House" color="#0e7490" :value="K.cur.knocks" :d="delta(K.cur.knocks, K.prev.knocks)" :foot="t('较上期 {n}', { n: K.prev.knocks })" :spark="tr.map(x => x.knocks)" to="/properties?visited=1" />
      <KpiCard :label="t('有效交谈')" icon="ChatDotRound" color="#2563eb" :value="K.cur.talks" :d="delta(K.cur.talks, K.prev.talks)" :foot="t('交谈率 {n}%', { n: rate(K.cur.talks, K.cur.knocks) })" :spark="tr.map(x => x.talks)" :tip="t('与业主交谈、租客应门、预约估价')" />
      <KpiCard :label="t('新关联业主')" icon="Link" color="#16a34a" :value="K.cur.linked" :d="delta(K.cur.linked, K.prev.linked)" :foot="t('红点 → 绿点')" to="/map?f=green" />
      <KpiCard :label="t('预约估价')" icon="Calendar" color="#b8860b" :value="K.cur.appraisals" :d="delta(K.cur.appraisals, K.prev.appraisals)" :foot="t('潜在房源线索')" to="/tasks" />
      <KpiCard :label="t('开放参观登记')" icon="Key" color="#7c3aed" :value="K.cur.ofi" :d="delta(K.cur.ofi, K.prev.ofi)" :foot="t('访客人次')" to="/ofi" />
      <KpiCard :label="t('区域覆盖率')" icon="PieChart" color="#16a34a" :value="cov.pct" unit="%" :foot="t('{n} / {total} 已关联', { n: cov.green, total: cov.total })" to="/map" :tip="t('已关联业主的房产占比（按你的 Rex 可见范围计算）')" />
    </div>

    <div class="g1">
      <!-- 转化漏斗 -->
      <div class="card">
        <div class="card-h"><h3>{{ t('采集转化漏斗') }}</h3><span class="small muted">{{ t(periodLabel) }} · {{ scopeLabel }}</span></div>
        <div class="card-b funnel">
          <div v-for="(f, i) in funnel" :key="f.l" class="fs" @click="f.to && router.push(f.to)" :class="{ clickable: f.to }">
            <div class="fbar" :style="{ width: Math.max(8, f.v / funnel[0].v * 100) + '%', background: f.c }"><span>{{ f.v }}</span></div>
            <div class="fl"><b>{{ f.l }}</b><span v-if="i" class="small muted">{{ t('转化 {n}%', { n: rate(f.v, funnel[i - 1].v) }) }}</span></div>
          </div>
          <div class="insight"><el-icon color="#b8860b"><Opportunity /></el-icon><span>{{ funnelInsight }}</span></div>
        </div>
      </div>
      <!-- 趋势 -->
      <div class="card">
        <div class="card-h"><h3>{{ t('每日活动趋势') }}</h3><div class="sp" />
          <el-radio-group v-model="trendKind" size="small"><el-radio-button value="bar">{{ t('柱状') }}</el-radio-button><el-radio-button value="line">{{ t('折线') }}</el-radio-button></el-radio-group>
        </div>
        <v-chart class="ch" :option="trendOpt" autoresize @click="e => ElMessage.info(`${e.name}: ${e.seriesName} ${e.value}`)" />
      </div>
      <!-- 覆盖率 -->
      <div class="card">
        <div class="card-h"><h3>{{ t('各区覆盖率') }}</h3><div class="sp" /><el-button link size="small" @click="router.push('/map')">{{ t('地图查看 →') }}</el-button></div>
        <div class="card-b">
          <div v-for="s in subs" :key="s.suburb" class="sub" @click="router.push('/map?suburb=' + encodeURIComponent(s.suburb))">
            <div class="row"><b class="grow">{{ s.suburb }}</b><span class="serif pct">{{ s.pct }}%</span></div>
            <div class="stack">
              <i v-for="k in ['green', 'other', 'red', 'grey']" :key="k" :style="{ width: s[k] / s.total * 100 + '%', background: STATUS[k].color }" :title="t(STATUS[k].label) + ' ' + s[k]" />
            </div>
            <div class="small muted">{{ t('{n} 处房产 · 未关联 {red} · 待匹配 {grey}', { n: s.total, red: s.red, grey: s.grey }) }}</div>
          </div>
          <div class="legend"><span v-for="k in ['green', 'other', 'red', 'grey']" :key="k"><i class="dot" :style="{ background: STATUS[k].color }" />{{ t(STATUS[k].short) }}</span></div>
        </div>
      </div>
    </div>

    <div class="g2">
      <!-- 今日日程 -->
      <div class="card">
        <div class="card-h"><h3>{{ t('今日日程') }}</h3><el-tag size="small" type="warning" round v-if="overdue.length">{{ t('{n} 项逾期', { n: overdue.length }) }}</el-tag><div class="sp" /><el-button link size="small" @click="router.push('/tasks')">{{ t('全部 →') }}</el-button></div>
        <div class="card-b agenda">
          <div v-for="o in todayOfi" :key="o.id" class="ag ofi" @click="router.push('/ofi?o=' + o.id)">
            <div class="tm">{{ fmtTime(o.start) }}</div>
            <div class="grow"><div><el-tag size="small" effect="dark" color="#7c3aed" style="border:0">{{ t('开放参观') }}</el-tag> <b>{{ listingById(o.listing).addr }}</b></div><div class="small muted">{{ ofiState(o) }} · {{ t('已登记 {n} 人', { n: o.attendees.length }) }}</div></div>
            <el-icon><ArrowRight /></el-icon>
          </div>
          <div v-for="t in todo.slice(0, 7)" :key="t.id" class="ag" :class="{ late: t.due < startToday }">
            <el-checkbox :model-value="t.done" @change="done(t)" @click.stop />
            <div class="tm">{{ t.due < startToday ? fmtMD(t.due) : fmtTime(t.due) }}</div>
            <div class="grow" @click="openProp(t.propId)" style="cursor:pointer">
              <div class="ellipsis"><b>{{ tFn(t.title) }}</b> · {{ t.contact }}</div>
              <div class="small muted ellipsis">{{ propById(t.propId)?.addr }} · {{ userById(t.agent)?.name }}</div>
            </div>
            <el-tooltip :content="tFn('拨打 {n}', { n: t.mobile || '' })"><el-button circle size="small" :icon="Phone" @click="ElMessage.success(tFn('拨打 {name}（演示）', { name: t.contact }))" /></el-tooltip>
          </div>
          <el-empty v-if="!todo.length && !todayOfi.length" :description="t('今天没有待办，去地图上找找机会吧')" :image-size="70" />
        </div>
      </div>
      <!-- 潜在卖家机会 -->
      <div class="card">
        <div class="card-h"><h3>{{ t('潜在卖家机会') }}</h3><el-tooltip :content="t('未关联业主 + 持有 10 年以上 + 近 7 天未拜访 + 未设勿上门；持有越久、越久没去，评分越高')"><el-icon class="muted"><QuestionFilled /></el-icon></el-tooltip><div class="sp" /><el-button link size="small" @click="router.push('/properties?opp=1')">{{ t('全部 →') }}</el-button></div>
        <div class="card-b">
          <div v-for="o in opps" :key="o.p.id" class="opp" @click="openProp(o.p.id)">
            <div class="score" :style="{ background: `conic-gradient(#c9a227 ${o.score * 3.6}deg, #f1ead2 0)` }"><span>{{ o.score }}</span></div>
            <div class="grow"><div class="ellipsis"><b>{{ o.p.addr }}</b> <span class="small muted">{{ o.p.suburb }}</span></div>
              <div class="small muted">{{ t(o.p.tp.owner) }} · {{ t('持有 {n} 年', { n: o.years }) }} · {{ o.p.lastVisit ? t('上次 {n}', { n: relTime(o.p.lastVisit) }) : t('从未拜访') }}</div></div>
            <span class="small est">{{ money(o.p.estLow * 1000) }}+</span>
          </div>
        </div>
      </div>
      <!-- 实时动态 -->
      <div class="card">
        <div class="card-h"><h3>{{ t('外勤实时动态') }}</h3><span class="dot pulse" style="background:#22c55e" v-if="state.live" /><div class="sp" /><el-select v-model="feedType" size="small" style="width:145px" :placeholder="t('全部')"><el-option :label="t('全部')" value="" /><el-option :label="t('仅录入')" value="cap" /><el-option :label="t('仅估价')" value="appr" /></el-select></div>
        <div class="card-b feed">
          <TransitionGroup name="fade-up">
            <div v-for="a in feed" :key="a.id" class="fi" @click="a.propId && openProp(a.propId)">
              <Avatar :id="a.agent" :size="30" />
              <div class="grow">
                <div class="ellipsis"><b>{{ userById(a.agent).name.split(' ')[0] }}</b> {{ t(ACT[a.type].label) }} <b v-if="a.propId">{{ propById(a.propId).addr }}</b></div>
                <div class="small"><span :class="['res', resultCls(a)]">{{ t(a.result) }}</span><span v-if="a.captured" class="muted"> · {{ t('已写入 Rex') }}</span><span v-if="a.linked" style="color:#16a34a"> · {{ t('新关联') }}</span></div>
              </div>
              <span class="small muted">{{ relTime(a.at) }}</span>
            </div>
          </TransitionGroup>
        </div>
      </div>
    </div>

    <div class="g3">
      <!-- 排行 -->
      <div class="card">
        <div class="card-h"><h3>{{ t('经纪人表现') }}</h3><span class="small muted">{{ t(periodLabel) }}</span><div class="sp" />
          <el-select v-model="lbSort" size="small" style="width:150px"><el-option v-for="o in lbSorts" :key="o.v" :label="t('按{n}', { n: t(o.l) })" :value="o.v" /></el-select>
        </div>
        <div class="card-b">
          <el-table :data="lb" size="small" @row-click="r => router.push('/team?u=' + r.id)" row-class-name="clk">
            <el-table-column width="46"><template #default="{ $index }"><span class="rank" :class="'r' + $index">{{ $index + 1 }}</span></template></el-table-column>
            <el-table-column :label="t('经纪人')" min-width="150"><template #default="{ row }"><div class="row"><Avatar :id="row.id" :size="26" /><div><div><b>{{ row.user.name }}</b></div><div class="small muted">{{ t(row.user.team) }}</div></div></div></template></el-table-column>
            <el-table-column :label="t('拜访')" prop="knocks" width="80" align="right" />
            <el-table-column :label="t('交谈率')" width="140"><template #default="{ row }"><el-progress :percentage="row.rate" :stroke-width="6" :color="'#2563eb'" /></template></el-table-column>
            <el-table-column :label="t('录入')" prop="captured" width="85" align="right" />
            <el-table-column :label="t('估价')" prop="appraisals" width="90" align="right"><template #default="{ row }"><b style="color:#b8860b">{{ row.appraisals }}</b></template></el-table-column>
            <el-table-column :label="t('签约')" prop="listings" width="85" align="right" />
          </el-table>
        </div>
      </div>
      <!-- 热力图 -->
      <div class="card">
        <div class="card-h"><h3>{{ t('最佳上门时段') }}</h3><el-tooltip :content="t('颜色越深 = 交谈率越高。基于历史拜访结果')"><el-icon class="muted"><QuestionFilled /></el-icon></el-tooltip><div class="sp" /><el-radio-group v-model="heatKind" size="small"><el-radio-button value="rate">{{ t('交谈率') }}</el-radio-button><el-radio-button value="n">{{ t('拜访量') }}</el-radio-button></el-radio-group></div>
        <v-chart class="ch" :option="heatOpt" autoresize />
        <div class="insight" style="margin:0 18px 16px"><el-icon color="#b8860b"><Opportunity /></el-icon><span>{{ heatInsight }}</span></div>
      </div>
      <!-- 结果构成 -->
      <div class="card">
        <div class="card-h"><h3>{{ t('拜访结果构成') }}</h3></div>
        <v-chart class="ch" :option="pieOpt" autoresize />
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Phone } from '@element-plus/icons-vue'
import VChart from '../components/chart'
import KpiCard from '../components/KpiCard.vue'
import Avatar from '../components/Avatar.vue'
import { state, me, scopeAgents, USERS, STATUS, ACT, TALK, userById, propById, listingById, now, fmtTime, fmtMD, relTime, money, startOfDay, WEEK } from '../data/store'
import { PERIODS, kpis, delta, coverage, suburbCoverage, trend, heat, leaderboard, resultMix, opportunities } from '../data/metrics'
import { t, tx, locale } from '../i18n'
import { openProp } from '../data/ui'

defineOptions({ name: 'Dashboard' })
const router = useRouter()
const period = ref('7d'), scope = ref('all'), trendKind = ref('bar'), heatKind = ref('rate'), feedType = ref(''), lbSort = ref('score')
const tFn = t // task iteration variable shadows t in the template
const scopeOpts = computed(() => me.value.role === 'admin' ? [{ label: t('全办公室'), value: 'all' }, { label: t('Owen 团队'), value: 'Owen 团队' }, { label: t('Lily 团队'), value: 'Lily 团队' }] : [{ label: t('我的团队'), value: 'all' }, { label: t('仅我'), value: 'me' }])
watch(() => state.me, () => scope.value = 'all')
const agents = computed(() => {
  const base = scopeAgents.value
  if (scope.value === 'me') return [me.value.id]
  if (scope.value !== 'all') return base.filter(id => userById(id).team === scope.value)
  return base
})
const scopeLabel = computed(() => scopeOpts.value.find(o => o.value === scope.value)?.label || t('我'))
const periodLabel = computed(() => PERIODS.find(p => p.k === period.value).l)
const rate = (a, b) => b ? Math.round(a / b * 100) : 0

const tick = ref(0)
const K = computed(() => (state.acts.length, kpis(period.value, agents.value)))
const tr = computed(() => (state.acts.length, trend(period.value, agents.value)))
const cov = computed(() => (state.acts.length, coverage()))
const subs = computed(() => (state.acts.length, suburbCoverage()))
const opps = computed(() => (state.acts.length, opportunities(6)))
const lbSorts = [{ v: 'score', l: '综合' }, { v: 'knocks', l: '拜访量' }, { v: 'rate', l: '交谈率' }, { v: 'appraisals', l: '估价' }]
const lb = computed(() => { const L = (state.acts.length, leaderboard(period.value, agents.value)); return lbSort.value === 'score' ? L : [...L].sort((a, b) => b[lbSort.value] - a[lbSort.value]) })

const funnel = computed(() => [
  { l: t('上门拜访'), v: K.value.cur.knocks, c: '#0f2a44' },
  { l: t('有效交谈'), v: K.value.cur.talks, c: '#1f4b73' },
  { l: t('语音录入 Rex'), v: K.value.cur.captured, c: '#2f6b9a' },
  { l: t('预约估价'), v: K.value.cur.appraisals, c: '#c9a227', to: '/tasks' },
  { l: t('签约委托'), v: K.value.cur.listings, c: '#16a34a' },
])
const funnelInsight = computed(() => {
  const c = K.value.cur, r = rate(c.captured, c.talks)
  if (r < 80) return t('有 {n} 次交谈没有录入 Rex，信息可能流失。建议交谈后立即语音录入。', { n: c.talks - c.captured })
  return t('录入率 {n}%，表现很好。交谈 → 估价转化 {rate}%，可以重点跟进潜在卖家。', { n: r, rate: rate(c.appraisals, c.talks) })
})

const hour = now().getHours()
const greet = computed(() => t(hour < 12 ? '早上好' : hour < 18 ? '下午好' : '晚上好'))
const todayStr = computed(() => { const d = now(); return locale.value === 'en' ? d.toLocaleDateString('en-AU', { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' }) : `${d.getFullYear()}${t('年')}${d.getMonth() + 1}${t('月')}${d.getDate()}${t('日')} ${WEEK[d.getDay()]}` })
const startToday = computed(() => startOfDay(now()))
const myTaskScope = t => me.value.role === 'admin' || agents.value.includes(t.agent)
const todo = computed(() => state.tasks.filter(t => !t.done && myTaskScope(t) && t.due < new Date(startToday.value.getTime() + 864e5)).sort((a, b) => a.due - b.due))
const overdue = computed(() => todo.value.filter(t => t.due < startToday.value))
const todayOfi = computed(() => state.opens.filter(o => startOfDay(o.start).getTime() === startToday.value.getTime()))
const ofiState = o => now() < o.start ? `${fmtTime(o.start)}–${fmtTime(o.end)} · ${t('未开始')}` : now() < o.end ? t('进行中') : t('已结束')
function done(t) { t.done = true; ElMessage({ type: 'success', message: tFn('已完成「{name}」', { name: tFn(t.title) }), grouping: true }) }

const feed = computed(() => state.acts.filter(a => agents.value.includes(a.agent) && (feedType.value === '' || (feedType.value === 'cap' ? a.captured : a.result === '预约估价'))).slice(-12).reverse())
const resultCls = a => a.result === '预约估价' || a.type === 'listing' ? 'gold' : TALK.has(a.result) ? 'blue' : ''

const axisC = '#98a2b3'
const trendOpt = computed(() => ({
  grid: { left: 36, right: 14, top: 34, bottom: 28 },
  tooltip: { trigger: 'axis' },
  legend: { top: 4, right: 10, itemWidth: 10, itemHeight: 10, textStyle: { fontSize: 12 } },
  xAxis: { type: 'category', data: tr.value.map(x => `${x.d.getMonth() + 1}/${x.d.getDate()}`), axisLine: { lineStyle: { color: '#e5e7eb' } }, axisLabel: { color: axisC, fontSize: 11 } },
  yAxis: { type: 'value', splitLine: { lineStyle: { color: '#f1f2f4' } }, axisLabel: { color: axisC, fontSize: 11 } },
  dataZoom: tr.value.length > 31 ? [{ type: 'inside' }] : [],
  series: [
    { name: t('拜访'), type: trendKind.value, data: tr.value.map(x => x.knocks), itemStyle: { color: '#0f2a44', borderRadius: [3, 3, 0, 0] }, smooth: true, barMaxWidth: 16 },
    { name: t('交谈'), type: trendKind.value, data: tr.value.map(x => x.talks), itemStyle: { color: '#6f9bc4', borderRadius: [3, 3, 0, 0] }, smooth: true, barMaxWidth: 16 },
    { name: t('录入'), type: 'line', data: tr.value.map(x => x.captured), itemStyle: { color: '#c9a227' }, smooth: true, symbol: 'none', lineStyle: { width: 2.5 } },
  ],
}))
const H = computed(() => (state.acts.length, heat(period.value, agents.value)))
const HOURS = Array.from({ length: 11 }, (_, i) => i + 9)
const DAYS = [1, 2, 3, 4, 5, 6, 0]
const heatOpt = computed(() => {
  const data = []
  DAYS.forEach((d, y) => HOURS.forEach((h, x) => { const c = H.value[d + '-' + h]; const v = !c ? 0 : heatKind.value === 'n' ? c.n : (c.n >= 2 ? Math.round(c.t / c.n * 100) : 0); data.push([x, y, v]) }))
  const mx = Math.max(1, ...data.map(d => d[2]))
  return {
    grid: { left: 44, right: 12, top: 8, bottom: 46 },
    tooltip: { formatter: p => `${WEEK[DAYS[p.value[1]]]} ${HOURS[p.value[0]]}:00<br/>${heatKind.value === 'n' ? t('拜访 {n} 次', { n: p.value[2] }) : t('交谈率 {n}%', { n: p.value[2] })}` },
    xAxis: { type: 'category', data: HOURS.map(h => t('{n}点', { n: h })), splitArea: { show: true }, axisLabel: { color: axisC, fontSize: 10 }, axisLine: { show: false }, axisTick: { show: false } },
    yAxis: { type: 'category', data: DAYS.map(d => WEEK[d]), axisLabel: { color: axisC, fontSize: 11 }, axisLine: { show: false }, axisTick: { show: false } },
    visualMap: { min: 0, max: mx, calculable: false, orient: 'horizontal', left: 'center', bottom: 0, itemHeight: 120, itemWidth: 10, inRange: { color: ['#f7f3e6', '#e6c35c', '#b8860b', '#0f2a44'] }, textStyle: { fontSize: 10, color: axisC } },
    series: [{ type: 'heatmap', data, itemStyle: { borderColor: '#fff', borderWidth: 2, borderRadius: 3 } }],
  }
})
const heatInsight = computed(() => {
  let best = null
  DAYS.forEach(d => HOURS.forEach(h => { const c = H.value[d + '-' + h]; if (c && c.n >= 3) { const r = c.t / c.n; if (!best || r > best.r) best = { d, h, r } } }))
  return best ? t('{day} {start}:00–{end}:00 交谈率最高（{n}%），适合安排扫街。', { day: WEEK[best.d], start: best.h, end: best.h + 1, n: Math.round(best.r * 100) }) : t('数据不足')
})
const pieOpt = computed(() => ({
  tooltip: { trigger: 'item', formatter: p => t('{name}: {n} 次 ({pct}%)', { name: p.name, n: p.value, pct: p.percent }) },
  legend: { bottom: 0, itemWidth: 10, itemHeight: 10, textStyle: { fontSize: 11 } },
  color: ['#cbd5e1', '#94a3b8', '#2563eb', '#0f2a44', '#c9a227', '#dc2626', '#16a34a'],
  series: [{ type: 'pie', radius: ['48%', '72%'], center: ['50%', '44%'], itemStyle: { borderColor: '#fff', borderWidth: 2, borderRadius: 4 }, label: { show: false }, emphasis: { label: { show: true, fontSize: 13, fontWeight: 'bold', formatter: '{b}\n{d}%' } }, data: resultMix(K.value.list).map(({ name, value }) => ({ name: t(name), value })) }],
}))
</script>

<style scoped>
.hello { display: flex; align-items: flex-end; gap: 12px; margin-bottom: 18px; flex-wrap: wrap; }
.hi { font-size: 28px; font-weight: 700; color: var(--navy); margin-bottom: 4px; }
.sp { flex: 1; }
.kpis { display: grid; grid-template-columns: repeat(6, 1fr); gap: 14px; margin-bottom: 16px; }
.g1 { display: grid; grid-template-columns: 1fr 1.5fr 1fr; gap: 16px; margin-bottom: 16px; }
.g2 { display: grid; grid-template-columns: 1.15fr 1fr 1fr; gap: 16px; margin-bottom: 16px; }
.g3 { display: grid; grid-template-columns: 1.4fr 1.1fr .8fr; gap: 16px; }
@media (max-width: 1400px) { .kpis { grid-template-columns: repeat(3, 1fr); } .g1, .g2, .g3 { grid-template-columns: 1fr 1fr; } }
.ch { height: 270px; }
.funnel .fs { margin-bottom: 9px; }
.fs.clickable { cursor: pointer; }
.fbar { height: 26px; border-radius: 6px; color: #fff; font-weight: 700; display: flex; align-items: center; padding: 0 10px; font-size: 13px; transition: width .6s; }
.fl { display: flex; justify-content: space-between; font-size: 12px; margin-top: 3px; }
.insight { display: flex; gap: 8px; align-items: flex-start; background: var(--gold-soft); border-radius: 10px; padding: 9px 12px; font-size: 12.5px; color: #6b5310; margin-top: 10px; line-height: 1.5; }
.sub { padding: 8px 0; cursor: pointer; border-radius: 8px; }
.sub:hover .stack { box-shadow: 0 0 0 2px #e6c35c; }
.pct { font-weight: 700; font-size: 17px; color: var(--navy); }
.stack { display: flex; height: 9px; border-radius: 9px; overflow: hidden; margin: 5px 0 3px; background: #f1f2f4; }
.stack i { display: block; height: 100%; transition: width .5s; }
.legend { display: flex; gap: 12px; font-size: 12px; color: var(--mut); margin-top: 8px; }
.legend span { display: flex; align-items: center; gap: 4px; }
.legend .dot { width: 8px; height: 8px; }
.agenda { max-height: 380px; overflow: auto; }
.ag { display: flex; align-items: center; gap: 10px; padding: 9px 10px; border-radius: 10px; border: 1px solid transparent; }
.ag:hover { background: #f8f9fb; border-color: var(--line); }
.ag.ofi { background: #f5f3ff; cursor: pointer; margin-bottom: 6px; }
.ag.late .tm { color: #dc2626; }
.tm { font-weight: 700; font-size: 13px; width: 52px; color: var(--navy); }
.opp { display: flex; align-items: center; gap: 12px; padding: 8px 6px; border-radius: 10px; cursor: pointer; }
.opp:hover { background: #fbf8ee; }
.score { width: 38px; height: 38px; border-radius: 50%; display: grid; place-items: center; flex: none; }
.score span { width: 30px; height: 30px; border-radius: 50%; background: #fff; display: grid; place-items: center; font-size: 12px; font-weight: 800; color: #8a6a12; }
.est { color: var(--navy); font-weight: 700; }
.feed { max-height: 380px; overflow: hidden; }
.fi { display: flex; align-items: center; gap: 10px; padding: 7px 4px; border-bottom: 1px dashed #eef0f3; cursor: pointer; font-size: 13px; }
.res.gold { color: #b8860b; font-weight: 700; }
.res.blue { color: #2563eb; }
.rank { display: inline-grid; place-items: center; width: 22px; height: 22px; border-radius: 6px; background: #f1f2f4; font-weight: 700; font-size: 12px; }
.rank.r0 { background: #c9a227; color: #fff; } .rank.r1 { background: #a8b3c2; color: #fff; } .rank.r2 { background: #c08457; color: #fff; }
:deep(.clk) { cursor: pointer; }
</style>
