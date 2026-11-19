<template>
  <div class="page">
    <div class="page-head"><h1>{{ t('数据导出') }}</h1></div>
    <div class="card wizard">
      <el-steps :active="step" finish-status="success">
        <el-step :title="t('范围')" /><el-step :title="t('字段')" /><el-step :title="t('确认')" />
      </el-steps>
      <div v-if="step === 0" class="fields">
        <label>{{ t('区域') }}
          <el-select v-model="form.suburb" filterable style="width: 240px">
            <el-option value="" :label="t('全部')" />
            <el-option v-for="name in suburbs" :key="name" :value="name" :label="name" />
          </el-select>
        </label>
        <label>{{ t('状态') }}
          <el-select v-model="form.status" style="width: 240px">
            <el-option value="" :label="t('全部')" />
            <el-option value="registered" :label="t('已登记')" />
            <el-option value="unregistered" :label="t('未登记')" />
            <el-option value="contact" :label="t('有联系人')" />
          </el-select>
        </label>
        <el-checkbox v-model="form.excludeDNC">{{ t('排除勿扰及勿敲门') }}</el-checkbox>
      </div>
      <div v-else-if="step === 1" class="fields">
        <label>{{ t('选择字段') }}</label>
        <el-checkbox-group v-model="form.fields">
          <el-checkbox v-for="field in allowed" :key="field.id" :value="field.id">{{ t(field.label) }}</el-checkbox>
        </el-checkbox-group>
        <div class="muted">{{ t('第三方授权数据，不可导出') }}</div>
        <el-checkbox v-for="field in restricted" :key="field" disabled>{{ t(field) }}</el-checkbox>
      </div>
      <div v-else class="fields">
        <label>{{ t('导出原因') }}<el-input v-model="form.reason" type="textarea" /></label>
        <el-checkbox v-model="form.acknowledge">{{ t('确认仅用于已获授权的业务用途') }}</el-checkbox>
      </div>
      <div class="actions">
        <el-button v-if="step" @click="step--">{{ t('上一步') }}</el-button>
        <el-button v-if="step < 2" type="primary" @click="step++">{{ t('下一步') }}</el-button>
        <el-button v-else type="primary" :disabled="!form.reason.trim() || !form.acknowledge || !form.fields.length"
          @click="download">{{ t('下载 CSV') }}</el-button>
      </div>
    </div>
    <div class="card records">
      <div class="card-h"><h3>{{ t('导出记录') }}</h3></div>
      <div v-for="row in records" :key="row.at" class="record">{{ fmtDate(row.at) }} · {{ row.diff }}</div>
      <el-empty v-if="!records.length" :description="t('暂无记录')" />
    </div>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue';
import { ElMessage } from 'element-plus';
import { api, token } from '../api';
import { fmtDate } from '../admin-format';
import { t } from '../i18n';
const step = ref(0);
const suburbs = ref([]);
const records = ref([]);
const form = reactive({ suburb: '', status: '', excludeDNC: true,
  fields: ['address', 'suburb'], reason: '', acknowledge: false });
const allowed = [
  { id: 'address', label: '地址' }, { id: 'suburb', label: '区域' },
  { id: 'lotPlan', label: '地块编号' }, { id: 'contactName', label: '联系人姓名' },
  { id: 'mobile', label: '手机号' }, { id: 'email', label: '邮箱' },
  { id: 'tags', label: '标签' }, { id: 'notesCount', label: '备注数量' },
];
const restricted = ['业主姓名', '成交价格', '成交日期', '持有年限', '异地业主'];
async function load() {
  try {
    records.value = await api('admin/exports');
    suburbs.value = (await api('admin/parcel-stats')).map(item => item.suburb);
  } catch (error) { ElMessage.error(error.message); }
}
async function download() {
  try {
    const response = await fetch('/api/v1/admin/export', {
      method: 'POST', headers: { Authorization: `Bearer ${token.value}`, 'Content-Type': 'application/json' },
      body: JSON.stringify(form),
    });
    if (!response.ok) throw Error((await response.json()).error);
    const url = URL.createObjectURL(await response.blob());
    const anchor = document.createElement('a');
    anchor.href = url; anchor.download = 'properties.csv'; anchor.click();
    setTimeout(() => URL.revokeObjectURL(url), 30000);
    await load();
  } catch (error) { ElMessage.error(error.message); }
}
onMounted(load);
</script>

<style scoped>
.wizard { padding: 24px; }
.fields { display: flex; flex-direction: column; gap: 16px; margin: 28px 0; }
.fields label { display: flex; flex-direction: column; gap: 8px; }
.actions { display: flex; justify-content: flex-end; gap: 8px; }
.records { margin-top: 20px; }
.record { padding: 10px 20px; border-top: 1px solid var(--line); }
</style>
