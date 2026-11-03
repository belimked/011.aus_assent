<template>
  <div class="page">
    <div class="page-head">
      <div><h1>{{ t('跟进任务') }}</h1><div class="sub">{{ t('来自语音录入、OFI 登记和手动创建的提醒 · 拖动卡片可改期') }}</div></div>
      <div class="sp" />
      <el-select v-if="me.role !== 'agent'" v-model="who" style="width:180px" :placeholder="t('所有人')"><el-option :label="t('所有人')" value="" /><el-option v-for="u in agentsList" :key="u.id" :label="u.name" :value="u.id" /></el-select>
      <el-checkbox v-model="showDone" border>{{ t('显示已完成') }}</el-checkbox>
      <el-button type="primary" :icon="Plus" @click="dlg = true">{{ t('新建跟进') }}</el-button>
    </div>

    <div class="board">
      <div v-for="c in cols" :key="c.k" class="col" :class="{ over: dragOver === c.k }" @dragover.prevent="dragOver = c.k" @dragleave="dragOver = null" @drop="drop(c)">
        <div class="ch"><span class="dot" :style="{ background: c.color }" /><b>{{ t(c.l) }}</b><span class="cnt">{{ c.items.length }}</span></div>
        <TransitionGroup name="fade-up" tag="div" class="items">
          <div v-for="task in c.items" :key="task.id" class="tk" :class="{ done: task.done }" draggable="true" @dragstart="drag = task" @dragend="drag = null; dragOver = null">
            <div class="row">
              <el-checkbox :model-value="task.done" @change="toggle(task)" />
              <span class="tt" :class="'t-' + task.type">{{ t(TYPE[task.type]) }}</span>
              <div class="grow" />
              <span class="small" :class="{ late: !task.done && task.due < now() }">{{ c.k === 'today' || c.k === 'late' ? fmtTime(task.due) : fmtMD(task.due) }}</span>
            </div>
            <div class="ti">{{ t(task.title) }}</div>
            <div class="small muted pa" @click="openProp(task.propId)"><el-icon><Location /></el-icon> {{ propById(task.propId)?.addr }}</div>
            <div class="row foot">
              <span class="small"><b>{{ task.contact }}</b></span>
              <div class="grow" />
              <el-tag size="small" effect="plain" round>{{ t(task.source) }}</el-tag>
              <Avatar :id="task.agent" :size="20" />
            </div>
            <div class="acts">
              <el-button size="small" text :icon="Phone" @click="ElMessage.success(t('拨打 {name}（演示）', { name: task.contact }))">{{ t('拨打') }}</el-button>
              <el-dropdown trigger="click" @command="d => snooze(task, d)"><el-button size="small" text :icon="Clock">{{ t('推迟') }}</el-button>
                <template #dropdown><el-dropdown-menu><el-dropdown-item :command="1/24">{{ t('1 小时后') }}</el-dropdown-item><el-dropdown-item :command="1">{{ t('明天') }}</el-dropdown-item><el-dropdown-item :command="7">{{ t('下周') }}</el-dropdown-item></el-dropdown-menu></template>
              </el-dropdown>
            </div>
          </div>
        </TransitionGroup>
        <div v-if="!c.items.length" class="empty small muted">{{ t(c.k === 'late' ? '🎉 没有逾期' : '拖到这里') }}</div>
      </div>
    </div>

    <el-dialog v-model="dlg" :title="t('新建跟进')" width="480px">
      <el-form :model="nf" label-width="80px">
        <el-form-item :label="t('房产')"><el-select v-model="nf.propId" filterable :placeholder="t('输入地址搜索')" style="width:100%"><el-option v-for="p in state.props" :key="p.id" :label="p.addr + ', ' + p.suburb" :value="p.id" /></el-select></el-form-item>
        <el-form-item :label="t('类型')"><el-radio-group v-model="nf.type"><el-radio-button v-for="(l, k) in TYPE" :key="k" :value="k">{{ t(l) }}</el-radio-button></el-radio-group></el-form-item>
        <el-form-item :label="t('内容')"><el-input v-model="nf.title" :placeholder="t('例如：回电讨论估价')" /></el-form-item>
        <el-form-item :label="t('时间')"><el-date-picker v-model="nf.due" type="datetime" format="YYYY-MM-DD HH:mm" style="width:100%" :shortcuts="shortcuts" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dlg = false">{{ t('取消') }}</el-button><el-button type="primary" :disabled="!nf.propId || !nf.title" @click="create">{{ t('创建') }}</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { Plus, Phone, Clock } from '@element-plus/icons-vue'
import { state, me, USERS, scopeAgents, propById, fmtTime, fmtMD, now, startOfDay, DAY, addTask } from '../data/store'
import { openProp } from '../data/ui'
import Avatar from '../components/Avatar.vue'
import { t, tx, locale } from '../i18n'

const TYPE = { call: '电话', appraisal: '估价', visit: '上门', report: '报告' }
const who = ref(''), showDone = ref(false), drag = ref(null), dragOver = ref(null), dlg = ref(false)
const agentsList = computed(() => USERS.filter(u => scopeAgents.value.includes(u.id)))
const mine = computed(() => state.tasks.filter(t => scopeAgents.value.includes(t.agent) && (!who.value || t.agent === who.value) && (showDone.value || !t.done)))
const cols = computed(() => {
  const s = startOfDay(now()).getTime(), d1 = s + DAY, d2 = s + 2 * DAY, w = s + 7 * DAY
  const by = f => mine.value.filter(f).sort((a, b) => a.due - b.due)
  return [
    { k: 'late', l: '已逾期', color: '#dc2626', items: by(t => !t.done && t.due < s), off: null },
    { k: 'today', l: '今天', color: '#c9a227', items: by(t => t.due >= s && t.due < d1 || (t.done && t.due < s)), off: 0 },
    { k: 'tmr', l: '明天', color: '#2563eb', items: by(t => t.due >= d1 && t.due < d2), off: 1 },
    { k: 'week', l: '本周', color: '#0e7490', items: by(t => t.due >= d2 && t.due < w), off: 3 },
    { k: 'later', l: '以后', color: '#94a3b8', items: by(t => t.due >= w), off: 8 },
  ]
})
function drop(c) {
  dragOver.value = null
  if (!drag.value || c.off === null) return
  const task = drag.value, h = task.due.getHours(), m = task.due.getMinutes()
  task.due = new Date(startOfDay(now()).getTime() + c.off * DAY + h * 36e5 + m * 6e4)
  ElMessage.success(t('「{title}」已改到 {date}', { title: t(task.title), date: fmtMD(task.due) }))
}
function toggle(task) { task.done = !task.done; if (task.done) ElMessage({ message: t('完成「{title}」👍', { title: t(task.title) }), type: 'success', grouping: true }) }
function snooze(task, d) { task.due = new Date(Math.max(now(), task.due) + d * DAY); ElMessage.info(t('已推迟到 {date} {time}', { date: fmtMD(task.due), time: fmtTime(task.due) })) }
const nf = reactive({ propId: null, type: 'call', title: '', due: new Date(now().getTime() + DAY) })
const shortcuts = [{ text: '1 小时后', value: () => new Date(now().getTime() + 36e5) }, { text: '明早 9 点', value: () => new Date(startOfDay(now()).getTime() + DAY + 9 * 36e5) }, { text: '下周', value: () => new Date(now().getTime() + 7 * DAY) }]
function create() { const p = propById(nf.propId); addTask({ ...nf, contact: p.contacts[0]?.name || p.tp?.owner || '业主' }); dlg.value = false; ElMessage.success(t('已创建')); Object.assign(nf, { propId: null, title: '' }) }
</script>

<style scoped>
.board { display: grid; grid-template-columns: repeat(5, 1fr); gap: 14px; align-items: start; }
.col { background: #eceae4; border-radius: 14px; padding: 10px; min-height: 300px; transition: .15s; }
.col.over { background: #e6dcc0; box-shadow: inset 0 0 0 2px var(--gold); }
.ch { display: flex; align-items: center; gap: 8px; padding: 4px 4px 10px; }
.cnt { margin-left: auto; background: #fff; border-radius: 99px; padding: 0 8px; font-size: 12px; font-weight: 700; }
.items { display: flex; flex-direction: column; gap: 8px; }
.tk { background: #fff; border-radius: 11px; padding: 10px 12px 4px; box-shadow: 0 1px 2px rgba(0, 0, 0, .06); cursor: grab; border: 1px solid transparent; }
.tk:hover { border-color: #c8d5e2; }
.tk.done { opacity: .55; }
.tk.done .ti { text-decoration: line-through; }
.tt { font-size: 11px; padding: 1px 7px; border-radius: 5px; font-weight: 700; }
.t-call { background: #dbeafe; color: #1d4ed8; } .t-appraisal { background: #fef3c7; color: #92400e; } .t-visit { background: #cffafe; color: #0e7490; } .t-report { background: #ede9fe; color: #6d28d9; }
.late { color: #dc2626; font-weight: 700; }
.ti { font-weight: 650; margin: 6px 0 2px; font-size: 14px; }
.pa { cursor: pointer; display: flex; align-items: center; gap: 3px; }
.pa:hover { color: var(--navy); text-decoration: underline; }
.foot { margin-top: 6px; gap: 6px; }
.acts { display: flex; border-top: 1px solid #f3f4f6; margin-top: 8px; padding-top: 2px; }
.empty { text-align: center; padding: 24px 0; border: 1.5px dashed #d6d3c9; border-radius: 10px; }
</style>
