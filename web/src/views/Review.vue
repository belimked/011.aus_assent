<template>
  <div class="page">
    <div class="page-head">
      <div><h1>{{ t('审核队列') }}</h1><div class="sub">{{ t('地址匹配、AI 不确定字段、门口新地址、OCR 批次、范围外匹配 · 快捷键') }} <span class="kbd">A</span> {{ t('通过') }} <span class="kbd">S</span> {{ t('跳过') }}</div></div>
      <div class="sp" />
      <el-progress type="circle" :percentage="pct" :width="54" :stroke-width="5" color="#16a34a"><span class="small"><b>{{ doneN }}</b></span></el-progress>
      <div class="small muted">{{ t('今日已处理 {n} 项', { n: doneN }) }}<br>{{ t('剩余 {n} 项', { n: state.reviews.length }) }}</div>
    </div>
    <el-tabs v-model="tab">
      <el-tab-pane v-for="tabItem in TABS" :key="tabItem.k" :name="tabItem.k"><template #label>{{ t(tabItem.l) }} <el-badge :value="cnt(tabItem.k)" :hidden="!cnt(tabItem.k)" type="info" class="tb" /></template></el-tab-pane>
    </el-tabs>

    <div class="rv">
      <div class="rlist card">
        <div v-for="r in items" :key="r.id" class="ri" :class="{ on: cur?.id === r.id }" @click="curId = r.id">
          <el-icon :color="KIND[r.kind].c" :size="18"><component :is="KIND[r.kind].i" /></el-icon>
          <div class="grow"><div class="ellipsis"><b>{{ title(r) }}</b></div><div class="small muted ellipsis">{{ t(KIND[r.kind].l) }} · {{ relTime(r.at) }}</div></div>
          <el-tag v-if="r.score" size="small" :type="r.score > 85 ? 'success' : 'warning'">{{ r.score }}%</el-tag>
          <el-tag v-if="r.conf" size="small" type="warning">{{ r.conf }}%</el-tag>
        </div>
        <el-result v-if="!items.length" icon="success" :title="t('全部处理完了')" :sub-title="t('这一类没有待审核项目')" />
      </div>

      <div v-if="cur" class="card rd">
        <div class="card-h"><el-tag :color="KIND[cur.kind].c" effect="dark" style="border:0">{{ t(KIND[cur.kind].l) }}</el-tag><div class="sp" /><span class="small muted">{{ cur.agent ? t('提交人 {name} · ', { name: userById(cur.agent).name }) : '' }}{{ relTime(cur.at) }}</span></div>
        <div class="card-b">
          <!-- 地址匹配 -->
          <template v-if="cur.kind === 'address'">
            <div class="cmp">
              <div class="side"><div class="small muted">{{ t('Rex 中的地址') }}</div><div class="big">{{ cur.rexAddr }}</div><div class="small muted">{{ t('Rex 房产记录') }}</div></div>
              <div class="mid"><div class="score serif">{{ cur.score }}%</div><div class="small muted">{{ t('相似度') }}</div><el-icon :size="22"><Right /></el-icon></div>
              <div class="side"><div class="small muted">{{ t('候选地籍地块（DCDB / G-NAF）') }}</div><div class="big">{{ cur.cand }}</div><div class="small muted">{{ t('差异：') }}{{ t(cur.reason) }}</div></div>
            </div>
            <div ref="mini" class="mini" />
          </template>
          <!-- AI 字段 -->
          <template v-else-if="cur.kind === 'ai'">
            <div class="small muted">{{ t('房产') }} {{ propById(cur.propId).addr }} · {{ t('字段「{field}」AI 置信度 {conf}%', { field: t(cur.field), conf: cur.conf }) }}</div>
            <div class="tr">🎙 {{ t(cur.transcript) }} <el-button link type="primary" size="small" :icon="VideoPlay" @click="ElMessage.info(t('演示：播放原始录音片段'))">{{ t('播放原音') }}</el-button></div>
            <div class="opts">
              <div class="opt" :class="{ on: choice === cur.value }" @click="choice = cur.value"><div class="small muted">{{ t('AI 识别') }}</div><b>{{ t(cur.value) }}</b></div>
              <div class="opt" :class="{ on: choice === cur.alt }" @click="choice = cur.alt"><div class="small muted">{{ t('备选') }}</div><b>{{ t(cur.alt) }}</b></div>
              <div class="opt" :class="{ on: custom && choice === custom }"><div class="small muted">{{ t('手动输入') }}</div><el-input v-model="custom" size="small" @focus="choice = custom" @input="choice = custom" /></div>
            </div>
          </template>
          <!-- OCR -->
          <template v-else-if="cur.kind === 'ocr'">
            <div class="small muted" style="margin-bottom:8px">{{ t('黄色单元格为 OCR 不确定，点击即可修改。确认后整批写入 Rex。') }}</div>
            <el-table :data="cur.rows" size="small" border>
              <el-table-column v-for="c in ['name', 'mobile', 'email', 'interest']" :key="c" :label="t({ name: '姓名', mobile: '手机', email: '电邮', interest: '意向' }[c])">
                <template #default="{ row }"><el-input v-model="row[c]" size="small" :class="{ uns: row.unsure.includes(c) }" @input="row.unsure = row.unsure.filter(x => x !== c)" /></template>
              </el-table-column>
              <el-table-column width="50"><template #default="{ $index }"><el-button link type="danger" :icon="Delete" @click="cur.rows.splice($index, 1)" /></template></el-table-column>
            </el-table>
          </template>
          <!-- 新地址 / 范围外 -->
          <template v-else>
            <div class="big">{{ cur.addr }}</div>
            <p class="muted">{{ t(cur.why || cur.reason) }}</p>
            <el-alert v-if="cur.kind === 'scope'" type="info" :closable="false" show-icon :title="t('经纪人看不到这条匹配。你可以联系两个团队协调，或忽略。')" />
          </template>
        </div>
        <div class="acts">
          <el-button @click="skip">{{ t('跳过') }} <span class="kbd">S</span></el-button>
          <el-button v-if="cur.kind === 'address'" @click="reject">{{ t('不是同一地址') }}</el-button>
          <div class="sp" />
          <el-button class="gold-btn" :disabled="!canOk" @click="approve">{{ okLabel }} <span class="kbd" style="margin-left:6px">A</span></el-button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, nextTick, onMounted, onBeforeUnmount } from 'vue'
import L from 'leaflet'
import { ElMessage } from 'element-plus'
import { VideoPlay, Delete } from '@element-plus/icons-vue'
import { state, propById, userById, relTime, audit } from '../data/store'
import { t, tx, locale } from '../i18n'

const KIND = { address: { l: '地址匹配', i: 'Location', c: '#64748b' }, ai: { l: 'AI 不确定字段', i: 'MagicStick', c: '#b8860b' }, new: { l: '门口新地址', i: 'CirclePlus', c: '#0e7490' }, ocr: { l: 'OCR 批次', i: 'Camera', c: '#7c3aed' }, scope: { l: '范围外匹配', i: 'Lock', c: '#dc2626' } }
const TABS = [{ k: 'all', l: '全部' }, ...Object.entries(KIND).map(([k, v]) => ({ k, l: v.l }))]
const tab = ref('all'), curId = ref(null), choice = ref(''), custom = ref(''), doneN = ref(0), mini = ref()
const cnt = k => state.reviews.filter(r => k === 'all' || r.kind === k).length
const items = computed(() => state.reviews.filter(r => tab.value === 'all' || r.kind === tab.value))
const cur = computed(() => items.value.find(r => r.id === curId.value) || items.value[0])
const pct = computed(() => Math.round(doneN.value / Math.max(1, doneN.value + state.reviews.length) * 100))
const title = r => r.kind === 'address' ? r.rexAddr : r.kind === 'ai' ? `${propById(r.propId).addr} · ${t(r.field)}` : r.kind === 'ocr' ? t(r.title) : r.addr
const canOk = computed(() => cur.value?.kind !== 'ai' || !!choice.value)
const okLabel = computed(() => { const r = cur.value; if (!r) return ''; return r.kind === 'ocr' ? t('整批写入 Rex（{n} 行）', { n: r.rows.length }) : t({ address: '确认关联', ai: '确认并写入 Rex', new: '在 Rex 新建地址', scope: '已知悉' }[r.kind]) })
watch(cur, (r) => { choice.value = r?.kind === 'ai' ? r.value : ''; custom.value = ''; if (r?.kind === 'address') nextTick(drawMini) }, { immediate: true })
let mm
function drawMini() {
  mm?.remove(); if (!mini.value) return
  const p = propById(cur.value.propId)
  mm = L.map(mini.value, { zoomControl: false, attributionControl: false, dragging: false, scrollWheelZoom: false }).setView([p.lat, p.lng], 18)
  L.tileLayer('https://server.arcgisonline.com/ArcGIS/rest/services/Canvas/World_Light_Gray_Base/MapServer/tile/{z}/{y}/{x}', { maxZoom: 19, maxNativeZoom: 16 }).addTo(mm)
  L.circle([p.lat, p.lng], { radius: 14, color: '#e6c35c', weight: 3, fillOpacity: .15 }).addTo(mm)
}
function next(r) { const i = state.reviews.indexOf(r); state.reviews.splice(i, 1); curId.value = items.value[Math.min(i, items.value.length - 1)]?.id }
function approve() {
  const r = cur.value; if (!r || !canOk.value) return
  if (r.kind === 'address') { const p = propById(r.propId); p.status = 'red'; p.rexId = 40000 + p.id * 13; audit('地址关联', p.addr, `Rex ↔ ${r.cand}`) }
  if (r.kind === 'ai') audit('AI 字段确认', propById(r.propId).addr, `${r.field} = ${choice.value}`)
  if (r.kind === 'ocr') { if (r.rows.some(x => x.unsure.length)) return ElMessage.warning(t('还有黄色单元格没核对')); audit('OCR 批量写入', r.title, r.rows.length + ' 位联系人') }
  if (r.kind === 'new') audit('新建地址', r.addr, '已在 Rex 创建')
  doneN.value++; ElMessage.success(t('已处理')); next(r)
}
function reject() { audit('拒绝匹配', cur.value.rexAddr, cur.value.cand); doneN.value++; ElMessage.info(t('已标记为不匹配，将重新搜索候选地块')); next(cur.value) }
function skip() { const r = cur.value; if (!r) return; state.reviews.push(state.reviews.splice(state.reviews.indexOf(r), 1)[0]); curId.value = items.value[0]?.id }
const key = e => { if (['INPUT', 'TEXTAREA'].includes(e.target.tagName)) return; if (e.key === 'a') approve(); if (e.key === 's') skip() }
onMounted(() => window.addEventListener('keydown', key))
onBeforeUnmount(() => { window.removeEventListener('keydown', key); mm?.remove() })
</script>

<style scoped>
.tb { margin-left: 4px; }
.tb :deep(.el-badge__content) { position: static; transform: none; }
.rv { display: grid; grid-template-columns: 360px 1fr; gap: 16px; align-items: start; }
.rlist { max-height: calc(100vh - 260px); overflow: auto; padding: 6px; }
.ri { display: flex; align-items: center; gap: 10px; padding: 10px; border-radius: 10px; cursor: pointer; }
.ri:hover { background: #f7f8fa; }
.ri.on { background: var(--gold-soft); box-shadow: inset 3px 0 0 var(--gold); }
.cmp { display: grid; grid-template-columns: 1fr 120px 1fr; gap: 12px; align-items: center; }
.side { border: 1px solid var(--line); border-radius: 12px; padding: 14px; }
.big { font-size: 18px; font-weight: 700; margin: 6px 0; }
.mid { text-align: center; }
.score { font-size: 30px; font-weight: 700; color: var(--navy); }
.mini { height: 240px; border-radius: 12px; margin-top: 14px; overflow: hidden; }
.tr { background: #f7f8fa; border-radius: 10px; padding: 12px; margin: 10px 0 14px; line-height: 1.6; }
.opts { display: grid; grid-template-columns: repeat(3, 1fr); gap: 10px; }
.opt { border: 1.5px solid var(--line); border-radius: 12px; padding: 12px; cursor: pointer; }
.opt.on { border-color: var(--gold); background: var(--gold-soft); }
.opt b { font-size: 16px; display: block; margin-top: 4px; }
:deep(.uns .el-input__wrapper) { background: #fef9c3; box-shadow: 0 0 0 1px #facc15 inset; }
.acts { display: flex; gap: 8px; padding: 0 18px 18px; }
.sp { flex: 1; }
</style>
