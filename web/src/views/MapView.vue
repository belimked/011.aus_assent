<template>
  <div class="mv">
    <!-- 左侧面板 -->
    <aside class="panel">
      <div class="ph">
        <el-input v-model="q" :placeholder="t('街道或门牌…')" :prefix-icon="Search" clearable />
        <div class="row" style="margin-top:10px">
          <el-select v-model="suburb" :placeholder="t('全部区域')" clearable style="flex:1" @change="fitSuburb"><el-option v-for="s in SUBURBS" :key="s" :label="s" :value="s" /></el-select>
          <el-select v-model="sort" style="width:150px"><el-option :label="t('按门牌')" value="addr" /><el-option :label="t('持有最久')" value="held" /><el-option :label="t('最久未访')" value="visit" /></el-select>
        </div>
        <div class="chips">
          <button v-for="f in FILTERS" :key="f.k" class="chip" :class="{ on: filter === f.k }" @click="filter = f.k">
            <i v-if="f.c" :style="{ background: f.c }" />{{ t(f.l) }}<span>{{ count(f.k) }}</span>
          </button>
        </div>
        <div class="row small muted" style="margin-top:8px">
          <el-checkbox v-model="inView" size="small">{{ t('只看地图视野内') }}</el-checkbox>
          <div class="grow" />{{ t('共 {n} 处', { n: list.length }) }}
        </div>
      </div>
      <div v-if="planning" class="plan">
        <div class="row"><el-icon color="#b8860b"><Guide /></el-icon><b>{{ t('扫街路线') }}</b><span class="small muted">{{ t('已选 {n} 处 · 约 {min} 分钟步行', { n: route.length, min: walkMin }) }}</span></div>
        <div class="small muted" style="margin:4px 0 8px">{{ t('在地图或列表上点击房产加入路线，拖动顺序会自动优化。') }}</div>
        <div class="row">
          <el-button size="small" @click="autoPick">{{ t('智能推荐 10 处') }}</el-button>
          <el-button size="small" :disabled="!route.length" @click="route = []">{{ t('清空') }}</el-button>
          <div class="grow" />
          <el-button size="small" class="gold-btn" :disabled="!route.length" @click="sendRoute">{{ t('发送到手机') }}</el-button>
        </div>
      </div>
      <el-scrollbar class="list">
        <div v-for="p in list.slice(0, shown)" :key="p.id" class="li" :class="{ hov: hover === p.id, inr: route.includes(p.id) }"
          @mouseenter="hover = p.id" @mouseleave="hover = null" @click="clickProp(p)">
          <span class="dot" :style="{ background: STATUS[viewStatus(p)].color }" />
          <div class="grow">
            <div class="row"><b class="ellipsis">{{ p.addr }}</b><el-icon v-if="p.flags.dnk" color="#dc2626" :title="t('勿上门')"><CircleCloseFilled /></el-icon><el-tag v-for="tag in p.tags" :key="tag" size="small" type="warning" effect="plain" round>{{ t(tag) }}</el-tag></div>
            <div class="small muted ellipsis">{{ sub(p) }}</div>
          </div>
          <span v-if="planning" class="ri">{{ route.includes(p.id) ? route.indexOf(p.id) + 1 : '+' }}</span>
          <el-icon v-else class="muted"><ArrowRight /></el-icon>
        </div>
        <div v-if="list.length > shown" class="more"><el-button link @click="shown += 60">{{ t('再显示 {n} 条', { n: Math.min(60, list.length - shown) }) }}</el-button></div>
        <el-empty v-if="!list.length" :description="t('没有符合条件的房产')" :image-size="70"><el-button size="small" @click="reset">{{ t('清除筛选') }}</el-button></el-empty>
      </el-scrollbar>
    </aside>

    <!-- 地图 -->
    <div class="mapwrap">
      <div ref="el" class="map" />
      <div class="tools">
        <el-button-group>
          <el-button :type="planning ? 'primary' : ''" :icon="Guide" @click="planning = !planning">{{ t(planning ? '退出规划' : '规划扫街') }}</el-button>
          <el-button :icon="Aim" @click="locate">{{ t('附近') }}</el-button>
          <el-button :icon="FullScreen" @click="fitAll">{{ t('全部') }}</el-button>
        </el-button-group>
        <el-switch v-model="showHeat" :active-text="t('拜访热度')" style="margin-left:12px;background:#fff;padding:4px 10px;border-radius:8px" />
      </div>
      <div class="cov">
        <div class="small muted">{{ t('当前视野覆盖率') }}</div>
        <div class="serif cv">{{ vcov.pct }}%</div>
        <div class="stack"><i v-for="k in ['green', 'other', 'red', 'grey']" :key="k" :style="{ width: vcov[k] / Math.max(1, vcov.total) * 100 + '%', background: STATUS[k].color }" /></div>
        <div class="leg"><span v-for="k in ['green', 'red', 'other', 'grey']" :key="k"><i :style="{ background: STATUS[k].color }" />{{ t(STATUS[k].short) }} {{ vcov[k] }}</span></div>
        <div class="small muted" style="margin-top:6px">{{ t('白心红点 = 第三方有业主名 · 方块 = 单元楼') }}</div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onActivated, nextTick, shallowRef } from 'vue'
import { useRoute } from 'vue-router'
import L from 'leaflet'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search, Guide, Aim, FullScreen } from '@element-plus/icons-vue'
import { state, STATUS, SUBURBS, viewStatus, yearsHeld, relTime, userById, me, now, DAY, audit } from '../data/store'
import { openProp } from '../data/ui'
import { t, tx, locale } from '../i18n'

defineOptions({ name: 'MapView' })
const r = useRoute()
const q = ref(''), suburb = ref(''), filter = ref('all'), sort = ref('addr'), inView = ref(true), hover = ref(null), shown = ref(60)
const planning = ref(false), route = ref([]), showHeat = ref(false)
const el = ref(), bounds = shallowRef(null)
const FILTERS = [
  { k: 'all', l: '全部' }, { k: 'red', l: '未关联', c: '#dc2626' }, { k: 'green', l: '已关联', c: '#16a34a' }, { k: 'other', l: '他人', c: '#8b5cf6' }, { k: 'grey', l: '待匹配', c: '#9ca3af' },
  { k: 'owner', l: '有业主名' }, { k: 'held10', l: '持有10年+' }, { k: 'stale', l: '90天未访' }, { k: 'dnk', l: '勿上门' },
]
const match = (p, k) => {
  const s = viewStatus(p)
  switch (k) {
    case 'all': return true
    case 'owner': return s === 'red' && !!p.tp
    case 'held10': return (yearsHeld(p) ?? 0) >= 10
    case 'stale': return !p.lastVisit || now() - p.lastVisit > 90 * DAY
    case 'dnk': return p.flags.dnk
    default: return s === k
  }
}
const base = computed(() => state.props.filter(p => (!suburb.value || p.suburb === suburb.value) && (!q.value || p.addr.toLowerCase().includes(q.value.toLowerCase()))))
const count = k => base.value.filter(p => match(p, k)).length
const list = computed(() => {
  let L = base.value.filter(p => match(p, filter.value))
  if (inView.value && bounds.value) L = L.filter(p => bounds.value.contains([p.lat, p.lng]))
  const s = sort.value
  return [...L].sort((a, b) => s === 'held' ? (yearsHeld(b) ?? -1) - (yearsHeld(a) ?? -1) : s === 'visit' ? (a.lastVisit || 0) - (b.lastVisit || 0) : a.street.localeCompare(b.street) || (+a.num - +b.num) || (a.unit || 0) - (b.unit || 0))
})
const vcov = computed(() => {
  const c = { green: 0, red: 0, grey: 0, other: 0 }
  const L = state.props.filter(p => (!bounds.value || bounds.value.contains([p.lat, p.lng])) && (!suburb.value || p.suburb === suburb.value))
  L.forEach(p => c[viewStatus(p)]++)
  return { ...c, total: L.length, pct: L.length ? Math.round(c.green / L.length * 100) : 0 }
})
const sub = p => {
  const s = viewStatus(p)
  const who = s === 'green' ? p.contacts[0]?.name : s === 'red' && p.tp ? p.tp.owner + t('（第三方）') : s === 'other' ? t('其他团队') : t('待匹配')
  return `${who} · ${p.lastVisit ? relTime(p.lastVisit) + ' ' + t(p.lastResult) : t('从未拜访')}${p.tp ? ' · ' + t('持有 {n} 年', { n: yearsHeld(p) }) : ''}`
}

/* ---------------- 地图 ---------------- */
let map, layer, heatLayer, routeLine
const mk = new Map()
function icon(p) {
  const s = viewStatus(p), c = STATUS[s].color
  const inR = route.value.indexOf(p.id)
  const ring = hover.value === p.id ? 'box-shadow:0 0 0 4px rgba(201,162,39,.55);' : ''
  if (inR >= 0) return L.divIcon({ className: '', iconSize: [22, 22], html: `<div style="width:22px;height:22px;border-radius:50%;background:#0f2a44;color:#e6c35c;border:2px solid #e6c35c;font:700 11px/18px sans-serif;text-align:center">${inR + 1}</div>` })
  const shape = p.unit ? 'border-radius:3px' : 'border-radius:50%'
  const inner = s === 'red' && p.tp ? `<i style="position:absolute;inset:3.5px;background:#fff;border-radius:50%"></i>` : ''
  const dnk = p.flags.dnk ? `<b style="position:absolute;right:-5px;top:-6px;font-size:9px">⛔</b>` : ''
  const dim = filter.value !== 'all' && !match(p, filter.value) ? 'opacity:.18;' : ''
  return L.divIcon({ className: '', iconSize: [14, 14], html: `<div style="position:relative;width:14px;height:14px;background:${c};${shape};border:2px solid #fff;${ring}${dim}box-shadow:0 1px 3px rgba(0,0,0,.35)${ring ? ',0 0 0 4px rgba(201,162,39,.55)' : ''}">${inner}${dnk}</div>` })
}
function draw() {
  if (!map) return
  layer.clearLayers(); mk.clear()
  base.value.forEach(p => {
    const m = L.marker([p.lat + (p.unit ? (p.unit - 3.5) * 0.00004 : 0), p.lng + (p.unit ? 0.00012 : 0)], { icon: icon(p), riseOnHover: true })
      .bindTooltip(`<b>${p.addr}</b><br>${t(STATUS[viewStatus(p)].label)}${p.tp && viewStatus(p) === 'red' ? '<br>' + t('业主：') + p.tp.owner : ''}`, { direction: 'top', offset: [0, -6] })
      .on('click', () => clickProp(p)).on('mouseover', () => hover.value = p.id).on('mouseout', () => hover.value = null)
    m.addTo(layer); mk.set(p.id, m)
  })
  drawHeat(); drawRoute()
}
function refresh(id) { const p = state.props.find(x => x.id === id), m = mk.get(id); if (p && m) m.setIcon(icon(p)) }
function drawHeat() {
  heatLayer.clearLayers(); if (!showHeat.value) return
  const cnt = {}; state.acts.forEach(a => { if (a.type === 'knock' && now() - a.at < 30 * DAY) cnt[a.propId] = (cnt[a.propId] || 0) + 1 })
  base.value.forEach(p => { const n = cnt[p.id]; if (n) L.circle([p.lat, p.lng], { radius: 18 + n * 9, stroke: false, fillColor: '#c9a227', fillOpacity: Math.min(.5, .12 + n * .07) }).addTo(heatLayer) })
}
function drawRoute() {
  routeLine?.remove(); routeLine = null
  if (route.value.length > 1) routeLine = L.polyline(route.value.map(id => { const p = state.props.find(x => x.id === id); return [p.lat, p.lng] }), { color: '#0f2a44', weight: 3, dashArray: '6 6' }).addTo(map)
}
function clickProp(p) {
  if (!planning.value) return openProp(p.id)
  if (p.flags.dnk) return ElMessage.warning(t('{addr} 标记为勿上门，不能加入路线', { addr: p.addr }))
  const i = route.value.indexOf(p.id)
  i >= 0 ? route.value.splice(i, 1) : route.value.push(p.id)
  optimise()
}
function optimise() { // 最近邻排序
  if (route.value.length < 3) return
  const pts = route.value.map(id => state.props.find(x => x.id === id))
  const out = [pts.shift()]
  while (pts.length) { const l = out[out.length - 1]; let bi = 0, bd = 1e9; pts.forEach((p, i) => { const d = Math.hypot(p.lat - l.lat, p.lng - l.lng); if (d < bd) { bd = d; bi = i } }); out.push(pts.splice(bi, 1)[0]) }
  route.value = out.map(p => p.id)
}
const walkMin = computed(() => {
  let d = 0; const P = route.value.map(id => state.props.find(x => x.id === id))
  for (let i = 1; i < P.length; i++) d += Math.hypot((P[i].lat - P[i - 1].lat) * 111, (P[i].lng - P[i - 1].lng) * 99)
  return Math.round(d / 4.5 * 60 + P.length * 4)
})
function autoPick() {
  const c = list.value.filter(p => viewStatus(p) === 'red' && !p.flags.dnk && !route.value.includes(p.id)).sort((a, b) => (yearsHeld(b) ?? 0) - (yearsHeld(a) ?? 0) + ((a.lastVisit || 0) - (b.lastVisit || 0)) / 1e12).slice(0, 10)
  if (!c.length) return ElMessage.info(t('视野内没有合适的未关联房产，换个区域试试'))
  route.value.push(...c.map(p => p.id)); optimise()
  ElMessage.success(t('已推荐 {n} 处：未关联、持有时间长、较久未拜访', { n: c.length }))
}
async function sendRoute() {
  const who = me.value.role === 'agent' ? me.value.name : null
  const { value } = await ElMessageBox.prompt(t('发送给哪位经纪人？路线会出现在他的 App「今日扫街」里。'), t('发送扫街路线'), { inputValue: who || 'Kevin Zhang', confirmButtonText: t('发送') }).catch(() => ({}))
  if (!value) return
  audit('发送扫街路线', `${route.value.length} 处房产`, '接收人：' + value)
  ElMessage.success(t('已发送 {n} 处路线给 {name}', { n: route.value.length, name: value })); route.value = []; planning.value = false
}
function fitSuburb() { const pts = base.value.map(p => [p.lat, p.lng]); if (pts.length) map.flyToBounds(pts, { padding: [40, 40], duration: .6 }) }
function fitAll() { suburb.value = ''; nextTick(fitSuburb) }
function locate() { map.flyTo([-27.5813, 153.0650], 17, { duration: .8 }); ElMessage.info(t('已定位到当前位置附近（演示：Gager St）')) }
function reset() { q.value = ''; suburb.value = ''; filter.value = 'all'; inView.value = false }

watch([base, filter], draw)
watch(locale, draw)
watch(hover, (n, o) => { o && refresh(o); n && refresh(n) })
watch(route, (n, o) => { draw() }, { deep: true })
watch(showHeat, drawHeat)
watch(() => state.acts.length, () => { const a = state.acts[state.acts.length - 1]; if (a?.propId) refresh(a.propId); if (showHeat.value) drawHeat() })
watch(() => state.props.map(p => p.status + p.flags.dnk).join(), draw)
watch(() => state.me, draw)
watch(filter, () => shown.value = 60)

function applyQuery() {
  if (r.query.f) filter.value = r.query.f
  if (r.query.suburb) { suburb.value = r.query.suburb; nextTick(fitSuburb) }
  if (r.query.p) { const p = state.props.find(x => x.id === +r.query.p); if (p) { map.flyTo([p.lat, p.lng], 18); openProp(p.id) } }
}
onMounted(() => {
  map = L.map(el.value, { zoomControl: true, attributionControl: false }).setView([-27.582, 153.063], 15)
  L.tileLayer('https://server.arcgisonline.com/ArcGIS/rest/services/Canvas/World_Light_Gray_Base/MapServer/tile/{z}/{y}/{x}', { maxZoom: 19, maxNativeZoom: 16 }).addTo(map)
  L.tileLayer('https://server.arcgisonline.com/ArcGIS/rest/services/Canvas/World_Light_Gray_Reference/MapServer/tile/{z}/{y}/{x}', { maxZoom: 19, maxNativeZoom: 16 }).addTo(map)
  heatLayer = L.layerGroup().addTo(map); layer = L.layerGroup().addTo(map)
  const upd = () => bounds.value = map.getBounds()
  map.on('moveend', upd); upd(); draw(); applyQuery()
})
onActivated(() => { setTimeout(() => map?.invalidateSize(), 60); applyQuery() })
</script>

<style scoped>
.mv { display: flex; height: 100%; }
.panel { width: 380px; flex: none; background: #fff; border-right: 1px solid var(--line); display: flex; flex-direction: column; min-height: 0; }
.ph { padding: 14px 14px 10px; border-bottom: 1px solid var(--line); }
.chips { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 10px; }
.chip { border: 1px solid var(--line); background: #fff; border-radius: 99px; padding: 3px 10px; font-size: 12px; display: flex; align-items: center; gap: 5px; cursor: pointer; }
.chip i { width: 8px; height: 8px; border-radius: 50%; }
.chip span { color: var(--mut); font-size: 11px; }
.chip.on { background: var(--navy); border-color: var(--navy); color: #fff; }
.chip.on span { color: #cbd5e1; }
.plan { padding: 12px 14px; background: var(--gold-soft); border-bottom: 1px solid #f0e2b1; }
.plan .row { gap: 6px; }
.list { flex: 1; min-height: 0; }
.li { display: flex; align-items: center; gap: 10px; padding: 10px 14px; border-bottom: 1px solid #f3f4f6; cursor: pointer; }
.li.hov { background: #f7f8fa; }
.li.inr { background: #fbf5e2; }
.ri { width: 22px; height: 22px; border-radius: 50%; border: 1.5px dashed #c9a227; color: #8a6a12; font-size: 11px; font-weight: 700; display: grid; place-items: center; flex: none; }
.inr .ri { background: #0f2a44; border: 2px solid #e6c35c; color: #e6c35c; }
.more { text-align: center; padding: 10px; }
.mapwrap { flex: 1; position: relative; }
.map { position: absolute; inset: 0; }
.tools { position: absolute; z-index: 500; top: 12px; left: 56px; display: flex; align-items: center; }
.tools :deep(.el-button-group) { box-shadow: 0 2px 10px rgba(0, 0, 0, .12); border-radius: 8px; }
.cov { position: absolute; z-index: 500; right: 14px; top: 12px; width: 250px; background: rgba(255, 255, 255, .96); border-radius: 14px; padding: 12px 14px; box-shadow: 0 6px 24px rgba(15, 42, 68, .15); }
.cv { font-size: 30px; font-weight: 700; color: var(--navy); }
.stack { display: flex; height: 8px; border-radius: 8px; overflow: hidden; margin: 6px 0; background: #eee; }
.stack i { display: block; transition: width .4s; }
.leg { display: grid; grid-template-columns: 1fr 1fr; gap: 2px 8px; font-size: 12px; }
.leg span { display: flex; align-items: center; gap: 5px; }
.leg i { width: 8px; height: 8px; border-radius: 50%; }
</style>
