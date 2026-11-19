<template>
  <div class="page">
    <div class="page-head"><div><h1>{{ t('审核队列') }}</h1><div class="sub">{{ t('快捷键') }} <span class="kbd">A</span> {{ t('通过') }} <span class="kbd">S</span> {{ t('跳过') }}</div></div></div>
    <el-tabs v-model="tab">
      <el-tab-pane v-for="entry in tabs" :key="entry.id" :name="entry.id" :label="`${t(entry.label)} (${entry.count})`" />
    </el-tabs>
    <div v-if="tab === 'approvals'" class="card card-b">
      <label>{{ t('状态') }} <el-select v-model="status" @change="load" style="width:170px"><el-option :label="t('全部')" value="" /><el-option v-for="s in ['pending','approved','rejected']" :key="s" :label="t(statusNames[s])" :value="s" /></el-select></label>
      <div v-for="item in approvals" :key="item.id" class="ri"><b>{{ item.summary || item.id }}</b><span class="muted">{{ t(item.kind === 'merge' ? '合并记录' : '重新分配') }} · <StatusPill :status="item.status" short /></span><el-input v-if="item.status === 'pending'" v-model="notes[item.id]" :placeholder="t('备注')" style="width:180px" /><el-button v-if="item.status === 'pending'" @click="decide(item, 'approve')">{{ t('通过') }}</el-button><el-button v-if="item.status === 'pending'" @click="decide(item, 'reject')">{{ t('拒绝') }}</el-button></div>
    </div>
    <div v-else class="rv">
      <div class="rlist card"><div v-for="item in items" :key="item.id" class="ri" :class="{ on: current?.id === item.id }" @click="selected = item.id"><div class="grow"><b>{{ tab === 'address' ? item.address : item.text }}</b><div class="small muted">{{ tab === 'address' ? item.suburb : item.agentName }} · {{ relTime(item.updated_at || item.date) }}</div></div></div><el-empty v-if="!items.length" :description="t('这一类没有待审核项目')" /></div>
      <div v-if="current" class="card rd"><div class="card-h"><h3>{{ tab === 'address' ? t('地址匹配') : t('AI 采集') }}</h3><div class="sp" /><span class="small muted">{{ relTime(current.updated_at || current.date) }}</span></div><div class="card-b">
        <template v-if="tab === 'address'"><div class="big">{{ current.address }}</div><p>{{ current.lotPlan }} · {{ current.suburb }} · {{ t(marketKinds[current.kind] || current.kind) }} · {{ money(current.price) }} · {{ fmtDate(current.date) }}</p><h3>{{ t('候选地块') }}</h3><div v-for="p in candidates" :key="p.ID" class="ri" :class="{ on: chosen === p.ID }" @click="chosen = p.ID"><div class="grow"><b>{{ p.Address }}, {{ p.Suburb }}</b><div class="small muted">{{ p.LotPlan }} · <StatusPill :status="p.Registered ? 'registered' : 'unregistered'" short /> · {{ t('业主') }}: {{ p.OwnerNames || '—' }}</div></div><el-button @click="link(p.ID)">{{ t('关联到此地块') }}</el-button></div></template>
        <template v-else><p>{{ t('经纪人') }}: {{ current.agentName || '—' }} · {{ t('房产') }}: {{ property?.address || '—' }}</p><div class="tr">{{ current.text }}</div><h3>{{ t('提取字段') }}</h3><div v-for="(value, key) in current.extracted || {}" :key="key" class="field" :class="{ unsure: key === 'unsure' || current.extracted?.unsure?.includes?.(key) || (typeof current.extracted?.confidence?.[key] === 'number' && current.extracted.confidence[key] < 0.75) }">{{ key }}: {{ typeof value === 'object' ? JSON.stringify(value) : value }}</div></template>
      </div><div class="acts"><el-button @click="skip">{{ t('跳过') }} (S)</el-button><el-button v-if="tab === 'address'" @click="ignore">{{ t('不是同一地址') }}</el-button><div class="sp" /><el-button v-if="tab === 'ai'" type="primary" @click="remind">{{ t('提醒经纪人') }} (A)</el-button></div></div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue';
import { ElMessage } from 'element-plus';
import { api } from '../api';
import { fmtDate, relTime, money } from '../admin-format';
import { t } from '../i18n';
import StatusPill from '../components/StatusPill.vue';

const tab = ref('address');
const address = ref([]);
const captures = ref([]);
const approvals = ref([]);
const candidates = ref([]);
const chosen = ref('');
const selected = ref('');
const status = ref('pending');
const notes = ref({});
const property = ref(null);
const statusNames = { pending: '待审核', approved: '已批准', rejected: '已拒绝' };
const marketKinds = { sale: '成交', listing: '挂牌', withdrawn: '撤牌' };
const tabs = computed(() => [
  { id: 'address', label: '地址匹配', count: address.value.length },
  { id: 'ai', label: 'AI 采集', count: captures.value.length },
  { id: 'approvals', label: '审批', count: approvals.value.filter(x => x.status === 'pending').length },
]);
const items = computed(() => tab.value === 'address' ? address.value : captures.value);
const current = computed(() => items.value.find(x => x.id === selected.value) || items.value[0]);
async function load() {
  try {
    [address.value, captures.value, approvals.value] = await Promise.all([
      api('admin/market-review'), api('admin/captures'), api(`admin/approvals?status=${status.value}`),
    ]);
  } catch (error) { ElMessage.error(error.message); }
}
watch(current, async item => {
  candidates.value = [];
  chosen.value = '';
  property.value = null;
  if (!item) return;
  try {
    if (tab.value === 'address') {
      candidates.value = await api(`admin/market-review/${encodeURIComponent(item.id)}/candidates`);
      if (candidates.value.length === 1) chosen.value = candidates.value[0].ID;
    }
    else if (item.propId) property.value = (await api(`admin/properties?q=${encodeURIComponent(item.propId)}`))[0];
  } catch (error) { ElMessage.error(error.message); }
});
async function link(id) {
  if (!current.value) return;
  try { await api(`admin/market-review/${encodeURIComponent(current.value.id)}/link`, { method: 'POST', body: JSON.stringify({ propertyId: id }) }); await load(); }
  catch (error) { ElMessage.error(error.message); }
}
async function ignore() {
  if (!current.value) return;
  try { await api(`admin/market-review/${encodeURIComponent(current.value.id)}/ignore`, { method: 'POST', body: '{}' }); await load(); }
  catch (error) { ElMessage.error(error.message); }
}
async function remind() {
  const item = current.value;
  if (!item?.ownerId) return ElMessage.warning(t('没有经纪人'));
  try {
    await api('admin/todos', { method: 'POST', body: JSON.stringify({
      id: `capture-remind-${item.id}`, assigneeId: item.ownerId, d: new Date().toLocaleDateString('en-CA'),
      t: `Review voice capture · ${property.value?.address || item.text.slice(0, 48)}`, kind: 'call', propId: item.propId,
    }) });
    ElMessage.success(t('已创建')); skip();
  } catch (error) { ElMessage.error(error.message); }
}
async function decide(item, decision) {
  try { await api(`approvals/${encodeURIComponent(item.id)}/decide`, { method: 'POST', body: JSON.stringify({ decision, note: notes.value[item.id] || '' }) }); await load(); }
  catch (error) { ElMessage.error(error.message); }
}
function skip() {
  const index = items.value.findIndex(x => x.id === current.value?.id);
  selected.value = items.value[(index + 1) % items.value.length]?.id || '';
}
function key(event) {
  if (['INPUT', 'TEXTAREA'].includes(event.target.tagName)) return;
  if (event.key.toLowerCase() === 's') skip();
  if (event.key.toLowerCase() === 'a' && tab.value === 'ai') remind();
  if (event.key.toLowerCase() === 'a' && tab.value === 'address' && chosen.value) link(chosen.value);
}
onMounted(() => { load(); window.addEventListener('keydown', key); });
onBeforeUnmount(() => window.removeEventListener('keydown', key));
</script>

<style scoped>
.rv { display: grid; grid-template-columns: 340px minmax(0, 1fr); gap: 16px; align-items: start; }
.rlist { padding: 6px; max-height: calc(100vh - 260px); overflow: auto; }
.ri { display: flex; gap: 12px; align-items: center; padding: 12px; border-bottom: 1px solid var(--line); cursor: pointer; }
.ri.on { background: var(--gold-soft); box-shadow: inset 3px 0 0 var(--gold); }
.big { font-size: 20px; font-weight: 700; }
.tr { padding: 14px; background: #f7f8fa; line-height: 1.7; }
.field { padding: 6px 10px; }
.unsure { background: #fef9c3; }
.acts { display: flex; gap: 8px; padding: 12px 18px 18px; }
</style>
