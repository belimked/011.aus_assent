<template>
  <div class="page">
    <div class="page-head"><div><h1>{{ t('系统设置') }}</h1><div class="sub">{{ t('PDF 中的待定事项集中在这里，确认后即可生效 · 修改会记入审计日志') }}</div></div><div class="sp" /><el-button type="primary" @click="save">{{ t('保存设置') }}</el-button></div>
    <div class="sg">
      <div class="card"><div class="card-h"><h3>{{ t('着色规则') }}</h3></div><div class="card-b">
        <el-form label-position="top">
          <el-form-item :label="t('哪些关系算「已关联业主」（绿色）')"><el-select v-model="s.ownerRoles" multiple style="width:100%"><el-option v-for="r in ['业主', '房东', '共同业主', '租客', '亲属', '物业经理']" :key="r" :label="t(r)" :value="r" /></el-select></el-form-item>
          <el-form-item :label="t('其他团队跟进的房产显示为')">
            <el-radio-group v-model="s.otherMode"><el-radio value="neutral">{{ t('紫色中性点（不透露归属）') }}</el-radio><el-radio value="red">{{ t('与未关联相同（红色）') }}</el-radio></el-radio-group>
          </el-form-item>
        </el-form></div></div>
      <div class="card"><div class="card-h"><h3>{{ t('第三方数据') }}</h3></div><div class="card-b">
        <el-form label-position="top">
          <el-form-item :label="t('数据商')"><el-radio-group v-model="s.provider"><el-radio-button value="Pricefinder" /><el-radio-button value="RP Data" /></el-radio-group></el-form-item>
          <el-alert type="warning" :closable="false" show-icon :title="t('批量显示业主名（地图白心、列表列、10 年筛选）需数据商书面授权')" />
        </el-form></div></div>
      <div class="card"><div class="card-h"><h3>{{ t('标签与邮件列表') }}</h3></div><div class="card-b">
        <div class="small muted" style="margin-bottom:6px">{{ t('Rex 标签（经纪人可选）') }}</div>
        <el-tag v-for="tag in s.tags" :key="tag" closable @close="s.tags.splice(s.tags.indexOf(tag), 1)" class="tg">{{ t(tag) }}</el-tag>
        <el-input v-model="nt" size="small" style="width:120px" :placeholder="t('+ 新标签')" @keydown.enter="nt && s.tags.push(nt); nt = ''" />
        <div class="small muted" style="margin:14px 0 6px">{{ t('邮件列表') }}</div>
        <el-tag v-for="list in s.lists" :key="list" type="info" class="tg">{{ t(list) }}</el-tag>
      </div></div>
      <div class="card"><div class="card-h"><h3>{{ t('安全与保留') }}</h3></div><div class="card-b">
        <el-form label-width="205px">
          <el-form-item :label="t('原始录音/照片保留')"><el-input-number v-model="s.retention" :min="1" :max="90" /> <span class="muted small" style="margin-left:8px">{{ t('天') }}</span></el-form-item>
          <el-form-item :label="t('手机缓存失效')"><el-input-number v-model="s.cacheHours" :min="1" :max="72" /> <span class="muted small" style="margin-left:8px">{{ t('小时') }}</span></el-form-item>
          <el-form-item :label="t('异常访问阈值')"><el-slider v-model="s.anomaly" :min="20" :max="200" show-input style="width:300px" /></el-form-item>
          <el-form-item :label="t('页面水印')"><el-switch v-model="s.watermark" /></el-form-item>
        </el-form>
      </div></div>
    </div>
  </div>
</template>
<script setup>
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { state, audit } from '../data/store'
import { t, tx, locale } from '../i18n'
const s = state.settings, nt = ref('')
function save() { audit('修改设置', '系统设置', `保留 ${s.retention} 天 · 缓存 ${s.cacheHours}h · 阈值 ${s.anomaly}`); ElMessage.success(t('设置已保存')) }
</script>
<style scoped>
.sg { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
.tg { margin: 0 6px 6px 0; }
</style>
