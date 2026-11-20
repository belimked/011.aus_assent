<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h1>{{ t('开放参观') }}</h1>
        <div class="sub">{{ t('访客名单与跟进') }}</div>
      </div>
      <div class="sp" />
      <label class="period-label">
        {{ t('时段') }}
        <el-segmented v-model="when" :options="periodOptions" />
      </label>
    </div>

    <div class="ofi">
      <div class="olist">
        <div
          v-for="event in opens"
          :key="event.key"
          class="oc"
          :class="{ on: current?.key === event.key }"
          @click="selected = event.key"
        >
          <div class="oimg">
            <img v-if="photoUrls[event.listing.propertyId]" :src="photoUrls[event.listing.propertyId]" alt="" />
            <el-tag size="small" :type="eventStatus(event).type" effect="dark" round>
              {{ t(eventStatus(event).label) }}
            </el-tag>
          </div>
          <div class="grow">
            <div class="serif oa">{{ event.address }}</div>
            <div class="small muted">{{ fmtDate(event.d) }} · {{ event.time || '—' }}</div>
            <div class="small row listing-agent">
              <Avatar :name="event.listing.agent" :size="18" />
              {{ event.listing.agent }}
              <span class="grow" />
              <b>{{ event.attendees.length }}</b> {{ t('人') }}
            </div>
          </div>
        </div>
        <el-empty v-if="!opens.length" :description="t('没有开放参观')" :image-size="60" />
      </div>

      <div v-if="current" class="detail">
        <div class="card hero">
          <div class="grow">
            <div class="row">
              <el-tag size="small" :type="eventStatus(current).type" effect="dark" round>
                {{ t(eventStatus(current).label) }}
              </el-tag>
              <span class="small suburb">{{ current.listing.propertySuburb }}</span>
            </div>
            <div class="serif ha">{{ current.address }}</div>
            <div class="small">{{ fmtDate(current.d) }} · {{ current.time || '—' }}</div>
            <div class="row facts">
              <span v-if="current.listing.propertyBeds">{{ current.listing.propertyBeds }} {{ t('卧') }}</span>
              <span v-if="current.listing.propertyBaths">{{ current.listing.propertyBaths }} {{ t('卫') }}</span>
              <span v-if="current.listing.propertyCars">{{ current.listing.propertyCars }} {{ t('车位') }}</span>
              <span v-if="current.listing.price" class="price">{{ current.listing.price }}</span>
            </div>
          </div>
          <div class="hstats">
            <div><b class="serif">{{ current.attendees.length }}</b><span>{{ t('访客') }}</span></div>
            <div><b class="serif">{{ current.attendees.filter(returning).length }}</b><span>{{ t('回头客') }}</span></div>
            <div><b class="serif">{{ current.attendees.filter(a => a.req?.bp).length }}</b><span>{{ t('要验房报告') }}</span></div>
            <div><b class="serif price">{{ current.attendees.filter(a => a.ps).length }}</b><span>{{ t('潜在卖家') }}</span></div>
          </div>
        </div>

        <div class="card">
          <div class="card-h">
            <h3>{{ t('访客名单') }}</h3>
            <div class="sp" />
            <label class="filter-label">
              {{ t('筛选') }}
              <el-select v-model="filter" :empty-values="[null, undefined]" style="width: 150px">
                <el-option :label="t('全部')" value="" />
                <el-option :label="t('回头客')" value="returning" />
                <el-option :label="t('要报告')" value="report" />
                <el-option :label="t('潜在卖家')" value="seller" />
                <el-option :label="t('未跟进')" value="todo" />
              </el-select>
            </label>
          </div>
          <div class="card-b">
            <div v-for="attendee in visitors" :key="attendee.id" class="vi">
              <Avatar :name="visitorName(attendee)" :size="34" />
              <div class="grow">
                <div class="row visitor-tags">
                  <b>{{ visitorName(attendee) }}</b>
                  <el-tag v-if="returning(attendee)" size="small" type="success" round>
                    {{ t('回头客') }}
                  </el-tag>
                  <el-tag v-if="attendee.req?.bp" size="small" type="info" round>
                    {{ t('要报告') }}
                  </el-tag>
                  <el-tag v-if="attendee.ps" size="small" type="warning" round>
                    {{ t('潜在卖家') }}
                  </el-tag>
                </div>
                <div class="small muted">
                  {{ attendee.contactMobile || '—' }} · {{ attendee.time || fmtDate(attendee.d) }}
                </div>
              </div>
              <el-tag v-if="attendee.follow" type="success">{{ t('已跟进') }}</el-tag>
              <el-button v-else-if="identity?.role === 'admin'" size="small" @click="follow(attendee)">{{ t('跟进') }}</el-button>
            </div>
            <el-empty v-if="!visitors.length" :description="t('暂无访客')" :image-size="60" />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue';
import { ElMessage } from 'element-plus';
import { api, token, identity } from '../api';
import { fmtDate } from '../admin-format';
import { t } from '../i18n';
import Avatar from '../components/Avatar.vue';

const listings = ref([]);
const attendees = ref([]);
const agents = ref([]);
const selected = ref('');
const filter = ref('');
const when = ref('all');
const photoUrls = ref({});
const periodOptions = computed(() => [
  { label: t('今天'), value: 'today' },
  { label: t('即将'), value: 'up' },
  { label: t('已结束'), value: 'past' },
  { label: t('全部'), value: 'all' },
]);
const events = computed(() => listings.value.flatMap(listing =>
  (listing.ofi || []).map((event, index) => ({
    ...event,
    index,
    listing,
    key: `${listing.id}-${index}`,
    address: listing.propertyAddress || listing.propertyId,
    attendees: attendees.value.filter(a => a.listingId === listing.id && a.eventIndex === index),
  })),
).sort((a, b) => (b.d || '').localeCompare(a.d || '')));
const opens = computed(() => events.value.filter(event => {
  const state = eventStatus(event).key;
  if (when.value === 'today') return event.d === new Date().toLocaleDateString('en-CA');
  if (when.value === 'up') return state === 'up';
  if (when.value === 'past') return state === 'past';
  return true;
}));
const current = computed(() => opens.value.find(event => event.key === selected.value) ||
  opens.value.find(event => event.attendees.length) || opens.value[0]);
const visitors = computed(() => (current.value?.attendees || []).filter(attendee => {
  if (filter.value === 'returning') return returning(attendee);
  if (filter.value === 'report') return !!attendee.req?.bp;
  if (filter.value === 'seller') return !!attendee.ps;
  if (filter.value === 'todo') return !attendee.follow;
  return true;
}));

function visitorName(attendee) {
  return `${attendee.contactFirst || ''} ${attendee.contactLast || ''}`.trim() || '—';
}
function eventStatus(event) {
  const [from, to] = (event.time || '').split(/[–—-]/);
  const start = new Date(`${event.d}T${from?.trim() || '00:00'}:00`);
  const end = new Date(`${event.d}T${to?.trim() || '23:59'}:00`);
  if (Date.now() < start.valueOf()) return { key: 'up', label: '即将', type: 'info' };
  if (Date.now() < end.valueOf()) return { key: 'active', label: '进行中', type: 'success' };
  return { key: 'past', label: '已结束', type: '' };
}
function returning(attendee) {
  return attendees.value.some(other => other.contactId === attendee.contactId && other.id !== attendee.id &&
    events.value.some(event => event.listing.id === other.listingId &&
      event.index === other.eventIndex && event.d < current.value.d));
}
async function loadPhotos() {
  for (const listing of listings.value) {
    const photo = listing.propertyPhotos?.findLast(item => item.uploaded);
    if (!photo || photoUrls.value[listing.propertyId]) continue;
    try {
      const response = await fetch(`/api/v1/photos/${encodeURIComponent(photo.id)}`, {
        headers: { Authorization: `Bearer ${token.value}` },
      });
      if (response.ok) photoUrls.value[listing.propertyId] = URL.createObjectURL(await response.blob());
    } catch {
      // Keep the cadastral placeholder when an image is unavailable.
    }
  }
}
async function load() {
  try {
    [listings.value, attendees.value, agents.value] = await Promise.all([
      api('admin/listings'), api('admin/ofi_attendees'), api('admin/users'),
    ]);
    if (when.value === 'all' && events.value.some(event => event.d === new Date().toLocaleDateString('en-CA'))) {
      when.value = 'today';
    }
    loadPhotos();
  } catch (error) {
    ElMessage.error(error.message);
  }
}
async function follow(attendee) {
  const event = current.value;
  // The listing owner is authoritative: seeded display names can differ from account names.
  const eligible = agent => (agent.role === 'agent' || agent.role === 'leader') && !agent.disabled;
  const owner = agents.value.find(agent => agent.id === event.listing.ownerId && eligible(agent)) ||
    agents.value.find(agent => agent.name === event.listing.agent && eligible(agent));
  if (!owner) return ElMessage.warning(t('没有经纪人'));
  const date = new Date();
  date.setDate(date.getDate() + 1);
  try {
    await api('admin/todos', {
      method: 'POST',
      body: JSON.stringify({
        id: `ofi-follow-${attendee.id}`,
        assigneeId: owner.id,
        cid: attendee.contactId,
        propId: event.listing.propertyId,
        kind: 'call',
        d: date.toLocaleDateString('en-CA'),
        t: `OFI follow-up · ${visitorName(attendee)} · ${event.address}`,
      }),
    });
    await api(`admin/ofi_attendees/${encodeURIComponent(attendee.id)}`, {
      method: 'PATCH', body: JSON.stringify({ version: attendee.version, follow: true }),
    });
    await load();
    ElMessage.success(t('已跟进'));
  } catch (error) {
    ElMessage.error(error.message);
    await load();
  }
}
onMounted(load);
onBeforeUnmount(() => Object.values(photoUrls.value).forEach(url => URL.revokeObjectURL(url)));
</script>

<style scoped>
.period-label, .filter-label { display: inline-flex; gap: 8px; align-items: center; }
.ofi {
  display: grid;
  grid-template-columns: 320px minmax(0, 1fr);
  gap: 16px;
  align-items: start;
}
.olist { display: flex; flex-direction: column; gap: 10px; }
.oc {
  display: flex;
  gap: 12px;
  background: #fff;
  border: 1px solid var(--line);
  border-radius: 14px;
  padding: 10px;
  cursor: pointer;
}
.oc.on { border-color: var(--gold); box-shadow: 0 4px 16px rgba(201, 162, 39, .18); }
.oimg {
  width: 78px;
  height: 78px;
  border-radius: 10px;
  flex: none;
  padding: 6px;
  background: linear-gradient(135deg, #8e795d, #23384a);
}
.oimg img { width: 100%; height: 100%; object-fit: cover; border-radius: 6px; }
.oimg:has(img) { padding: 0; position: relative; }
.oimg:has(img) .el-tag { position: absolute; top: 6px; left: 6px; }
.oa { font-size: 16px; font-weight: 700; }
.listing-agent { margin-top: 4px; }
.hero {
  color: white;
  display: flex;
  gap: 20px;
  padding: 24px;
  margin-bottom: 16px;
  align-items: center;
  border: 0;
  background: linear-gradient(120deg, #674b33, #302a2a);
}
.ha { font-size: 30px; font-weight: 700; margin: 6px 0; }
.suburb { opacity: .8; }
.facts { gap: 14px; margin-top: 12px; }
.price { color: #e6c35c; }
.hstats {
  display: grid;
  grid-template-columns: repeat(4, minmax(68px, 1fr));
  gap: 12px;
  text-align: center;
}
.hstats b { display: block; font-size: 30px; }
.hstats span { display: block; font-size: 12px; line-height: 1.25; opacity: .8; }
.vi {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 0;
  border-bottom: 1px solid var(--line);
}
.visitor-tags { flex-wrap: wrap; gap: 6px; }
</style>
