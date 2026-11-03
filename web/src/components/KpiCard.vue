<template>
  <div class="kpi" :class="{ clickable: !!to, dark }" @click="to && router.push(to)">
    <div class="row top">
      <div class="ic" :style="{ background: color + '1a', color }"><el-icon :size="18"><component :is="icon" /></el-icon></div>
      <span class="lb">{{ label }}</span>
      <el-tooltip v-if="tip" :content="tip"><el-icon class="muted" style="margin-left:auto"><QuestionFilled /></el-icon></el-tooltip>
    </div>
    <div class="v serif">{{ shown }}<small v-if="unit">{{ unit }}</small></div>
    <div class="row ft">
      <span v-if="d !== null && d !== undefined" class="d" :class="d >= 0 ? 'up' : 'down'">{{ d >= 0 ? '▲' : '▼' }} {{ Math.abs(d) }}%</span>
      <span class="small muted">{{ foot }}</span>
    </div>
    <svg v-if="spark?.length" class="spark" viewBox="0 0 100 28" preserveAspectRatio="none"><polyline :points="pts" fill="none" :stroke="color" stroke-width="1.8" /><polygon :points="'0,28 ' + pts + ' 100,28'" :fill="color" opacity=".08" /></svg>
  </div>
</template>
<script setup>
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
const router = useRouter()
const props = defineProps({ label: String, value: Number, unit: String, d: Number, foot: String, icon: String, color: { type: String, default: '#0f2a44' }, spark: Array, to: String, tip: String, dark: Boolean })
const shown = ref(0)
watch(() => props.value, (v, o) => {
  const from = o ?? 0, t0 = performance.now()
  const step = t => { const k = Math.min(1, (t - t0) / 600); shown.value = Math.round(from + (v - from) * (1 - Math.pow(1 - k, 3))); if (k < 1) requestAnimationFrame(step) }
  requestAnimationFrame(step)
}, { immediate: true })
const pts = computed(() => { const a = props.spark, mx = Math.max(1, ...a); return a.map((v, i) => `${(i / (a.length - 1)) * 100},${26 - (v / mx) * 22}`).join(' ') })
</script>
<style scoped>
.kpi { background: #fff; border: 1px solid var(--line); border-radius: 14px; padding: 14px 16px 10px; position: relative; overflow: hidden; transition: .15s; }
.kpi.clickable { cursor: pointer; }
.kpi.clickable:hover { border-color: #c8d5e2; box-shadow: 0 6px 18px rgba(15, 42, 68, .08); transform: translateY(-1px); }
.ic { width: 30px; height: 30px; border-radius: 8px; display: grid; place-items: center; }
.lb { font-size: 13px; color: #475467; }
.v { font-size: 30px; font-weight: 700; margin: 8px 0 2px; color: var(--ink); }
.v small { font-size: 14px; margin-left: 2px; color: var(--mut); font-family: var(--el-font-family); }
.ft { gap: 8px; position: relative; z-index: 1; }
.d { font-size: 12px; font-weight: 700; }
.up { color: #16a34a; } .down { color: #dc2626; }
.spark { position: absolute; right: 0; bottom: 0; width: 55%; height: 40px; }
.dark { background: rgba(255, 255, 255, .04); border-color: rgba(255, 255, 255, .08); }
.dark .lb, .dark .muted { color: #9fb2c8; }
.dark .v { color: #fff; }
</style>
