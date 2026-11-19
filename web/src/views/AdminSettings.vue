<template>
  <div class="page">
    <div class="page-head"><h1>{{ t('系统设置') }}</h1></div>
    <div class="settings">
      <div class="card">
        <div class="card-h"><h3>{{ t('会话有效期') }}</h3></div>
        <div class="card-b">
          <label>{{ t('小时') }} <el-input-number v-model="hours" :min="1" :max="720" /></label>
          <p class="small muted">{{ t('仅对新登录的会话生效') }}</p>
          <el-button type="primary" @click="save">{{ t('保存') }}</el-button>
        </div>
      </div>
      <div class="card">
        <div class="card-h"><h3>{{ t('公司标签') }}</h3></div>
        <div class="card-b">{{ t('手机使用内置标签；公司标签暂不可编辑') }}</div>
      </div>
      <div class="card">
        <div class="card-h"><h3>{{ t('第三方数据') }}</h3></div>
        <div class="card-b">
          <p>{{ data.provider }} · {{ fmtDate(data.lastImport) }} · {{ data.importRows || 0 }} {{ t('行') }}</p>
          <p class="muted">{{ t('第三方授权数据，不可导出') }}</p>
        </div>
      </div>
      <div class="card">
        <div class="card-h"><h3>{{ t('Rex 连接') }}</h3></div>
        <div class="card-b">
          <p>{{ t(data.rexConfigured ? '已配置' : '未配置') }}</p>
          <p>{{ t('待投递') }}: {{ data.rexPending }} · {{ t('失败') }}: {{ data.rexFailed }}</p>
          <p v-if="data.rexError" class="muted">{{ data.rexError }}</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue';
import { ElMessage } from 'element-plus';
import { api } from '../api';
import { fmtDate } from '../admin-format';
import { t } from '../i18n';
const data = ref({});
const hours = ref(720);
async function load() {
  try { data.value = await api('admin/settings'); hours.value = Number(data.value.sessionHours); }
  catch (error) { ElMessage.error(error.message); }
}
async function save() {
  try {
    await api('admin/settings', { method: 'PATCH', body: JSON.stringify({ sessionHours: String(hours.value) }) });
    ElMessage.success(t('已保存'));
    await load();
  } catch (error) { ElMessage.error(error.message); }
}
onMounted(load);
</script>

<style scoped>
.settings { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; }
@media (max-width: 800px) { .settings { grid-template-columns: 1fr; } }
</style>
