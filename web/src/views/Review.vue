<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h1>{{ t('审核队列') }}</h1>
        <div class="sub">
          {{ t('快捷键') }} <span class="kbd">A</span> {{ t('通过') }}
          <span class="kbd">S</span> {{ t('跳过') }}
        </div>
      </div>
    </div>

    <el-tabs v-model="tab">
      <el-tab-pane
        v-for="entry in tabs"
        :key="entry.id"
        :name="entry.id"
        :label="`${t(entry.label)} (${entry.count})`"
      />
    </el-tabs>

    <div v-if="tab === 'approvals'" class="card card-b">
      <label class="filter-label">
        {{ t('状态') }}
        <el-select v-model="status" style="width: 170px" @change="load">
          <el-option :label="t('全部')" value="" />
          <el-option
            v-for="state in ['pending', 'approved', 'rejected']"
            :key="state"
            :label="t(statusNames[state])"
            :value="state"
          />
        </el-select>
      </label>
      <div v-for="item in approvals" :key="item.id" class="approval-row">
        <b class="grow">{{ item.summary || item.id }}</b>
        <span>{{ t(item.kind === 'merge' ? '合并记录' : '重新分配') }}</span>
        <StatusPill :status="item.status" short />
        <el-input
          v-if="item.status === 'pending'"
          v-model="notes[item.id]"
          :placeholder="t('备注')"
          style="width: 180px"
        />
        <el-button v-if="item.status === 'pending'" @click="decide(item, 'approve')">
          {{ t('通过') }}
        </el-button>
        <el-button v-if="item.status === 'pending'" @click="decide(item, 'reject')">
          {{ t('拒绝') }}
        </el-button>
      </div>
    </div>

    <div v-else class="rv">
      <div class="rlist card">
        <div
          v-for="item in items"
          :key="item.id"
          class="ri"
          :class="{ on: current?.id === item.id }"
          @click="selected = item.id"
        >
          <div class="grow">
            <b class="ellipsis">{{ tab === 'address' ? item.address : item.text }}</b>
            <div class="small muted">
              {{ tab === 'address' ? item.suburb : item.agentName }} ·
              {{ relTime(item.updated_at || item.date) }}
            </div>
          </div>
          <el-tag v-if="tab === 'address' && candidateCache[item.id]?.length" size="small" type="info">
            {{ t(basisNames[candidateCache[item.id][0].Basis]) }}
          </el-tag>
        </div>
        <el-empty v-if="!items.length" :description="t('这一类没有待审核项目')" />
      </div>

      <div v-if="current" class="card rd">
        <div class="card-h">
          <h3>{{ t(tab === 'address' ? '地址匹配' : 'AI 采集') }}</h3>
          <div class="sp" />
          <span class="small muted">{{ relTime(current.updated_at || current.date) }}</span>
        </div>

        <div class="card-b">
          <template v-if="tab === 'address'">
            <div class="comparison">
              <div class="comparison-card">
                <div class="small muted">{{ t('市场记录') }}</div>
                <div class="big">{{ current.address }}</div>
                <div class="small muted">{{ current.suburb }}</div>
                <div class="meta">
                  <span v-if="current.lotPlan">{{ current.lotPlan }}</span>
                  <span>{{ t(marketKinds[current.kind] || current.kind) }}</span>
                  <span>{{ money(current.price) }}</span>
                  <span>{{ fmtDate(current.date) }}</span>
                </div>
              </div>
              <div class="match-basis">
                <b>{{ candidate ? t(basisNames[candidate.Basis]) : '—' }}</b>
                <el-icon :size="22"><Right /></el-icon>
              </div>
              <div class="comparison-card">
                <div class="small muted">{{ t('选中地块') }}</div>
                <template v-if="candidate">
                  <div class="big">{{ candidate.Address }}</div>
                  <div class="small muted">{{ candidate.Suburb }} · {{ candidate.LotPlan || '—' }}</div>
                  <div class="owner-line">
                    <StatusPill :status="candidate.Registered ? 'registered' : 'unregistered'" short />
                    <span>{{ t('业主') }}: {{ candidate.OwnerNames || '—' }}</span>
                  </div>
                  <div class="small muted difference">{{ difference }}</div>
                </template>
                <el-empty v-else :description="t('暂无候选地块')" :image-size="40" />
              </div>
            </div>

            <ReviewParcelMap v-if="candidate" :candidate="candidate" />
            <h3 class="candidate-heading">{{ t('候选地块') }}</h3>
            <div class="candidate-list">
              <label
                v-for="parcel in visibleCandidates"
                :key="parcel.ID"
                class="candidate-row"
                :class="{ on: chosen === parcel.ID }"
              >
                <input v-model="chosen" type="radio" name="parcel" :value="parcel.ID" />
                <span class="grow">
                  <b>{{ parcel.Address }}, {{ parcel.Suburb }}</b>
                  <span class="small muted">{{ parcel.LotPlan || '—' }} · {{ parcel.OwnerNames || '—' }}</span>
                </span>
                <el-tag size="small" type="info">{{ t(basisNames[parcel.Basis]) }}</el-tag>
              </label>
              <el-button v-if="candidates.length > 5" link @click="showMore = !showMore">
                {{ t(showMore ? '收起候选' : '更多候选') }}
              </el-button>
            </div>
          </template>

          <template v-else>
            <p>
              {{ t('经纪人') }}: {{ current.agentName || '—' }} ·
              {{ t('房产') }}: {{ property?.address || '—' }}
            </p>
            <div class="transcript">{{ current.text }}</div>
            <h3>{{ t('提取字段') }}</h3>
            <div
              v-for="(value, field) in current.extracted || {}"
              :key="field"
              class="field"
              :class="{ unsure: unsureField(current, field) }"
            >
              {{ field }}: {{ typeof value === 'object' ? JSON.stringify(value) : value }}
            </div>
          </template>
        </div>

        <div class="acts">
          <el-button @click="skip">{{ t('跳过') }} <span class="kbd">S</span></el-button>
          <el-button v-if="tab === 'address'" @click="ignore">{{ t('不是同一地址') }}</el-button>
          <div class="sp" />
          <el-button v-if="tab === 'address'" class="gold-btn" :disabled="!candidate" @click="link(chosen)">
            {{ t('确认关联') }} <span class="kbd">A</span>
          </el-button>
          <el-button v-else class="gold-btn" @click="remind">
            {{ t('提醒经纪人') }} <span class="kbd">A</span>
          </el-button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue';
import { ElMessage } from 'element-plus';
import { Right } from '@element-plus/icons-vue';
import { api } from '../api';
import { fmtDate, relTime, money } from '../admin-format';
import { t } from '../i18n';
import StatusPill from '../components/StatusPill.vue';
import ReviewParcelMap from '../components/ReviewParcelMap.vue';

const tab = ref('address');
const address = ref([]);
const captures = ref([]);
const approvals = ref([]);
const candidateCache = ref({});
const chosen = ref('');
const selected = ref('');
const status = ref('pending');
const notes = ref({});
const property = ref(null);
const showMore = ref(false);
const statusNames = { pending: '待审核', approved: '已批准', rejected: '已拒绝' };
const marketKinds = { sale: '成交', listing: '挂牌', withdrawn: '撤牌' };
const basisNames = {
  'lot-exact': '地籍完全一致',
  'lot-range': '地籍范围内',
  'address-exact': '街道门牌一致',
  street: '同一街道',
};
const tabs = computed(() => [
  { id: 'address', label: '地址匹配', count: address.value.length },
  { id: 'ai', label: 'AI 采集', count: captures.value.length },
  { id: 'approvals', label: '审批', count: approvals.value.filter(item => item.status === 'pending').length },
]);
const items = computed(() => tab.value === 'address' ? address.value : captures.value);
const current = computed(() => items.value.find(item => item.id === selected.value) || items.value[0]);
const candidates = computed(() => candidateCache.value[current.value?.id] || []);
const candidate = computed(() => candidates.value.find(item => item.ID === chosen.value));
const visibleCandidates = computed(() => showMore.value ? candidates.value : candidates.value.slice(0, 5));
const difference = computed(() => {
  if (!candidate.value || !current.value) return '';
  const differences = [];
  const marketNumber = current.value.address.match(/^\d+/)?.[0];
  const parcelNumber = candidate.value.Address.match(/^\d+/)?.[0];
  if (marketNumber && parcelNumber && marketNumber !== parcelNumber) differences.push(t('门牌号不同'));
  if (!current.value.lotPlan && candidate.value.LotPlan) {
    differences.push(t('市场记录无地籍编号'));
  } else if (current.value.lotPlan && current.value.lotPlan !== candidate.value.LotPlan) {
    differences.push(t('地籍编号不同'));
  }
  return differences.length ? differences.join(' · ') : t('地址与地籍信息一致');
});

async function load() {
  try {
    [address.value, captures.value, approvals.value] = await Promise.all([
      api('admin/market-review'),
      api('admin/captures'),
      api(`admin/approvals?status=${status.value}`),
    ]);
    const ids = address.value.map(item => item.id);
    const responses = await Promise.allSettled(ids.map(id =>
      api(`admin/market-review/${encodeURIComponent(id)}/candidates`),
    ));
    candidateCache.value = Object.fromEntries(ids.map((id, index) => [
      id, responses[index].status === 'fulfilled' ? responses[index].value : [],
    ]));
  } catch (error) {
    ElMessage.error(error.message);
  }
}
watch([current, candidates], ([item, list]) => {
  showMore.value = false;
  if (tab.value === 'address') {
    if (!list.some(parcel => parcel.ID === chosen.value)) chosen.value = list[0]?.ID || '';
    return;
  }
  property.value = null;
  if (item?.propId) {
    api(`admin/properties?q=${encodeURIComponent(item.propId)}`)
      .then(rows => { property.value = rows[0] || null; })
      .catch(error => ElMessage.error(error.message));
  }
});
async function link(id) {
  if (!current.value || !id) return;
  try {
    await api(`admin/market-review/${encodeURIComponent(current.value.id)}/link`, {
      method: 'POST', body: JSON.stringify({ propertyId: id }),
    });
    await load();
  } catch (error) {
    ElMessage.error(error.message);
  }
}
async function ignore() {
  if (!current.value) return;
  try {
    await api(`admin/market-review/${encodeURIComponent(current.value.id)}/ignore`, {
      method: 'POST', body: '{}',
    });
    await load();
  } catch (error) {
    ElMessage.error(error.message);
  }
}
async function remind() {
  const item = current.value;
  if (!item?.ownerId) return ElMessage.warning(t('没有经纪人'));
  try {
    await api('admin/todos', {
      method: 'POST',
      body: JSON.stringify({
        id: `capture-remind-${item.id}`,
        assigneeId: item.ownerId,
        d: new Date().toLocaleDateString('en-CA'),
        t: `Review voice capture · ${property.value?.address || item.text.slice(0, 48)}`,
        kind: 'call',
        propId: item.propId,
      }),
    });
    ElMessage.success(t('已创建'));
    skip();
  } catch (error) {
    ElMessage.error(error.message);
  }
}
async function decide(item, decision) {
  try {
    await api(`approvals/${encodeURIComponent(item.id)}/decide`, {
      method: 'POST', body: JSON.stringify({ decision, note: notes.value[item.id] || '' }),
    });
    await load();
  } catch (error) {
    ElMessage.error(error.message);
  }
}
function unsureField(item, field) {
  return field === 'unsure' || item.extracted?.unsure?.includes?.(field) ||
    (typeof item.extracted?.confidence?.[field] === 'number' && item.extracted.confidence[field] < 0.75);
}
function skip() {
  const index = items.value.findIndex(item => item.id === current.value?.id);
  selected.value = items.value[(index + 1) % items.value.length]?.id || '';
}
function key(event) {
  if (['INPUT', 'TEXTAREA'].includes(event.target.tagName)) return;
  if (event.key.toLowerCase() === 's') skip();
  if (event.key.toLowerCase() === 'a' && tab.value === 'ai') remind();
  if (event.key.toLowerCase() === 'a' && tab.value === 'address' && chosen.value) link(chosen.value);
}
onMounted(() => {
  load();
  window.addEventListener('keydown', key);
});
onBeforeUnmount(() => window.removeEventListener('keydown', key));
</script>

<style scoped>
.rv {
  display: grid;
  grid-template-columns: 340px minmax(0, 1fr);
  gap: 16px;
  align-items: start;
}
.rlist {
  padding: 6px;
  max-height: calc(100vh - 260px);
  overflow: auto;
}
.ri {
  display: flex;
  gap: 10px;
  align-items: center;
  padding: 12px;
  border-bottom: 1px solid var(--line);
  cursor: pointer;
}
.ri b { display: block; }
.ri.on, .candidate-row.on {
  background: var(--gold-soft);
  box-shadow: inset 3px 0 0 var(--gold);
}
.ri .el-tag { flex: none; max-width: 100px; }
.comparison {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 104px minmax(0, 1fr);
  gap: 12px;
  align-items: center;
}
.comparison-card {
  min-height: 170px;
  border: 1px solid var(--line);
  border-radius: 12px;
  padding: 16px;
}
.big { font-size: 19px; font-weight: 700; margin: 8px 0; }
.meta { display: flex; flex-wrap: wrap; gap: 6px 12px; margin-top: 12px; }
.match-basis {
  display: flex;
  align-items: center;
  flex-direction: column;
  text-align: center;
  gap: 8px;
  color: var(--navy);
  font-size: 12px;
}
.owner-line { display: flex; align-items: center; gap: 8px; margin: 12px 0; }
.difference { border-top: 1px solid var(--line); padding-top: 8px; }
.candidate-heading { margin: 16px 0 8px; }
.candidate-list { border: 1px solid var(--line); border-radius: 10px; overflow: hidden; }
.candidate-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--line);
  cursor: pointer;
}
.candidate-row .small { display: block; }
.candidate-row input { accent-color: #aa8221; }
.candidate-list > .el-button { margin: 8px 12px; }
.transcript { padding: 14px; background: #f7f8fa; border-radius: 10px; line-height: 1.7; }
.field { padding: 6px 10px; }
.unsure { background: #fef9c3; }
.acts { display: flex; gap: 8px; padding: 12px 18px 18px; }
.filter-label { display: inline-flex; align-items: center; gap: 8px; margin-bottom: 12px; }
.approval-row {
  display: flex;
  gap: 12px;
  align-items: center;
  padding: 12px;
  border-top: 1px solid var(--line);
}
</style>
