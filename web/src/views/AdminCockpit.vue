<template>
  <div class="cockpit">
    <div class="heading"><h1>{{ t('数据驾驶舱') }}</h1>
      <el-button @click="toggle">{{ t(fullscreen ? '退出全屏' : '全屏') }}</el-button>
    </div>
    <AdminDashboard />
  </div>
</template>

<script setup>
import { ref, onUnmounted } from 'vue';
import { t } from '../i18n';
import AdminDashboard from './AdminDashboard.vue';
const fullscreen = ref(false);
function toggle() {
  if (document.fullscreenElement) document.exitFullscreen();
  else document.querySelector('.cockpit').requestFullscreen();
  fullscreen.value = !fullscreen.value;
}
onUnmounted(() => { if (document.fullscreenElement) document.exitFullscreen(); });
</script>

<style scoped>
.cockpit { min-height: 100%; padding: 24px; background: #0f2a44; color: white; }
.cockpit :deep(.page) { color: #162438; }
.heading { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
</style>
