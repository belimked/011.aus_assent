<template>
  <el-table :data="items" row-key="id">
    <el-table-column type="expand" width="42">
      <template #default="{ row }">
        <div v-if="Object.keys(row.changes || {}).length" class="changes">
          <div v-for="(change, field) in row.changes" :key="field"><b>{{ field }}</b>: <code>{{ display(change?.old) }}</code> → <code>{{ display(change?.new) }}</code></div>
        </div>
        <p v-else class="changes">{{ t('无字段变更') }}</p>
      </template>
    </el-table-column>
    <el-table-column :label="t('时间')" min-width="170"><template #default="{ row }">{{ fmtDateTime(row.at) }}</template></el-table-column>
    <el-table-column :label="t('方向')" min-width="120"><template #default="{ row }"><el-tag :type="row.direction === 'rex_to_app' ? 'success' : 'info'" size="small">{{ row.direction === 'rex_to_app' ? 'Rex → App' : 'App → Rex' }}</el-tag></template></el-table-column>
    <el-table-column :label="t('记录')" min-width="160"><template #default="{ row }">{{ row.entity || '—' }} · {{ row.localId || '—' }}</template></el-table-column>
    <el-table-column prop="rexId" :label="t('Rex ID')" min-width="110" />
    <el-table-column :label="t('变更字段')" min-width="125"><template #default="{ row }">{{ Object.keys(row.changes || {}).join(', ') || '—' }}</template></el-table-column>
    <el-table-column :label="t('结果')" width="105"><template #default="{ row }"><el-tag :type="resultType(row.result)" size="small">{{ t(resultLabels[row.result] || row.result) }}</el-tag></template></el-table-column>
    <el-table-column prop="error" :label="t('错误')" min-width="160" show-overflow-tooltip />
  </el-table>
</template>
<script setup>
import { fmtDateTime } from '../admin-format';
import { t } from '../i18n';
defineProps({ items: { type: Array, default: () => [] } });
const resultLabels = { ok: '成功', error: '失败', skipped: '已跳过', blocked: '被拦截' };
const resultType = (result) => ({ ok: 'success', error: 'danger', skipped: 'info', blocked: 'warning' })[result] || 'info';
const display = (value) => value === undefined ? '—' : JSON.stringify(value);
</script>
<style scoped>.changes { padding: 0 18px; display: grid; gap: 8px; overflow-wrap: anywhere; } code { white-space: pre-wrap; }</style>
