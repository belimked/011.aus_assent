<template>
  <div class="page">
    <div class="page-head"><h1>{{ t('Rex 同步事件') }}</h1></div>
    <div class="card bar">
      <label class="field">{{ t('方向') }}<el-select v-model="filter.direction" clearable style="width:150px"><el-option label="App → Rex" value="app_to_rex" /><el-option label="Rex → App" value="rex_to_app" /></el-select></label>
      <label class="field">{{ t('结果') }}<el-select v-model="result" clearable style="width:130px" @change="load(0)"><el-option v-for="item in results" :key="item.value" :value="item.value" :label="t(item.label)" /></el-select></label>
      <label v-for="key in ['entity','local_id','rex_id']" :key="key" class="field">{{ t(labels[key]) }}<el-input v-model="filter[key]" clearable /></label>
      <el-button @click="load(0)">{{ t('筛选') }}</el-button>
    </div>
    <div class="card"><RexEventsTable :items="visibleItems" /></div>
    <div class="pager"><span class="small muted">{{ t('本页 {n} 条', { n: visibleItems.length }) }} · {{ t('共 {n} 条', { n: total }) }}</span><el-button :disabled="offset === 0" @click="changePage(offset - limit)">{{ t('上一页') }}</el-button><el-button :disabled="offset + limit >= total" @click="changePage(offset + limit)">{{ t('下一页') }}</el-button></div>
  </div>
</template>
<script setup>
import { computed, ref, watch } from 'vue';
import { useRoute } from 'vue-router';
import { ElMessage } from 'element-plus';
import { api } from '../api';
import { t } from '../i18n';
import RexEventsTable from '../components/RexEventsTable.vue';
const route = useRoute();
const labels = { entity: '实体', local_id: '本地 ID', rex_id: 'Rex ID' };
const results = [{ value: 'ok', label: '成功' }, { value: 'error', label: '失败' }, { value: 'skipped', label: '已跳过' }, { value: 'blocked', label: '被拦截' }];
const filter = ref({ direction: '', entity: '', local_id: '', rex_id: '' });
const result = ref('');
const items = ref([]), total = ref(0), offset = ref(0);
const limit = 50;
let requestId = 0;
const visibleItems = computed(() => result.value ? items.value.slice(offset.value, offset.value + limit) : items.value);
function changePage(next) { if (result.value) offset.value = next; else load(next); }
async function load(next = 0) {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(filter.value)) if (value) params.set(key, value);
  const current = ++requestId;
  try {
    if (result.value) {
      // Result is not an endpoint parameter: collect server pages, then filter and paginate locally.
      const matches = [];
      let fetched = 0, count = 0;
      do {
        params.set('limit', limit); params.set('offset', fetched);
        const data = await api(`admin/rex/events?${params}`);
        if (current !== requestId) return;
        count = data.total || 0;
        const page = data.items || [];
        matches.push(...page.filter(item => item.result === result.value));
        fetched += page.length;
        if (!page.length) break;
      } while (fetched < count);
      items.value = matches; total.value = matches.length;
    } else {
      params.set('limit', limit); params.set('offset', next);
      const data = await api(`admin/rex/events?${params}`);
      if (current !== requestId) return;
      items.value = data.items || []; total.value = data.total || 0;
    }
    offset.value = next;
  } catch (error) { if (current === requestId) ElMessage.error(error.message); }
}
watch(() => route.query, (query) => {
  for (const key of Object.keys(filter.value)) filter.value[key] = typeof query[key] === 'string' ? query[key] : '';
  result.value = typeof query.result === 'string' ? query.result : '';
  load(0);
}, { immediate: true });
</script>
<style scoped>.bar { display: flex; flex-wrap: wrap; gap: 12px; align-items: end; padding: 14px; margin-bottom: 14px; }.field { display: grid; gap: 5px; min-width: 140px; font-size: 12px; color: var(--mut); }.pager { display: flex; align-items: center; justify-content: flex-end; gap: 12px; margin-top: 14px; }</style>
