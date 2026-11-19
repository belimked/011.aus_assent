<template>
  <div class="page"><div class="page-head"><div><h1>{{ t('开放参观') }}</h1><div class="sub">{{ t('访客名单与跟进') }}</div></div></div>
    <div class="ofi"><div class="olist"><div v-for="event in events" :key="event.key" class="oc" :class="{ on: current?.key === event.key }" @click="selected = event.key">
      <div class="grow"><div class="serif oa">{{ event.address }}</div><div class="small muted">{{ fmtDate(event.d) }} · {{ event.time || '—' }}</div><div class="small">{{ event.listing.agent }} · {{ event.attendees.length }} {{ t('人') }}</div></div>
    </div><el-empty v-if="!events.length" :description="t('没有开放参观')" /></div>
    <div v-if="current" class="detail"><div class="card hero"><div class="grow"><div class="serif ha">{{ current.address }}</div><div>{{ fmtDate(current.d) }} · {{ current.time || '—' }}</div></div><div class="stats"><b>{{ current.attendees.length }}</b><span>{{ t('访客') }}</span></div></div>
      <div class="card"><div class="card-h"><h3>{{ t('访客名单') }}</h3><div class="sp" /><label>{{ t('筛选') }} <el-select v-model="filter" style="width:150px"><el-option :label="t('全部')" value="" /><el-option :label="t('回头客')" value="returning" /><el-option :label="t('要报告')" value="report" /><el-option :label="t('潜在卖家')" value="seller" /><el-option :label="t('未跟进')" value="todo" /></el-select></label></div>
        <div class="card-b"><div v-for="attendee in visitors" :key="attendee.id" class="vi"><div class="grow"><b>{{ attendee.contactFirst }} {{ attendee.contactLast }}</b><div class="small muted">{{ attendee.contactMobile || attendee.contactId }} · {{ fmtDate(attendee.d) }}</div><div class="small"><el-tag v-if="returning(attendee)" size="small" type="success">{{ t('回头客') }}</el-tag><el-tag v-if="attendee.req?.bp" size="small">{{ t('要报告') }}</el-tag><el-tag v-if="attendee.ps" size="small" type="warning">{{ t('潜在卖家') }}</el-tag></div></div>
          <el-tag v-if="attendee.follow" type="success">{{ t('已建跟进') }}</el-tag><el-button v-else size="small" @click="follow(attendee)">{{ t('跟进') }}</el-button></div><el-empty v-if="!visitors.length" :description="t('暂无访客')" /></div>
      </div></div></div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue';
import { ElMessage } from 'element-plus';
import { api } from '../api';
import { fmtDate } from '../admin-format';
import { t } from '../i18n';
const listings = ref([]), attendees = ref([]), agents = ref([]);
const selected = ref(''), filter = ref('');
const events = computed(() => listings.value.flatMap(listing => (listing.ofi || []).map((event, index) => ({
  ...event, index, listing, key: `${listing.id}-${index}`,
  address: listing.propertyAddress || listing.propertyId,
  attendees: attendees.value.filter(a => a.listingId === listing.id && a.eventIndex === index),
}))).sort((a, b) => (b.d || '').localeCompare(a.d || '')));
const current = computed(() => events.value.find(e => e.key === selected.value) || events.value[0]);
const returning = a => attendees.value.some(other => other.contactId === a.contactId && other.id !== a.id &&
  events.value.some(e => e.listing.id === other.listingId && e.index === other.eventIndex && e.d < current.value.d));
const visitors = computed(() => (current.value?.attendees || []).filter(a => !filter.value ||
  (filter.value === 'returning' ? returning(a) : filter.value === 'report' ? !!a.req?.bp : filter.value === 'seller' ? !!a.ps : !a.follow)));
async function load() {
  try { [listings.value, attendees.value, agents.value] = await Promise.all([
    api('admin/listings'), api('admin/ofi_attendees'), api('admin/users'),
  ]); } catch (error) { ElMessage.error(error.message); }
}
async function follow(a) {
  const event = current.value;
  const owner = agents.value.find(x => x.id === event.listing.ownerId && x.role === 'agent') || agents.value.find(x => x.name === event.listing.agent && x.role === 'agent');
  if (!owner) return ElMessage.warning(t('没有经纪人'));
  const date = new Date(); date.setDate(date.getDate() + 1);
  try {
    await api('admin/todos', { method: 'POST', body: JSON.stringify({
      id: `ofi-follow-${a.id}`, assigneeId: owner.id, cid: a.contactId, propId: event.listing.propertyId, kind: 'call',
      d: date.toLocaleDateString('en-CA'), t: `OFI follow-up · ${a.contactFirst || ''} ${a.contactLast || ''} · ${event.address}`,
    }) });
    await api(`admin/ofi_attendees/${encodeURIComponent(a.id)}`, { method: 'PATCH', body: JSON.stringify({ version: a.version, follow: true }) });
    await load(); ElMessage.success(t('已建跟进'));
  } catch (error) { ElMessage.error(error.message); await load(); }
}
onMounted(load);
</script>

<style scoped>
.ofi { display: grid; grid-template-columns: 320px minmax(0, 1fr); gap: 16px; align-items: start; }
.olist { display: flex; flex-direction: column; gap: 10px; }
.oc { display: flex; background: white; border: 1px solid var(--line); border-radius: 14px; padding: 16px; cursor: pointer; }
.oc.on { border-color: var(--gold); box-shadow: 0 4px 16px rgba(201,162,39,.18); }
.oa { font-size: 17px; font-weight: 700; }.hero { background: #0f2a44; color: white; border: 0; display: flex; padding: 24px; margin-bottom: 16px; }
.ha { font-size: 28px; }.stats { display: flex; flex-direction: column; text-align: center; }.stats b { font-size: 30px; }
.vi { display: flex; align-items: center; gap: 12px; padding: 12px 0; border-bottom: 1px solid var(--line); }
</style>
