<template>
  <div class="page">
    <div class="page-head">
      <div><h1>{{ t('房产列表') }}</h1><div class="sub">{{ t('与地图共用数据 · 适合在电脑上规划拜访 · 第三方字段不可导出') }}</div></div>
      <div class="sp" />
      <el-radio-group v-model="mode"><el-radio-button value="table"><el-icon><List /></el-icon> {{ t('表格') }}</el-radio-button><el-radio-button value="card"><el-icon><Grid /></el-icon> {{ t('卡片') }}</el-radio-button></el-radio-group>
      <el-button v-if="me.role === 'admin'" :icon="Download" @click="router.push('/export')">{{ t('导出…') }}</el-button>
    </div>

    <div class="card bar">
      <el-input v-model="q" :placeholder="t('地址 / 业主 / 电话')" :prefix-icon="Search" clearable style="width:240px" />
      <el-select v-model="suburbs" multiple collapse-tags :placeholder="t('区域')" style="width:200px" clearable><el-option v-for="s in SUBURBS" :key="s" :label="s" :value="s" /></el-select>
      <el-select v-model="statuses" multiple collapse-tags :placeholder="t('状态')" style="width:180px" clearable><el-option v-for="(s, k) in STATUS" :key="k" :label="t(s.label)" :value="k"><span class="dot" :style="{ background: s.color, marginRight: '6px' }" />{{ t(s.label) }}</el-option></el-select>
      <el-select v-model="type" :placeholder="t('户型')" style="width:140px" clearable><el-option :label="t('独立屋')" value="House" /><el-option :label="t('联排')" value="Townhouse" /><el-option :label="t('公寓')" value="Unit" /></el-select>
      <el-select v-model="held" :placeholder="t('持有年限')" style="width:155px" clearable><el-option :label="t('10 年以上')" :value="10" /><el-option :label="t('15 年以上')" :value="15" /><el-option :label="t('20 年以上')" :value="20" /></el-select>
      <el-checkbox v-model="opp" border>{{ t('潜在卖家机会') }}</el-checkbox>
      <el-checkbox v-model="hideDnk" border>{{ t('隐藏勿上门') }}</el-checkbox>
      <div class="grow" />
      <el-button link v-if="active" @click="reset">{{ t('清除筛选') }}</el-button>
    </div>

    <!-- 批量操作条 -->
    <transition name="el-zoom-in-top">
      <div v-if="sel.length" class="bulk">
        <b>{{ t('已选 {n} 处', { n: sel.length }) }}</b>
        <el-button size="small" :icon="Guide" @click="bulk('route')">{{ t('加入扫街路线') }}</el-button>
        <el-button size="small" :icon="PriceTag" @click="bulk('tag')">{{ t('添加标签') }}</el-button>
        <el-button size="small" :icon="Calendar" @click="bulk('task')">{{ t('批量建跟进') }}</el-button>
        <div class="grow" />
        <el-button size="small" link @click="tbl?.clearSelection()">{{ t('取消选择') }}</el-button>
      </div>
    </transition>

    <div v-if="mode === 'table'" class="card">
      <el-table ref="tbl" :data="paged" row-key="id" @selection-change="s => sel = s" @row-click="r => openProp(r.id)" :default-sort="{ prop: 'addr' }" @sort-change="onSort" row-class-name="clk" height="calc(100vh - 330px)">
        <el-table-column type="selection" width="40" />
        <el-table-column :label="t('地址')" prop="addr" min-width="190" sortable="custom">
          <template #default="{ row }"><div class="row"><span class="dot" :style="{ background: STATUS[viewStatus(row)].color }" /><div><b>{{ row.addr }}</b><div class="small muted">{{ row.suburb }}</div></div></div></template>
        </el-table-column>
        <el-table-column :label="t('状态')" width="110"><template #default="{ row }"><StatusPill :status="viewStatus(row)" short /></template></el-table-column>
        <el-table-column :label="t('户型')" width="155"><template #default="{ row }"><span class="small">{{ t({ House: '独立屋', Townhouse: '联排', Unit: '公寓' }[row.type]) }} · {{ t('{b}卧{ba}卫', { b: row.beds, ba: row.baths }) }}</span></template></el-table-column>
        <el-table-column :label="t('Rex 联系人')" min-width="150"><template #default="{ row }"><span v-if="viewStatus(row) === 'green'">{{ row.contacts[0]?.name }} <span class="small muted">{{ t(row.contacts[0]?.role) }}</span></span><span v-else-if="viewStatus(row) === 'other'" class="small muted"><el-icon><Lock /></el-icon> {{ t('其他团队') }}</span><span v-else class="muted">—</span></template></el-table-column>
        <el-table-column :label="t('第三方业主')" min-width="140"><template #default="{ row }"><span v-if="row.tp" class="tpn">{{ row.tp.owner }}</span><span v-else class="muted">—</span></template></el-table-column>
        <el-table-column :label="t('持有')" prop="held" width="86" sortable="custom" align="right"><template #default="{ row }"><span v-if="row.tp" :class="{ hl: yearsHeld(row) >= 10 }">{{ t('持有 {n} 年', { n: yearsHeld(row) }) }}</span></template></el-table-column>
        <el-table-column :label="t('估价')" prop="est" width="110" sortable="custom" align="right"><template #default="{ row }">{{ money(row.estLow * 1000) }}+</template></el-table-column>
        <el-table-column :label="t('最近拜访')" prop="visit" width="160" sortable="custom"><template #default="{ row }"><div v-if="row.lastVisit"><div class="small">{{ t(row.lastResult) }}</div><div class="small muted">{{ relTime(row.lastVisit) }}</div></div><span v-else class="muted small">{{ t('从未') }}</span></template></el-table-column>
        <el-table-column :label="t('标记')" width="120"><template #default="{ row }"><el-tag v-if="row.flags.dnk" size="small" type="danger">{{ t('勿上门') }}</el-tag> <el-tag v-for="tag in row.tags" :key="tag" size="small" type="warning" effect="plain">{{ t(tag) }}</el-tag></template></el-table-column>
        <el-table-column width="96" fixed="right"><template #default="{ row }">
          <el-tooltip :content="t('地图上查看')"><el-button circle size="small" :icon="MapLocation" @click.stop="router.push('/map?p=' + row.id)" /></el-tooltip>
          <el-tooltip :content="t('语音录入')"><el-button circle size="small" :icon="Microphone" :disabled="row.flags.dnk" @click.stop="openCapture(row.id)" /></el-tooltip>
        </template></el-table-column>
      </el-table>
      <div class="pg"><span class="small muted">{{ t('共 {n} 处', { n: rows.length }) }}</span><el-pagination v-model:current-page="page" v-model:page-size="size" :page-sizes="[20, 50, 100]" layout="sizes, prev, pager, next" :total="rows.length" background size="small" /></div>
    </div>

    <div v-else class="cards">
      <div v-for="p in paged" :key="p.id" class="pc" @click="openProp(p.id)">
        <div class="img" :style="{ background: `linear-gradient(135deg, hsl(${p.id * 47 % 360} 35% 45%), hsl(${(p.id * 47 + 40) % 360} 40% 28%))` }">
          <StatusPill :status="viewStatus(p)" short class="sp1" />
          <el-tag v-if="p.flags.dnk" size="small" type="danger" effect="dark" class="sp2">{{ t('勿上门') }}</el-tag>
          <div class="price">{{ money(p.estLow * 1000) }} – {{ money(p.estHigh * 1000) }}</div>
        </div>
        <div class="pb">
          <div class="serif pa">{{ p.addr }}</div>
          <div class="small muted">{{ p.suburb }} QLD</div>
          <div class="specs"><span><el-icon><House /></el-icon> {{ p.beds }}</span><span><el-icon><Watermelon /></el-icon> {{ p.baths }}</span><span><el-icon><Van /></el-icon> {{ p.cars }}</span><span v-if="p.land">{{ p.land }} m²</span></div>
          <div class="small foot">{{ viewStatus(p) === 'green' ? '👤 ' + p.contacts[0]?.name : p.tp ? '📋 ' + p.tp.owner + ' · ' + t('持有 {n} 年', { n: yearsHeld(p) }) : '—' }}</div>
        </div>
      </div>
      <div class="pg" style="grid-column:1/-1"><el-pagination v-model:current-page="page" v-model:page-size="size" layout="prev, pager, next" :total="rows.length" background /></div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search, Download, Guide, PriceTag, Calendar, MapLocation, Microphone } from '@element-plus/icons-vue'
import { state, me, STATUS, SUBURBS, viewStatus, yearsHeld, relTime, money, addTask, now, DAY, audit } from '../data/store'
import { opportunities } from '../data/metrics'
import { openProp, openCapture } from '../data/ui'
import StatusPill from '../components/StatusPill.vue'
import { t, tx, locale } from '../i18n'

const router = useRouter(), route = useRoute()
const mode = ref('table'), q = ref(''), suburbs = ref([]), statuses = ref([]), type = ref(''), held = ref(null), opp = ref(!!route.query.opp), hideDnk = ref(false)
const page = ref(1), size = ref(20), sel = ref([]), tbl = ref(), sortBy = ref({ prop: 'addr', order: 'ascending' })
const active = computed(() => q.value || suburbs.value.length || statuses.value.length || type.value || held.value || opp.value || hideDnk.value)
function reset() { q.value = ''; suburbs.value = []; statuses.value = []; type.value = ''; held.value = null; opp.value = false; hideDnk.value = false }
const oppIds = computed(() => new Set(opportunities(999).map(o => o.p.id)))
const rows = computed(() => {
  const s = q.value.toLowerCase().replace(/\s/g, '')
  let L = state.props.filter(p =>
    (!s || (p.addr + p.suburb).toLowerCase().replace(/\s/g, '').includes(s) || p.contacts.some(c => (c.name + c.mobile).toLowerCase().replace(/\s/g, '').includes(s)) || p.tp?.owner.toLowerCase().replace(/\s/g, '').includes(s)) &&
    (!suburbs.value.length || suburbs.value.includes(p.suburb)) && (!statuses.value.length || statuses.value.includes(viewStatus(p))) &&
    (!type.value || p.type === type.value) && (!held.value || (yearsHeld(p) ?? 0) >= held.value) && (!opp.value || oppIds.value.has(p.id)) && (!hideDnk.value || !p.flags.dnk))
  const { prop, order } = sortBy.value, dir = order === 'descending' ? -1 : 1
  const key = { addr: p => p.street + String(p.num).padStart(4, '0') + (p.unit || 0), held: p => yearsHeld(p) ?? -1, est: p => p.estLow, visit: p => p.lastVisit ? +p.lastVisit : 0 }[prop] || (p => p.addr)
  return [...L].sort((a, b) => (key(a) > key(b) ? 1 : key(a) < key(b) ? -1 : 0) * dir)
})
const paged = computed(() => rows.value.slice((page.value - 1) * size.value, page.value * size.value))
watch(rows, () => { if ((page.value - 1) * size.value >= rows.value.length) page.value = 1 })
const onSort = s => { sortBy.value = s.order ? s : { prop: 'addr', order: 'ascending' } }

async function bulk(kind) {
  const n = sel.value.length
  if (kind === 'route') { ElMessage.success(t('已将 {n} 处加入扫街路线，可在地图页查看', { n })); audit('加入扫街路线', `${n} 处房产`, '来自房产列表') }
  if (kind === 'tag') {
    const { value } = await ElMessageBox.prompt(t('标签名（可选：{tags}）', { tags: state.settings.tags.map(t).join('、') }), t('批量添加标签'), { inputValue: state.settings.tags[0] }).catch(() => ({}))
    if (!value) return
    sel.value.forEach(p => !p.tags.includes(value) && p.tags.push(value)); audit('批量标签', `${n} 处房产`, value); ElMessage.success(t('已为 {n} 处添加「{tag}」', { n, tag: value }))
  }
  if (kind === 'task') {
    sel.value.forEach(p => addTask({ type: 'visit', title: '再次上门拜访', propId: p.id, contact: p.contacts[0]?.name || p.tp?.owner || '业主', due: new Date(now().getTime() + 3 * DAY) }))
    ElMessage.success(t('已创建 {n} 条跟进（3 天后）', { n }))
  }
  tbl.value?.clearSelection()
}
</script>

<style scoped>
.bar { display: flex; gap: 10px; flex-wrap: wrap; align-items: center; padding: 12px 14px; margin-bottom: 12px; }
.bar .el-checkbox { margin-right: 0; }
.bulk { display: flex; gap: 10px; align-items: center; background: var(--navy); color: #fff; padding: 10px 16px; border-radius: 12px; margin-bottom: 12px; }
.tpn { color: #92400e; background: var(--gold-soft); padding: 1px 6px; border-radius: 5px; font-size: 13px; }
.hl { color: #b45309; font-weight: 700; }
.pg { display: flex; justify-content: space-between; align-items: center; padding: 12px 16px; }
:deep(.clk) { cursor: pointer; }
.cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: 16px; }
.pc { background: #fff; border-radius: 14px; overflow: hidden; border: 1px solid var(--line); cursor: pointer; transition: .18s; }
.pc:hover { transform: translateY(-3px); box-shadow: 0 12px 28px rgba(15, 42, 68, .12); }
.img { height: 140px; position: relative; }
.sp1 { position: absolute; left: 10px; top: 10px; } .sp2 { position: absolute; right: 10px; top: 10px; }
.price { position: absolute; left: 10px; bottom: 10px; color: #fff; font-weight: 700; font-size: 15px; text-shadow: 0 1px 6px rgba(0, 0, 0, .4); }
.pb { padding: 12px 14px; }
.pa { font-size: 18px; font-weight: 700; }
.specs { display: flex; gap: 14px; margin: 8px 0; font-size: 13px; color: #475467; }
.specs span { display: flex; align-items: center; gap: 3px; }
.foot { border-top: 1px solid var(--line); padding-top: 8px; color: #475467; }
</style>
