<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h1>{{ t('跟进任务') }}</h1>
        <div class="sub">{{ t('拖动卡片可改期') }}</div>
      </div>
      <div class="sp" />
      <label class="filter-label">
        {{ t('经纪人') }}
        <el-select v-model="who" :empty-values="[null, undefined]" style="width: 170px">
          <el-option :label="t('所有人')" value="" />
          <el-option v-for="agent in agents" :key="agent.id" :label="agent.name" :value="agent.id" />
        </el-select>
      </label>
      <el-checkbox v-model="showDone" border>{{ t('显示已完成') }}</el-checkbox>
      <el-button type="primary" @click="dialog = true">{{ t('新建跟进') }}</el-button>
    </div>

    <div class="board">
      <div
        v-for="col in columns"
        :key="col.key"
        class="col"
        :class="{ over: dragOver === col.key }"
        @dragover.prevent="dragOver = col.key"
        @dragleave="dragOver = ''"
        @drop="drop(col)"
      >
        <div class="ch">
          <span class="dot" :style="{ background: col.color }" />
          <b>{{ t(col.label) }}</b>
          <span class="cnt">{{ col.items.length }}</span>
        </div>
        <div
          v-for="task in col.items"
          :key="task.id"
          class="tk"
          :class="{ done: task.done }"
          draggable="true"
          @dragstart="dragged = task"
          @dragend="dragged = null; dragOver = ''"
        >
          <div class="row card-top">
            <span class="kind" :class="`kind-${task.kind}`">
              {{ t(kinds[task.kind] || task.kind) }}
            </span>
            <span class="sp" />
            <span class="small" :class="{ late: col.key === 'late' }">
              {{ cardTime(task, col.key) }}
            </span>
          </div>
          <div class="ti">{{ task.t }}</div>
          <div v-if="task.propertyAddress" class="small muted row address">
            <el-icon><Location /></el-icon>
            {{ task.propertyAddress }}
          </div>
          <div class="row foot">
            <Avatar :name="task.who" :size="20" />
            <span class="small">{{ task.who }}</span>
            <span class="sp" />
            <el-tag v-if="source(task)" size="small" effect="plain" round>
              {{ t(source(task)) }}
            </el-tag>
          </div>
          <div class="row actions">
            <el-dropdown trigger="click" @command="days => snooze(task, days)">
              <el-button text size="small" :icon="Clock">{{ t('推迟') }}</el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item :command="1">{{ t('明天') }}</el-dropdown-item>
                  <el-dropdown-item :command="7">{{ t('下周') }}</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
            <span class="sp" />
            <el-select :model-value="task.assigneeId" size="small" style="width: 115px"
              :aria-label="t('经纪人')" @change="id => patch(task, { assigneeId: id })">
              <el-option v-for="member in agents" :key="member.id" :value="member.id" :label="member.name" />
            </el-select>
            <el-checkbox v-if="identity?.role === 'admin'" :model-value="!!task.done" @change="toggle(task)">
              {{ t(task.done ? '撤销完成' : '完成') }}
            </el-checkbox>
          </div>
        </div>
        <div v-if="!col.items.length" class="empty small muted">{{ t('拖到这里') }}</div>
      </div>
    </div>

    <el-dialog v-model="dialog" :title="t('新建跟进')" width="500px">
      <el-form :model="form" label-width="95px">
        <el-form-item :label="t('房产')">
          <el-select
            v-model="form.propId"
            filterable
            remote
            :remote-method="search"
            :placeholder="t('输入地址搜索')"
            style="width: 100%"
            @change="selectProperty"
          >
            <el-option
              v-for="parcel in properties"
              :key="parcel.id"
              :label="`${parcel.address}, ${parcel.suburb}`"
              :value="parcel.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('联系人（可选）')">
          <el-select
            v-model="form.cid"
            clearable
            filterable
            remote
            :remote-method="searchContacts"
            :placeholder="t('搜索联系人')"
            style="width: 100%"
          >
            <el-option
              v-for="contact in contacts"
              :key="contact.id"
              :label="`${contact.first} ${contact.last} · ${contact.mobile || ''}`"
              :value="contact.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('类型')">
          <el-select v-model="form.kind" style="width: 100%">
            <el-option v-for="(label, kind) in kinds" :key="kind" :label="t(label)" :value="kind" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('内容')"><el-input v-model="form.t" /></el-form-item>
        <el-form-item :label="t('日期')">
          <el-date-picker v-model="form.d" type="date" value-format="YYYY-MM-DD" style="width: 100%" />
        </el-form-item>
        <el-form-item :label="t('时间（可选）')">
          <el-time-picker v-model="form.time" format="HH:mm" value-format="HH:mm" style="width: 100%" />
        </el-form-item>
        <el-form-item :label="t('经纪人')">
          <el-select v-model="form.assigneeId" style="width: 100%">
            <el-option v-for="agent in agents" :key="agent.id" :label="agent.name" :value="agent.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialog = false">{{ t('取消') }}</el-button>
        <el-button
          type="primary"
          :disabled="!form.t || !form.d || !form.assigneeId"
          @click="create"
        >
          {{ t('创建') }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue';
import { ElMessage } from 'element-plus';
import { Clock, Location } from '@element-plus/icons-vue';
import { api, identity } from '../api';
import { locale, t } from '../i18n';
import Avatar from '../components/Avatar.vue';

const kinds = { call: '电话', hot: '紧急', appt: '预约', appraisal: '估价', visit: '上门', report: '报告' };
const todos = ref([]);
const agents = ref([]);
const adminIds = ref(new Set());
const properties = ref([]);
const contacts = ref([]);
const who = ref('');
const showDone = ref(false);
const dialog = ref(false);
const dragged = ref(null);
const dragOver = ref('');
const form = reactive({ propId: '', cid: '', kind: 'call', t: '', d: '', time: '', assigneeId: '' });
const today = () => new Date().toLocaleDateString('en-CA');
const visible = computed(() => todos.value.filter(task =>
  (!who.value || task.assigneeId === who.value) && (showDone.value || !task.done),
));
const columns = computed(() => {
  const base = new Date(`${today()}T00:00:00`).getTime();
  const groups = [
    { key: 'late', label: '已逾期', color: '#dc2626', offset: null, items: [] },
    { key: 'today', label: '今天', color: '#c9a227', offset: 0, items: [] },
    { key: 'tmr', label: '明天', color: '#2563eb', offset: 1, items: [] },
    { key: 'week', label: '本周', color: '#0e7490', offset: 3, items: [] },
    { key: 'later', label: '以后', color: '#94a3b8', offset: 8, items: [] },
  ];
  visible.value.forEach(task => {
    const days = Math.floor((new Date(`${task.d}T00:00:00`).getTime() - base) / 86400000);
    const column = days < 0 && !task.done ? 0 : days <= 0 ? 1 : days === 1 ? 2 : days < 7 ? 3 : 4;
    groups[column].items.push(task);
  });
  return groups;
});
function cardTime(task, column) {
  // Today and tomorrow only need the time; overdue cards keep their date so the delay is visible.
  if (['today', 'tmr'].includes(column)) return task.time || '';
  const date = new Date(`${task.d}T12:00:00`);
  if (Number.isNaN(date.valueOf())) return task.time || '';
  const language = locale.value === 'en' ? 'en-AU' : locale.value;
  const dateText = date.toLocaleDateString(language, {
    ...(date.getFullYear() === new Date().getFullYear() ? {} : { year: 'numeric' }),
    month: 'short', day: 'numeric',
  });
  return `${dateText}${task.time ? ` · ${task.time}` : ''}`;
}
function source(task) {
  if (task.t?.startsWith('Review voice capture')) return '语音录入';
  if (task.t?.startsWith('OFI follow-up')) return 'OFI 登记';
  if (adminIds.value.has(task.createdBy)) return '手动创建';
  return '';
}
async function load() {
  try {
    const [tasks, users] = await Promise.all([api('admin/todos'), api('admin/users')]);
    todos.value = tasks;
    agents.value = users.filter(agent => ['agent', 'leader'].includes(agent.role) && !agent.disabled);
    adminIds.value = new Set(users.filter(user => user.role === 'admin').map(user => user.id));
  } catch (error) {
    ElMessage.error(error.message);
  }
}
async function search(query) {
  if (!query?.trim()) return;
  try {
    properties.value = await api(`admin/properties?q=${encodeURIComponent(query)}&limit=30`);
  } catch (error) {
    ElMessage.error(error.message);
  }
}
async function searchContacts(query) {
  if (!query?.trim()) return;
  try {
    contacts.value = await api(`admin/contacts?q=${encodeURIComponent(query)}`);
  } catch (error) {
    ElMessage.error(error.message);
  }
}
function selectProperty(id) {
  const covering = properties.value.find(parcel => parcel.id === id)?.coveringAgents || [];
  if (covering.length === 1) form.assigneeId = covering[0].id;
}
async function patch(task, change) {
  try {
    await api(`admin/todos/${encodeURIComponent(task.id)}`, {
      method: 'PATCH', body: JSON.stringify({ ...change, version: task.version }),
    });
    await load();
  } catch (error) {
    ElMessage.error(error.message);
    await load();
  }
}
function toggle(task) {
  patch(task, { done: !task.done, doneAt: task.done ? '' : new Date().toISOString() });
}
function snooze(task, days) {
  const date = new Date(`${task.d}T12:00:00`);
  date.setDate(date.getDate() + days);
  patch(task, { d: date.toLocaleDateString('en-CA') });
}
function drop(column) {
  dragOver.value = '';
  if (!dragged.value || column.offset === null) return;
  const date = new Date(`${today()}T12:00:00`);
  date.setDate(date.getDate() + column.offset);
  patch(dragged.value, { d: date.toLocaleDateString('en-CA') });
  dragged.value = null;
}
async function create() {
  try {
    await api('admin/todos', { method: 'POST', body: JSON.stringify({ ...form }) });
    dialog.value = false;
    Object.assign(form, { propId: '', cid: '', kind: 'call', t: '', d: '', time: '', assigneeId: '' });
    await load();
  } catch (error) {
    ElMessage.error(error.message);
  }
}
onMounted(load);
</script>

<style scoped>
.filter-label { display: inline-flex; align-items: center; gap: 8px; }
.board {
  display: grid;
  grid-template-columns: repeat(5, minmax(180px, 1fr));
  gap: 14px;
  align-items: start;
  overflow-x: auto;
}
.col {
  background: #eceae4;
  border-radius: 14px;
  padding: 10px;
  min-height: 300px;
}
.col.over { background: #e6dcc0; box-shadow: inset 0 0 0 2px var(--gold); }
.ch, .row { display: flex; align-items: center; gap: 8px; }
.ch { padding: 4px 4px 10px; }
.ch .dot { width: 8px; height: 8px; border-radius: 50%; }
.cnt {
  margin-left: auto;
  background: white;
  border-radius: 99px;
  padding: 0 8px;
  font-size: 12px;
}
.tk {
  background: white;
  border-radius: 11px;
  padding: 10px 12px 4px;
  margin-bottom: 8px;
  cursor: grab;
}
.tk.done { opacity: .55; }
.tk.done .ti { text-decoration: line-through; }
.kind { font-size: 11px; font-weight: 700; border-radius: 5px; padding: 1px 7px; }
.kind-call { background: #dbeafe; color: #1d4ed8; }
.kind-hot, .kind-appraisal { background: #fef3c7; color: #92400e; }
.kind-visit { background: #cffafe; color: #0e7490; }
.kind-report { background: #ede9fe; color: #6d28d9; }
.kind-appt { background: #e0f2fe; color: #075985; }
.card-top .small { white-space: nowrap; }
.late { color: #dc2626; font-weight: 700; }
.ti { font-weight: 650; margin: 8px 0 4px; font-size: 14px; }
.address { min-height: 20px; }
.foot { margin-top: 8px; }
.actions { border-top: 1px solid #f3f4f6; margin-top: 8px; }
.empty { text-align: center; padding: 24px 0; border: 1px dashed #d6d3c9; border-radius: 10px; }
</style>
