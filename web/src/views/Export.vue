<template>
  <div class="page">
    <div class="page-head"><div><h1>{{ t('数据导出') }}</h1><div class="sub">{{ t('仅管理员 · 只能导出 Rex 字段，第三方数据（业主名、成交日期）不可导出 · 每次导出留痕') }}</div></div></div>
    <div class="ex">
      <div class="card">
        <div class="card-b">
          <el-steps :active="step" simple style="margin-bottom:18px"><el-step :title="t('选择范围')" /><el-step :title="t('选择字段')" /><el-step :title="t('确认导出')" /></el-steps>
          <div v-show="step === 0">
            <el-form label-width="90px">
              <el-form-item :label="t('区域')"><el-checkbox-group v-model="f.suburbs"><el-checkbox-button v-for="s in SUBURBS" :key="s" :value="s">{{ s }}</el-checkbox-button></el-checkbox-group></el-form-item>
              <el-form-item :label="t('状态')"><el-checkbox-group v-model="f.status"><el-checkbox v-for="(s, k) in STATUS" :key="k" :value="k">{{ t(s.label) }}</el-checkbox></el-checkbox-group></el-form-item>
              <el-form-item :label="t('排除')"><el-checkbox v-model="f.noDnk">{{ t('勿上门') }}</el-checkbox><el-checkbox v-model="f.noDnc">{{ t('勿致电') }}</el-checkbox></el-form-item>
            </el-form>
          </div>
          <div v-show="step === 1">
            <div class="fields">
              <label v-for="c in FIELDS" :key="c.k" class="fd" :class="{ dis: c.tp, on: f.fields.includes(c.k) }">
                <el-checkbox :model-value="f.fields.includes(c.k)" :disabled="c.tp" @change="v => v ? f.fields.push(c.k) : f.fields.splice(f.fields.indexOf(c.k), 1)" />
                <div><b>{{ t(c.l) }}</b><div class="small muted">{{ t(c.tp ? '第三方数据 · 授权不允许导出' : 'Rex 字段') }}</div></div>
                <el-icon v-if="c.tp" color="#dc2626" style="margin-left:auto"><Lock /></el-icon>
              </label>
            </div>
          </div>
          <div v-show="step === 2">
            <el-descriptions :column="1" border>
              <el-descriptions-item :label="t('范围')">{{ f.suburbs.join(locale === 'en' ? ', ' : '、') || t('全部区域') }} · {{ f.status.map(s => t(STATUS[s].short)).join('/') || t('全部状态') }}</el-descriptions-item>
              <el-descriptions-item :label="t('行数')"><b>{{ preview.length }}</b> {{ t('行') }}</el-descriptions-item>
              <el-descriptions-item :label="t('字段')">{{ f.fields.map(k => t(FIELDS.find(x => x.k === k).l)).join(locale === 'en' ? ', ' : '、') }}</el-descriptions-item>
            </el-descriptions>
            <el-input v-model="f.reason" :placeholder="t('导出用途（必填，会写入审计日志）')" style="margin-top:14px" />
            <el-checkbox v-model="f.ack" style="margin-top:10px">{{ t('我确认导出文件仅在办公室内部使用，不外传') }}</el-checkbox>
          </div>
          <div class="nav">
            <el-button v-if="step" @click="step--">{{ t('上一步') }}</el-button><div class="grow" />
            <el-button v-if="step < 2" type="primary" :disabled="step === 1 && !f.fields.length" @click="step++">{{ t('下一步') }}</el-button>
            <el-button v-else class="gold-btn" :disabled="!f.reason || !f.ack" :icon="Download" @click="go">{{ t('导出 Excel') }}</el-button>
          </div>
        </div>
      </div>
      <div class="card">
        <div class="card-h"><h3>{{ t('预览') }}</h3><span class="small muted">{{ t('{n} 行，显示前 6 行', { n: preview.length }) }}</span></div>
        <div class="card-b"><el-table :data="preview.slice(0, 6)" size="small" border><el-table-column v-for="k in f.fields" :key="k" :label="t(FIELDS.find(x => x.k === k).l)" :prop="k" show-overflow-tooltip /></el-table></div>
        <div class="card-h"><h3>{{ t('导出记录') }}</h3></div>
        <div class="card-b">
          <div v-for="e in state.exports" :key="e.at" class="er"><el-icon><Document /></el-icon><div class="grow"><b>{{ tx(e.scope) }}</b><div class="small muted">{{ e.who }} · {{ fmtDate(e.at) }} · {{ t('{n} 个字段', { n: e.fields }) }} · {{ tx(e.reason) }}</div></div></div>
        </div>
      </div>
    </div>
  </div>
</template>
<script setup>
import { ref, reactive, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { Download } from '@element-plus/icons-vue'
import { state, STATUS, SUBURBS, viewStatus, fmtDate, now, audit } from '../data/store'
import { t, tx, locale } from '../i18n'
const step = ref(0)
const FIELDS = [{ k: 'addr', l: '地址' }, { k: 'suburb', l: '区域' }, { k: 'status', l: '状态' }, { k: 'contact', l: 'Rex 联系人' }, { k: 'role', l: '关系' }, { k: 'mobile', l: '手机' }, { k: 'lastVisit', l: '最近拜访' }, { k: 'lastResult', l: '拜访结果' }, { k: 'tpOwner', l: '第三方业主名', tp: true }, { k: 'tpSale', l: '最近成交日期', tp: true }]
const f = reactive({ suburbs: ['Sunnybank'], status: ['red'], noDnk: true, noDnc: true, fields: ['addr', 'suburb', 'status', 'lastVisit', 'lastResult'], reason: '', ack: false })
const preview = computed(() => state.props.filter(p => (!f.suburbs.length || f.suburbs.includes(p.suburb)) && (!f.status.length || f.status.includes(viewStatus(p))) && !(f.noDnk && p.flags.dnk) && !(f.noDnc && p.flags.dnc))
  .map(p => ({ addr: p.addr, suburb: p.suburb, status: t(STATUS[viewStatus(p)].short), contact: p.contacts[0]?.name || '', role: t(p.contacts[0]?.role || ''), mobile: p.contacts[0]?.mobile || '', lastVisit: p.lastVisit ? fmtDate(p.lastVisit) : '', lastResult: t(p.lastResult || '') })))
function go() {
  const cols = f.fields, head = cols.map(k => t(FIELDS.find(x => x.k === k).l))
  const csv = '\ufeff' + [head, ...preview.value.map(r => cols.map(k => `"${String(r[k]).replace(/"/g, '""')}"`))].map(r => r.join(',')).join('\n')
  const a = document.createElement('a'); a.href = URL.createObjectURL(new Blob([csv], { type: 'text/csv' })); a.download = `export-${Date.now()}.csv`; a.click()
  const scope = `${f.suburbs.join('/') || '全部'} · ${f.status.map(s => STATUS[s].short).join('/') || '全部'} · ${preview.value.length} 行`
  state.exports.unshift({ at: now(), who: 'Ping Han', scope, fields: cols.length, reason: f.reason })
  audit('导出', scope, `${cols.length} 个字段 · ${f.reason}`)
  ElMessage.success(t('已导出并记录')); step.value = 0; f.reason = ''; f.ack = false
}
</script>
<style scoped>
.ex { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; align-items: start; }
.fields { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
.fd { display: flex; gap: 10px; align-items: center; border: 1.5px solid var(--line); border-radius: 10px; padding: 10px 12px; cursor: pointer; }
.fd.on { border-color: var(--navy); background: #f2f5f9; }
.fd.dis { background: #fafafa; cursor: not-allowed; opacity: .7; }
.nav { display: flex; margin-top: 18px; }
.er { display: flex; gap: 10px; align-items: center; padding: 8px 0; border-bottom: 1px solid #f3f4f6; }
</style>
