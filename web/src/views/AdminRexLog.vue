<template>
  <div class="page">
    <div class="page-head"><h1>{{ t('Rex 调用日志') }}</h1></div>
    <div class="card bar">
      <label v-for="key in ['service','method','caller']" :key="key" class="field">{{ t(labels[key]) }}<el-input v-model="filter[key]" clearable /></label>
      <label class="field">{{ t('类型') }}<el-select v-model="filter.kind" clearable style="width: 120px"><el-option v-for="k in ['auth','read','write']" :key="k" :label="k" :value="k" /></el-select></label>
      <label class="field">{{ t('结果') }}<el-select v-model="filter.ok" clearable style="width: 110px"><el-option :label="t('成功')" value="1" /><el-option :label="t('失败')" value="0" /></el-select></label>
      <label class="field">{{ t('开始') }}<input v-model="filter.from" type="datetime-local" /></label>
      <label class="field">{{ t('结束') }}<input v-model="filter.to" type="datetime-local" /></label>
      <el-button @click="load(true)">{{ t('筛选') }}</el-button>
    </div>
    <div class="card">
      <el-table :data="rows" @row-click="open">
        <el-table-column prop="at" :label="t('时间')" min-width="180" />
        <el-table-column prop="accountId" :label="t('账户 ID')" width="110" />
        <el-table-column prop="direction" :label="t('方向')" width="90" />
        <el-table-column prop="kind" :label="t('类型')" width="90" />
        <el-table-column prop="service" label="Service" />
        <el-table-column prop="method" label="Method" />
        <el-table-column prop="caller" :label="t('调用方')" />
        <el-table-column prop="httpStatus" label="HTTP" width="90" />
        <el-table-column :label="t('结果')" width="90"><template #default="{row}">{{ row.ok ? t('成功') : t('失败') }}</template></el-table-column>
        <el-table-column prop="resultSummary" :label="t('摘要')" show-overflow-tooltip />
      </el-table>
      <el-button v-if="hasMore" @click="load(false)">{{ t('加载更多') }}</el-button>
    </div>
    <el-dialog v-model="visible" :title="t('Rex 调用详情')" width="min(850px, 95vw)">
      <p>{{ detail.at }} · {{ detail.service }} / {{ detail.method }} · {{ t('尝试') }} {{ detail.attempt }} · {{ detail.durationMs }} ms</p>
      <p v-if="detail.error">{{ detail.error }}</p>
      <h4>{{ t('请求参数') }}</h4><pre>{{ pretty(detail.requestJson) }}</pre>
      <h4>{{ t('响应摘录') }}</h4><pre>{{ pretty(detail.responseExcerpt) }}</pre>
    </el-dialog>
  </div>
</template>
<script setup>
import { onMounted, ref } from 'vue';
import { ElMessage } from 'element-plus';
import { api } from '../api';
import { t } from '../i18n';
const labels = { service: '服务', method: '方法', caller: '调用方' };
const filter = ref({ service: '', method: '', caller: '', kind: '', ok: '', from: '', to: '' });
const rows = ref([]), detail = ref({}), visible = ref(false), hasMore = ref(false);
function pretty(s) { try { return JSON.stringify(JSON.parse(s), null, 2) } catch { return s || '' } }
async function load(reset) {
 try {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(filter.value)) if (value !== '') params.set(key, key === 'from' || key === 'to' ? new Date(value).toISOString() : value);
  if (!reset && rows.value.length) params.set('before', rows.value.at(-1).id);
  const data = await api(`admin/rex/log?${params}`);
  rows.value = reset ? data.items : [...rows.value, ...data.items];
  hasMore.value = data.items.length === 50;
 } catch (e) { ElMessage.error(e.message) }
}
async function open(row) { try { detail.value = await api(`admin/rex/log/${row.id}`); visible.value = true } catch (e) { ElMessage.error(e.message) } }
onMounted(() => load(true));
</script>
<style scoped>pre { white-space: pre-wrap; overflow-wrap: anywhere; max-height: 270px; overflow: auto; background: #f5f6f7; padding: 12px; }</style>
