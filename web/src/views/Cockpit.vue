<template>
  <div class="ck" ref="root">
    <header>
      <div class="row">
        <div class="logo"><svg viewBox="0 0 32 32"><path d="M6 16l10-9 10 9v10h-7v-7h-6v7H6z" fill="#e6c35c" /></svg></div>
        <div><div class="serif t1">Place Sunnybank · {{ t('业主信息采集驾驶舱') }}</div><div class="t2">REX CAPTURE COMMAND CENTRE</div></div>
      </div>
      <div class="ctr">
        <el-segmented v-model="period" :options="PERIODS.map(p => ({ label: t(p.l), value: p.k }))" size="small" />
      </div>
      <div class="row right">
        <div class="live"><span class="dot pulse" :style="{ background: state.live ? '#22c55e' : '#f59e0b' }" />{{ state.live ? t('LIVE') : t('PAUSED') }}</div>
        <div class="clock serif">{{ clock }}</div>
        <el-tooltip :content="t('全屏 (F)')"><el-button circle :icon="FullScreen" @click="fs" /></el-tooltip>
        <el-tooltip :content="t('返回工作台 (Esc)')"><el-button circle :icon="Close" @click="router.push('/dashboard')" /></el-tooltip>
      </div>
    </header>

    <div class="body">
      <!-- 左列 -->
      <div class="col">
        <div class="panel kp">
          <div v-for="k in kpiList" :key="k.l" class="kk">
            <div class="kl">{{ k.l }}</div>
            <div class="kv serif" :style="{ color: k.c }">{{ k.v }}<small v-if="k.u">{{ k.u }}</small></div>
            <div class="kd" :class="k.d >= 0 ? 'up' : 'down'">{{ k.d >= 0 ? '▲' : '▼' }} {{ Math.abs(k.d) }}% <span>{{ t('较上期') }}</span></div>
          </div>
        </div>
        <div class="panel">
          <div class="ph">{{ t('覆盖率') }}</div>
          <v-chart class="gauge" :option="gaugeOpt" autoresize />
          <div class="subs">
            <div v-for="s in subs" :key="s.suburb" class="sb" :class="{ on: focus === s.suburb }" @click="focusSuburb(s.suburb)">
              <span class="grow">{{ s.suburb }}</span><b>{{ s.pct }}%</b>
              <div class="bar"><i :style="{ width: s.pct + '%' }" /></div>
            </div>
          </div>
        </div>
        <div class="panel grow-p">
          <div class="ph">{{ t('采集漏斗') }}</div>
          <v-chart class="fn" :option="funnelOpt" autoresize />
        </div>
      </div>

      <!-- 中列：地图 -->
      <div class="col mid">
        <div class="panel map-p">
          <div ref="mapEl" class="map" />
          <div class="map-legend">
            <span v-for="k in ['green', 'red', 'other', 'grey']" :key="k" :class="{ off: hidden.includes(k) }" @click="toggle(k)"><i :style="{ background: STATUS[k].color }" />{{ t(STATUS[k].short) }} {{ cov[k] }}</span>
          </div>
          <div class="map-tip">{{ focus ? t('{name} · 点击区域条切换', { name: focus }) : t('全区 {n} 处房产 · 闪烁 = 刚刚有经纪人到访', { n: cov.total }) }}</div>
          <transition name="pop"><div v-if="toast" class="map-toast" :key="toast.id"><Avatar :id="toast.agent" :size="34" /><div><b>{{ userById(toast.agent).name }}</b> {{ t(toast.result) }}<div class="small">{{ propById(toast.propId).addr }} · {{ propById(toast.propId).suburb }}</div></div></div></transition>
        </div>
        <div class="panel tr-p">
          <div class="ph">{{ t('每日活动 · 拜访 / 交谈 / 录入') }}</div>
          <v-chart class="trc" :option="trendOpt" autoresize />
        </div>
      </div>

      <!-- 右列 -->
      <div class="col">
        <div class="panel lb-p">
          <div class="ph">{{ t('经纪人排行') }}</div>
          <div v-for="(r, i) in lb" :key="r.id" class="lbr">
            <span class="rk" :class="'r' + i">{{ i + 1 }}</span>
            <Avatar :id="r.id" :size="28" />
            <div class="grow"><div class="nm">{{ r.user.name }}</div>
              <div class="bar"><i :style="{ width: r.knocks / Math.max(1, lb[0].knocks) * 100 + '%', background: r.user.color }" /></div></div>
            <div class="nums"><b>{{ r.knocks }}</b><span>{{ t('拜访') }}</span></div>
            <div class="nums"><b style="color:#e6c35c">{{ r.appraisals }}</b><span>{{ t('估价') }}</span></div>
          </div>
        </div>
        <div class="panel">
          <div class="ph">{{ t('结果构成') }}</div>
          <v-chart class="pie" :option="pieOpt" autoresize />
        </div>
        <div class="panel grow-p feed-p">
          <div class="ph">{{ t('实时动态') }} <span class="cnt">{{ t('{n} 条今日记录', { n: todayCount }) }}</span></div>
          <TransitionGroup name="slide" tag="div" class="feed">
            <div v-for="a in feed" :key="a.id" class="fi">
              <span class="ft">{{ fmtTime(a.at) }}</span>
              <span class="fa" :style="{ color: userById(a.agent).color }">{{ userById(a.agent).name.split(' ')[0] }}</span>
              <span class="grow ellipsis">{{ a.propId ? propById(a.propId).addr : t(ACT[a.type].label) }}</span>
              <span class="fr" :class="{ g: a.result === '预约估价' || a.type === 'listing', b: TALK.has(a.result) }">{{ t(a.result) }}</span>
            </div>
          </TransitionGroup>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount, watch, shallowRef } from 'vue'
import { useRouter } from 'vue-router'
import L from 'leaflet'
import { darkLayer } from '../components/tiles'
import { FullScreen, Close } from '@element-plus/icons-vue'
import VChart from '../components/chart'
import Avatar from '../components/Avatar.vue'
import { state, STATUS, ACT, TALK, viewStatus, userById, propById, scopeAgents, now, fmtTime, startOfDay, SUBURBS } from '../data/store'
import { t, tx, locale } from '../i18n'
import { PERIODS, kpis, delta, coverage, suburbCoverage, trend, leaderboard, resultMix } from '../data/metrics'

const router = useRouter()
const period = ref('30d'), focus = ref(null), hidden = ref([]), clock = ref(''), toast = ref(null)
const mapEl = ref(), root = ref()
const agents = computed(() => scopeAgents.value)
const K = computed(() => (state.acts.length, kpis(period.value, agents.value)))
const cov = computed(() => (state.acts.length, coverage(focus.value)))
const subs = computed(() => (state.acts.length, suburbCoverage()))
const lb = computed(() => (state.acts.length, leaderboard(period.value, agents.value).slice(0, 6)))
const tr = computed(() => (state.acts.length, trend(period.value, agents.value)))
const feed = computed(() => state.acts.slice(-14).reverse())
const todayCount = computed(() => state.acts.filter(a => a.at >= startOfDay(now())).length)
const kpiList = computed(() => {
  const c = K.value.cur, p = K.value.prev
  return [
    { l: t('上门拜访'), v: c.knocks, d: delta(c.knocks, p.knocks), c: '#fff' },
    { l: t('有效交谈'), v: c.talks, d: delta(c.talks, p.talks), c: '#7cc4ff' },
    { l: t('录入 Rex'), v: c.captured, d: delta(c.captured, p.captured), c: '#7cc4ff' },
    { l: t('新关联业主'), v: c.linked, d: delta(c.linked, p.linked), c: '#4ade80' },
    { l: t('预约估价'), v: c.appraisals, d: delta(c.appraisals, p.appraisals), c: '#e6c35c' },
    { l: t('签约委托'), v: c.listings, d: delta(c.listings, p.listings), c: '#e6c35c' },
  ]
})
const axisText = { color: '#8ea3bd', fontSize: 11 }
const gaugeOpt = computed(() => ({
  series: [{ type: 'gauge', startAngle: 210, endAngle: -30, min: 0, max: 100, radius: '95%', center: ['50%', '58%'], progress: { show: true, width: 12, itemStyle: { color: { type: 'linear', x: 0, y: 0, x2: 1, y2: 0, colorStops: [{ offset: 0, color: '#16a34a' }, { offset: 1, color: '#e6c35c' }] } } },
    axisLine: { lineStyle: { width: 12, color: [[1, 'rgba(255,255,255,.08)']] } }, axisTick: { show: false }, splitLine: { show: false }, axisLabel: { show: false }, pointer: { show: false },
    title: { offsetCenter: [0, '38%'], color: '#8ea3bd', fontSize: 12 }, detail: { offsetCenter: [0, '0%'], fontSize: 34, fontWeight: 700, color: '#fff', formatter: '{value}%', fontFamily: 'Georgia' },
    data: [{ value: cov.value.pct, name: focus.value || t('全区已关联') }] }],
}))
const funnelOpt = computed(() => {
  const c = K.value.cur
  return { tooltip: { trigger: 'item', formatter: p => `${p.name}: ${p.value}` }, series: [{ type: 'funnel', left: 8, right: 96, top: 6, bottom: 6, minSize: '18%', gap: 3, sort: 'none', label: { color: '#fff', fontSize: 12, formatter: p => `${p.name}  ${p.value}` }, itemStyle: { borderWidth: 0 },
    data: [{ name: t('拜访'), value: c.knocks, itemStyle: { color: '#1f4b73' } }, { name: t('交谈'), value: c.talks, itemStyle: { color: '#2f6b9a' } }, { name: t('录入'), value: c.captured, itemStyle: { color: '#3f86bd' } }, { name: t('估价'), value: c.appraisals, itemStyle: { color: '#b8860b' } }, { name: t('签约'), value: c.listings, itemStyle: { color: '#16a34a' } }] }] }
})
const trendOpt = computed(() => ({
  grid: { left: 34, right: 10, top: 14, bottom: 22 }, tooltip: { trigger: 'axis' },
  xAxis: { type: 'category', data: tr.value.map(x => `${x.d.getMonth() + 1}/${x.d.getDate()}`), axisLabel: axisText, axisLine: { lineStyle: { color: 'rgba(255,255,255,.1)' } } },
  yAxis: { type: 'value', axisLabel: axisText, splitLine: { lineStyle: { color: 'rgba(255,255,255,.05)' } } },
  series: [
    { name: t('拜访'), type: 'bar', data: tr.value.map(x => x.knocks), itemStyle: { color: 'rgba(124,196,255,.35)', borderRadius: [2, 2, 0, 0] }, barMaxWidth: 12 },
    { name: t('交谈'), type: 'line', data: tr.value.map(x => x.talks), smooth: true, symbol: 'none', lineStyle: { color: '#7cc4ff', width: 2 } },
    { name: t('录入'), type: 'line', data: tr.value.map(x => x.captured), smooth: true, symbol: 'none', lineStyle: { color: '#e6c35c', width: 2 }, areaStyle: { color: 'rgba(230,195,92,.12)' } },
  ],
}))
const pieOpt = computed(() => ({
  tooltip: { trigger: 'item' }, color: ['#334e6d', '#4c6a8c', '#7cc4ff', '#2f6b9a', '#e6c35c', '#f87171'],
  legend: { orient: 'vertical', right: 6, top: 'middle', textStyle: { color: '#c3d0e0', fontSize: 11 }, itemWidth: 8, itemHeight: 8 },
  series: [{ type: 'pie', radius: ['50%', '78%'], center: ['32%', '50%'], label: { show: false }, itemStyle: { borderColor: '#0b1a2b', borderWidth: 2 }, data: resultMix(K.value.list).map(({ name, value }) => ({ name: t(name), value })) }],
}))

/* ---------- 地图 ---------- */
let map, layer
const markers = new Map()
function drawMap() {
  layer.clearLayers(); markers.clear()
  state.props.forEach(p => {
    const s = viewStatus(p); if (hidden.value.includes(s)) return
    if (focus.value && p.suburb !== focus.value) return
    const m = L.circleMarker([p.lat, p.lng], { radius: 4.5, weight: 0, fillOpacity: .9, fillColor: STATUS[s].color }).bindTooltip(tx(`${p.addr} · ${STATUS[s].short}`))
    m.addTo(layer); markers.set(p.id, m)
  })
}
function toggle(k) { const i = hidden.value.indexOf(k); i < 0 ? hidden.value.push(k) : hidden.value.splice(i, 1); drawMap() }
function focusSuburb(s) {
  focus.value = focus.value === s ? null : s; drawMap()
  const pts = state.props.filter(p => !focus.value || p.suburb === focus.value).map(p => [p.lat, p.lng])
  map.flyToBounds(pts, { padding: [40, 40], duration: .8 })
}
let lastId = 0
watch(() => state.acts.length, () => {
  const a = state.acts[state.acts.length - 1]
  if (!a || a.id === lastId || !a.propId || !map) return
  lastId = a.id
  const p = propById(a.propId)
  const ring = L.circleMarker([p.lat, p.lng], { radius: 6, color: '#e6c35c', weight: 2, fill: false, className: 'ring' }).addTo(map)
  setTimeout(() => ring.remove(), 2400)
  markers.get(p.id)?.setStyle({ fillColor: STATUS[viewStatus(p)].color })
  if (TALK.has(a.result)) { toast.value = a; clearTimeout(tt); tt = setTimeout(() => toast.value = null, 3500) }
})
let tt, ct
const keys = e => { if (e.key === 'Escape' && !document.fullscreenElement) router.push('/dashboard'); if (e.key.toLowerCase() === 'f') fs() }
function fs() { document.fullscreenElement ? document.exitFullscreen() : root.value.requestFullscreen?.() }
onMounted(() => {
  const tick = () => { const d = now(); clock.value = d.toTimeString().slice(0, 8) }
  tick(); ct = setInterval(tick, 1000)
  map = L.map(mapEl.value, { zoomControl: false }).setView([-27.587, 153.066], 14)
  darkLayer().addTo(map)
  layer = L.layerGroup().addTo(map); drawMap()
  map.fitBounds(state.props.map(p => [p.lat, p.lng]), { padding: [30, 30] })
  window.addEventListener('keydown', keys)
})
watch(locale, () => { if (layer) drawMap() })
onBeforeUnmount(() => { clearInterval(ct); window.removeEventListener('keydown', keys); map?.remove() })
</script>

<style scoped>
.ck { height: 100vh; background: radial-gradient(1200px 600px at 50% -10%, #17365a 0%, #0b1a2b 55%, #071320 100%); color: #e2e8f0; display: flex; flex-direction: column; overflow: hidden; }
header { height: 64px; display: flex; align-items: center; justify-content: space-between; padding: 0 22px; border-bottom: 1px solid rgba(230, 195, 92, .18); background: linear-gradient(180deg, rgba(230, 195, 92, .06), transparent); flex: none; }
.logo { width: 36px; height: 36px; border: 1px solid rgba(230, 195, 92, .5); border-radius: 9px; display: grid; place-items: center; margin-right: 10px; }
.logo svg { width: 22px; }
.t1 { font-size: 19px; font-weight: 700; color: #fff; letter-spacing: .5px; }
.t2 { font-size: 10px; letter-spacing: 3px; color: #e6c35c; }
.ctr :deep(.el-segmented) { --el-segmented-bg-color: rgba(255, 255, 255, .06); --el-segmented-item-selected-bg-color: #e6c35c; --el-segmented-item-selected-color: #1b1300; --el-segmented-color: #c3d0e0; --el-segmented-item-hover-bg-color: rgba(255,255,255,.08); }
.right { gap: 14px; }
.right :deep(.el-button) { background: rgba(255, 255, 255, .06); border-color: rgba(255, 255, 255, .12); color: #c3d0e0; }
.live { display: flex; align-items: center; gap: 6px; font-size: 12px; letter-spacing: 2px; color: #86efac; font-weight: 700; }
.clock { font-size: 24px; font-weight: 700; color: #fff; font-variant-numeric: tabular-nums; }
.body { flex: 1; display: grid; grid-template-columns: 1fr 2.1fr 1fr; gap: 14px; padding: 14px; min-height: 0; }
.col { display: flex; flex-direction: column; gap: 14px; min-height: 0; }
.panel { background: rgba(255, 255, 255, .035); border: 1px solid rgba(255, 255, 255, .07); border-radius: 14px; padding: 12px 14px; position: relative; }
.panel::before { content: ''; position: absolute; left: 14px; top: 0; width: 28px; height: 2px; background: #e6c35c; border-radius: 2px; }
.grow-p { flex: 1; min-height: 0; display: flex; flex-direction: column; }
.ph { font-size: 13px; color: #c3d0e0; font-weight: 600; letter-spacing: 1px; margin-bottom: 6px; display: flex; align-items: center; }
.cnt { margin-left: auto; font-size: 11px; color: #8ea3bd; font-weight: 400; letter-spacing: 0; }
.kp { display: grid; grid-template-columns: 1fr 1fr; gap: 10px 12px; }
.kk { padding: 4px 2px; }
.kl { font-size: 12px; color: #8ea3bd; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.kv { font-size: 28px; font-weight: 700; line-height: 1.15; font-variant-numeric: tabular-nums; }
.kv small { font-size: 13px; }
.kd { font-size: 11px; font-weight: 700; } .kd span { color: #64748b; font-weight: 400; }
.up { color: #4ade80; } .down { color: #f87171; }
.gauge { height: 150px; }
.subs { margin-top: 2px; }
.sb { display: flex; flex-wrap: wrap; align-items: center; font-size: 12px; padding: 4px 6px; border-radius: 6px; cursor: pointer; }
.sb:hover, .sb.on { background: rgba(230, 195, 92, .1); }
.sb b { color: #fff; }
.bar { flex-basis: 100%; height: 4px; background: rgba(255, 255, 255, .07); border-radius: 4px; margin-top: 3px; overflow: hidden; }
.bar i { display: block; height: 100%; background: linear-gradient(90deg, #16a34a, #e6c35c); transition: width .5s; }
.fn { flex: 1; min-height: 150px; }
.mid { gap: 14px; }
.map-p { flex: 1; padding: 0; overflow: hidden; min-height: 0; }
.map { position: absolute; inset: 0; border-radius: 14px; background: #0b1a2b; }
.map-legend { position: absolute; z-index: 500; left: 14px; top: 14px; display: flex; gap: 6px; }
.map-legend span { background: rgba(7, 19, 32, .82); border: 1px solid rgba(255, 255, 255, .1); padding: 4px 10px; border-radius: 99px; font-size: 12px; cursor: pointer; display: flex; align-items: center; gap: 5px; }
.map-legend span.off { opacity: .35; }
.map-legend i { width: 8px; height: 8px; border-radius: 50%; }
.map-tip { position: absolute; z-index: 500; left: 14px; bottom: 12px; font-size: 11px; color: #8ea3bd; background: rgba(7, 19, 32, .7); padding: 3px 9px; border-radius: 6px; }
.map-toast { position: absolute; z-index: 600; right: 14px; top: 14px; display: flex; gap: 10px; align-items: center; background: rgba(7, 19, 32, .92); border: 1px solid rgba(230, 195, 92, .5); padding: 10px 14px; border-radius: 12px; font-size: 13px; box-shadow: 0 10px 30px rgba(0, 0, 0, .4); }
.map-toast .small { color: #8ea3bd; }
.pop-enter-active, .pop-leave-active { transition: all .3s; }
.pop-enter-from, .pop-leave-to { opacity: 0; transform: translateY(-8px); }
.tr-p { height: 200px; flex: none; }
.trc { height: 160px; }
.lbr { display: flex; align-items: center; gap: 9px; padding: 5px 0; }
.rk { width: 20px; height: 20px; border-radius: 5px; background: rgba(255, 255, 255, .08); font-size: 11px; font-weight: 800; display: grid; place-items: center; }
.rk.r0 { background: #e6c35c; color: #1b1300; } .rk.r1 { background: #a8b3c2; color: #0b1a2b; } .rk.r2 { background: #c08457; color: #fff; }
.nm { font-size: 13px; }
.lbr .bar { margin-top: 4px; }
.nums { display: flex; flex-direction: column; align-items: flex-end; min-width: 38px; }
.nums b { font-size: 15px; } .nums span { font-size: 10px; color: #64748b; }
.pie { height: 140px; }
.feed-p { overflow: hidden; }
.feed { overflow: hidden; flex: 1; }
.fi { display: flex; gap: 8px; font-size: 12px; padding: 5px 0; border-bottom: 1px dashed rgba(255, 255, 255, .06); align-items: center; }
.ft { color: #64748b; font-variant-numeric: tabular-nums; }
.fa { font-weight: 700; width: 52px; }
.fr { color: #8ea3bd; max-width: 95px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; } .fr.b { color: #7cc4ff; } .fr.g { color: #e6c35c; font-weight: 700; }
.slide-enter-active { transition: all .5s; }
.slide-enter-from { opacity: 0; transform: translateX(20px); background: rgba(230, 195, 92, .15); }
:deep(.ring) { animation: ring 2.4s ease-out forwards; }
@keyframes ring { from { stroke-width: 3; opacity: 1; } to { stroke-width: 0; opacity: 0; transform-origin: center; } }
@media (max-width: 1300px) { .body { grid-template-columns: 1fr 1.6fr; } .body > .col:last-child { display: none; } }
</style>
