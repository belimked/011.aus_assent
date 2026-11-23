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
          <p>{{ t('账户 ID') }}: {{ rex.accountId || '—' }} · {{ t('地址') }}: {{ rex.baseUrl || '—' }} · App ID: {{ rex.appId || '—' }}</p>
          <p>{{ t('已验证账户') }}: <strong>{{ rex.verifiedAccount || '—' }}</strong> <el-tag v-if="String(rex.verifiedAccount) === '2057'" type="success" size="small">{{ t('沙箱') }}</el-tag></p>
          <p><el-tag :type="rex.writeBlocked ? 'warning' : String(rex.verifiedAccount) === '2057' ? 'success' : 'info'">{{ rex.writeBlocked ? t('写入被拦截') : t('写入未拦截') }}</el-tag> <span v-if="rex.writeBlocked">{{ rex.writeBlockReason || '—' }}</span></p>
          <p>{{ t('读取') }}: {{ rex.read ? t('已开启') : t('已关闭') }} · {{ t('写入') }}: {{ rex.write ? t('已开启') : t('已关闭') }}</p>
          <p>{{ t('自动拉取') }}: {{ rex.pollEnabled ? t('已开启') : t('已关闭') }} · {{ t('上次拉取') }}: {{ rex.lastPollAt ? fmtDate(rex.lastPollAt) : '—' }} · {{ rex.lastPollAt ? (rex.lastPollOK ? t('成功') : t('失败')) : '—' }}</p>
          <p v-if="rex.readReason" class="muted">{{ t('读取原因') }}: {{ rex.readReason }}</p>
          <p v-if="rex.writeReason" class="muted">{{ t('写入原因') }}: {{ rex.writeReason }}</p>
          <p>{{ t('最近登录') }}: {{ rex.lastLoginAt ? fmtDate(rex.lastLoginAt) : '—' }} · {{ rex.lastLoginAt ? (rex.lastLoginOK ? t('成功') : t('失败')) : '—' }}</p>
          <p>{{ t('最近一小时调用') }}: {{ rex.callsHour || 0 }} · {{ t('失败') }}: {{ rex.failuresHour || 0 }}</p>
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
const rex = ref({});
const hours = ref(720);
async function load() {
  try { [data.value, rex.value] = await Promise.all([api('admin/settings'), api('admin/rex/status')]); hours.value = Number(data.value.sessionHours); }
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
