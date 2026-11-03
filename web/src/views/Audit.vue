<template>
  <div class="page">
    <div class="page-head">
      <div><h1>{{ t('审计日志') }}</h1><div class="sub">{{ t('所有读写 Rex、导出、权限变更都会记录 · 长期保存') }}</div></div>
      <div class="sp" />
      <el-tag type="success" effect="plain" round><span class="dot pulse" style="background:#22c55e;width:7px;height:7px;margin-right:6px" />{{ t('实时写入') }}</el-tag>
    </div>
    <div class="card bar">
      <el-input v-model="q" :placeholder="t('地址、内容…')" :prefix-icon="Search" clearable style="width:240px" />
      <el-select v-model="who" :placeholder="t('操作人')" clearable style="width:160px"><el-option v-for="u in USERS" :key="u.id" :label="u.name" :value="u.name" /></el-select>
      <el-select v-model="act" :placeholder="t('操作类型')" clearable filterable style="width:170px"><el-option v-for="a in actions" :key="a" :label="t(a)" :value="a" /></el-select>
      <el-date-picker v-model="dr" type="daterange" :range-separator="t('至')" :start-placeholder="t('开始')" :end-placeholder="t('结束')" style="width:260px" />
      <div class="grow" /><span class="small muted">{{ t('{n} 条', { n: rows.length }) }}</span>
    </div>
    <div class="card">
      <el-table :data="rows.slice(0, 200)" size="small" height="calc(100vh - 290px)">
        <el-table-column :label="t('时间')" width="150"><template #default="{ row }">{{ fmtMD(row.at) }} {{ fmtTime(row.at) }}</template></el-table-column>
        <el-table-column :label="t('操作人')" width="150"><template #default="{ row }"><el-link underline="never" @click="who = row.who">{{ row.who }}</el-link></template></el-table-column>
        <el-table-column :label="t('操作')" width="140"><template #default="{ row }"><el-tag size="small" :type="tagType(row.action)" effect="plain" @click="act = row.action" style="cursor:pointer">{{ t(row.action) }}</el-tag></template></el-table-column>
        <el-table-column :label="t('对象')" width="200"><template #default="{ row }"><el-link underline="never" @click="q = row.target">{{ tx(row.target) }}</el-link></template></el-table-column>
        <el-table-column :label="t('详情')" show-overflow-tooltip><template #default="{ row }">{{ tx(row.detail) }}</template></el-table-column>
      </el-table>
    </div>
  </div>
</template>
<script setup>
import { ref, computed } from 'vue'
import { Search } from '@element-plus/icons-vue'
import { state, USERS, fmtMD, fmtTime } from '../data/store'
import { t, tx, locale } from '../i18n'
const q = ref(''), who = ref(''), act = ref(''), dr = ref(null)
const actions = computed(() => [...new Set(state.audit.map(a => a.action))])
const rows = computed(() => state.audit.filter(a => (!who.value || a.who === who.value) && (!act.value || a.action === act.value) && (!q.value || (a.target + a.detail).toLowerCase().includes(q.value.toLowerCase())) && (!dr.value || (a.at >= dr.value[0] && a.at <= new Date(dr.value[1].getTime() + 864e5)))))
const tagType = a => /撤销|免打扰|拒绝/.test(a) ? 'danger' : /导出/.test(a) ? 'warning' : /录入|关联/.test(a) ? 'success' : 'info'
</script>
<style scoped>
.bar { display: flex; gap: 10px; align-items: center; padding: 12px 14px; margin-bottom: 12px; flex-wrap: wrap; }
</style>
