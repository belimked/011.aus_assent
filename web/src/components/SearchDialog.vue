<template>
  <el-dialog v-model="ui.search" width="620px" :show-close="false" top="10vh" class="sdlg" @opened="inp?.focus()" @closed="q = ''">
    <el-input ref="inp" v-model="q" size="large" :placeholder="t('输入门牌、街道、业主姓名或手机号…')" :prefix-icon="Search" clearable @keydown.down.prevent="move(1)" @keydown.up.prevent="move(-1)" @keydown.enter="go(list[idx])" />
    <div class="res">
      <div v-if="!q" class="hint">
        <div class="small muted" style="margin-bottom:8px">{{ t('快捷入口') }}</div>
        <div class="quick">
          <el-button v-for="s in quick" :key="s.t" :icon="s.i" @click="ui.search = false; router.push(s.to)">{{ t(s.t) }}</el-button>
        </div>
        <div class="small muted" style="margin:14px 0 6px">{{ t('试试搜索') }}</div>
        <el-tag v-for="t in ['Gager', 'Mary Chen', '0412', 'Runcorn']" :key="t" class="eg" effect="plain" @click="q = t">{{ t }}</el-tag>
      </div>
      <div v-for="(r, i) in list" :key="r.p.id" class="it" :class="{ on: i === idx }" @mouseenter="idx = i" @click="go(r)">
        <span class="dot" :style="{ background: STATUS[viewStatus(r.p)].color }" />
        <div class="grow">
          <div class="a">{{ r.p.addr }}<span class="muted small">, {{ r.p.suburb }}</span></div>
          <div class="small muted">{{ r.why }}</div>
        </div>
        <StatusPill :status="viewStatus(r.p)" short />
      </div>
      <el-empty v-if="q && !list.length" :description="t('没有找到匹配的房产')" :image-size="70" />
    </div>
    <div class="foot small muted"><span><span class="kbd">↑↓</span> {{ t('选择') }}</span><span><span class="kbd">Enter</span> {{ t('打开房产') }}</span><span><span class="kbd">Esc</span> {{ t('关闭') }}</span></div>
  </el-dialog>
</template>
<script setup>
import { ref, computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import { Search } from '@element-plus/icons-vue'
import { state, STATUS, viewStatus } from '../data/store'
import { ui, openProp } from '../data/ui'
import StatusPill from './StatusPill.vue'
import { t, tx, locale } from '../i18n'
const router = useRouter()
const q = ref(''), idx = ref(0), inp = ref()
const quick = [{ t: '今日跟进', i: 'Calendar', to: '/tasks' }, { t: '区域地图', i: 'MapLocation', to: '/map' }, { t: '开放参观', i: 'Key', to: '/ofi' }, { t: '数据驾驶舱', i: 'DataBoard', to: '/cockpit' }]
const list = computed(() => {
  const s = q.value.trim().toLowerCase(); if (!s) return []
  const digits = s.replace(/\s/g, '')
  const out = []
  for (const p of state.props) {
    let why = null
    if ((p.addr + ' ' + p.suburb).toLowerCase().includes(s)) why = `${t({ House: '独立屋', Townhouse: '联排', Unit: '公寓' }[p.type] || p.type)} · ${t('{n} 卧', { n: p.beds })} · ${p.lastVisit ? t('最近拜访 {result}', { result: t(p.lastResult) }) : t('尚未拜访')}`
    else { const c = p.contacts.find(c => c.name.toLowerCase().includes(s) || (digits.length >= 3 && c.mobile.replace(/\s/g, '').includes(digits))); if (c) why = t('Rex 联系人：{name} · {role} · {mobile}', { name: c.name, role: t(c.role), mobile: c.mobile }) }
    if (!why && p.tp && p.tp.owner.toLowerCase().includes(s)) why = t('第三方业主名：{name}（{provider}）', { name: p.tp.owner, provider: state.settings.provider })
    if (why) out.push({ p, why })
    if (out.length >= 8) break
  }
  return out
})
watch(q, () => idx.value = 0)
const move = d => { idx.value = (idx.value + d + list.value.length) % Math.max(1, list.value.length) }
function go(r) { if (!r) return; ui.search = false; openProp(r.p.id) }
</script>
<style scoped>
.res { margin-top: 12px; min-height: 120px; max-height: 420px; overflow: auto; }
.it { display: flex; align-items: center; gap: 12px; padding: 10px 12px; border-radius: 10px; cursor: pointer; }
.it.on { background: #f2f5f9; }
.a { font-weight: 600; }
.quick { display: flex; gap: 8px; flex-wrap: wrap; }
.quick .el-button { margin: 0; }
.eg { margin-right: 6px; cursor: pointer; }
.foot { display: flex; gap: 16px; border-top: 1px solid var(--line); padding-top: 10px; margin-top: 8px; }
</style>
<style>.sdlg .el-dialog__header { display: none; } .sdlg { border-radius: 16px; }</style>
