<template>
  <div class="page">
    <div class="page-head"><div><h1>{{ t('跟进任务') }}</h1><div class="sub">{{ t('拖动卡片可改期') }}</div></div><div class="sp" />
      <label>{{ t('经纪人') }} <el-select v-model="who" style="width:170px"><el-option :label="t('所有人')" value="" /><el-option v-for="a in agents" :key="a.id" :label="a.name" :value="a.id" /></el-select></label>
      <el-checkbox v-model="showDone" border>{{ t('显示已完成') }}</el-checkbox><el-button type="primary" @click="dialog = true">{{ t('新建跟进') }}</el-button>
    </div>
    <div class="board"><div v-for="col in columns" :key="col.key" class="col" :class="{ over: dragOver === col.key }" @dragover.prevent="dragOver = col.key" @dragleave="dragOver = ''" @drop="drop(col)">
      <div class="ch"><b>{{ t(col.label) }}</b><span class="cnt">{{ col.items.length }}</span></div>
      <div v-for="task in col.items" :key="task.id" class="tk" :class="{ done: task.done }" draggable="true" @dragstart="dragged = task" @dragend="dragged = null; dragOver = ''">
        <div class="row"><el-checkbox :model-value="!!task.done" @change="toggle(task)" /><span class="small">{{ t(kinds[task.kind] || task.kind) }}</span><span class="sp" /><span class="small">{{ fmtDateTime(`${task.d}T${task.time || '09:00'}`) }}</span></div>
        <div class="ti">{{ task.t }}</div><div class="small muted row"><Avatar :name="task.who" :size="20" /> {{ task.who }} · {{ task.propertyAddress || '—' }}</div>
        <el-dropdown trigger="click" @command="days => snooze(task, days)"><el-button text size="small">{{ t('推迟') }}</el-button><template #dropdown><el-dropdown-menu><el-dropdown-item :command="1">{{ t('明天') }}</el-dropdown-item><el-dropdown-item :command="7">{{ t('下周') }}</el-dropdown-item></el-dropdown-menu></template></el-dropdown>
      </div><div v-if="!col.items.length" class="empty small muted">{{ t('拖到这里') }}</div>
    </div></div>
    <el-dialog v-model="dialog" :title="t('新建跟进')" width="500px"><el-form :model="form" label-width="95px">
      <el-form-item :label="t('房产')"><el-select v-model="form.propId" filterable remote :remote-method="search" style="width:100%" :placeholder="t('输入地址搜索')" @change="selectProperty"><el-option v-for="p in properties" :key="p.id" :label="`${p.address}, ${p.suburb}`" :value="p.id" /></el-select></el-form-item>
      <el-form-item :label="t('联系人（可选）')"><el-select v-model="form.cid" clearable filterable remote :remote-method="searchContacts" style="width:100%" :placeholder="t('搜索联系人')"><el-option v-for="c in contacts" :key="c.id" :label="`${c.first} ${c.last} · ${c.mobile || ''}`" :value="c.id" /></el-select></el-form-item>
      <el-form-item :label="t('类型')"><el-select v-model="form.kind" style="width:100%"><el-option v-for="(label, kind) in kinds" :key="kind" :label="t(label)" :value="kind" /></el-select></el-form-item>
      <el-form-item :label="t('内容')"><el-input v-model="form.t" /></el-form-item>
      <el-form-item :label="t('日期')"><el-date-picker v-model="form.d" type="date" value-format="YYYY-MM-DD" style="width:100%" /></el-form-item>
      <el-form-item :label="t('时间（可选）')"><el-time-picker v-model="form.time" format="HH:mm" value-format="HH:mm" style="width:100%" /></el-form-item>
      <el-form-item :label="t('经纪人')"><el-select v-model="form.assigneeId" style="width:100%"><el-option v-for="a in agents" :key="a.id" :label="a.name" :value="a.id" /></el-select></el-form-item>
    </el-form><template #footer><el-button @click="dialog = false">{{ t('取消') }}</el-button><el-button type="primary" :disabled="!form.propId || !form.t || !form.d || !form.assigneeId" @click="create">{{ t('创建') }}</el-button></template></el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue';
import { ElMessage } from 'element-plus';
import { api } from '../api';
import { fmtDateTime } from '../admin-format';
import { t } from '../i18n';
import Avatar from '../components/Avatar.vue';
const kinds = { call: '电话', hot: '紧急', appt: '预约', appraisal: '估价', visit: '上门', report: '报告' };
const todos = ref([]), agents = ref([]), properties = ref([]), contacts = ref([]);
const who = ref(''), showDone = ref(false), dialog = ref(false), dragged = ref(null), dragOver = ref('');
const form = reactive({ propId: '', cid: '', kind: 'call', t: '', d: '', time: '', assigneeId: '' });
const today = () => new Date().toLocaleDateString('en-CA');
const visible = computed(() => todos.value.filter(x => (!who.value || x.assigneeId === who.value) && (showDone.value || !x.done)));
const columns = computed(() => {
  const base = new Date(`${today()}T00:00:00`).getTime();
  const groups = [
    { key: 'late', label: '已逾期', offset: null, items: [] }, { key: 'today', label: '今天', offset: 0, items: [] },
    { key: 'tmr', label: '明天', offset: 1, items: [] }, { key: 'week', label: '本周', offset: 3, items: [] },
    { key: 'later', label: '以后', offset: 8, items: [] },
  ];
  visible.value.forEach(task => {
    const n = Math.floor((new Date(`${task.d}T00:00:00`).getTime() - base) / 86400000);
    groups[n < 0 && !task.done ? 0 : n <= 0 ? 1 : n === 1 ? 2 : n < 7 ? 3 : 4].items.push(task);
  });
  return groups;
});
async function load() {
  try { [todos.value, agents.value] = await Promise.all([api('admin/todos'), api('admin/users')]); agents.value = agents.value.filter(a => a.role === 'agent' && !a.disabled); }
  catch (error) { ElMessage.error(error.message); }
}
async function search(query) { if (query?.trim()) { try { properties.value = await api(`admin/properties?q=${encodeURIComponent(query)}&limit=30`); } catch (error) { ElMessage.error(error.message); } } }
async function searchContacts(query) { if (query?.trim()) { try { contacts.value = await api(`admin/contacts?q=${encodeURIComponent(query)}`); } catch (error) { ElMessage.error(error.message); } } }
function selectProperty(id) { const covering = properties.value.find(p => p.id === id)?.coveringAgents || []; if (covering.length === 1) form.assigneeId = covering[0].id; }
async function patch(task, change) {
  try { await api(`admin/todos/${encodeURIComponent(task.id)}`, { method: 'PATCH', body: JSON.stringify({ ...change, version: task.version }) }); await load(); }
  catch (error) { ElMessage.error(error.message); await load(); }
}
function toggle(task) { patch(task, { done: !task.done, doneAt: task.done ? '' : new Date().toISOString() }); }
function snooze(task, days) { const date = new Date(`${task.d}T12:00:00`); date.setDate(date.getDate() + days); patch(task, { d: date.toLocaleDateString('en-CA') }); }
function drop(col) { dragOver.value = ''; if (!dragged.value || col.offset === null) return; const date = new Date(`${today()}T12:00:00`); date.setDate(date.getDate() + col.offset); patch(dragged.value, { d: date.toLocaleDateString('en-CA') }); dragged.value = null; }
async function create() {
  try { await api('admin/todos', { method: 'POST', body: JSON.stringify({ ...form }) }); dialog.value = false; Object.assign(form, { propId: '', cid: '', kind: 'call', t: '', d: '', time: '', assigneeId: '' }); await load(); }
  catch (error) { ElMessage.error(error.message); }
}
onMounted(load);
</script>

<style scoped>
.board { display: grid; grid-template-columns: repeat(5, minmax(180px, 1fr)); gap: 14px; align-items: start; overflow-x: auto; }
.col { background: #eceae4; border-radius: 14px; padding: 10px; min-height: 300px; }
.col.over { background: #e6dcc0; box-shadow: inset 0 0 0 2px var(--gold); }
.ch, .row { display: flex; align-items: center; gap: 8px; }
.ch { padding: 4px 4px 10px; }.cnt { margin-left: auto; background: white; border-radius: 99px; padding: 0 8px; }
.tk { background: white; border-radius: 11px; padding: 12px; margin-bottom: 8px; cursor: grab; }
.tk.done { opacity: .55; }.tk.done .ti { text-decoration: line-through; }.ti { font-weight: 650; margin: 8px 0; }
.empty { text-align: center; padding: 24px 0; border: 1px dashed #d6d3c9; border-radius: 10px; }
</style>
