<template>
  <div class="cockpit">
    <header class="heading">
      <div><h1>{{ t('数据驾驶舱') }}</h1><span>{{ t('每分钟更新') }}</span></div>
      <div class="controls">
        <label>{{ t('周期') }}
          <el-select v-model="period" style="width: 130px">
            <el-option value="today" :label="t('今天')" />
            <el-option value="7" :label="t('7 天')" />
            <el-option value="30" :label="t('30 天')" />
            <el-option value="90" :label="t('90 天')" />
          </el-select>
        </label>
        <el-button @click="toggle">{{ t(fullscreen ? '退出全屏' : '全屏') }}</el-button>
      </div>
    </header>
    <div class="kpis">
      <div v-for="card in cards" :key="card.key" class="panel kpi">
        <span>{{ t(card.label) }}</span><b>{{ card.value }}</b>
        <small>{{ t('较上期 {n}', { n: card.previous }) }}</small>
      </div>
    </div>
    <div class="grid">
      <section class="panel gauge"><h2>{{ t('区域覆盖率') }}</h2>
        <v-chart class="chart" :option="gaugeOption" autoresize />
        <p>{{ t('已关联 {n} / {total} 处房产', {
          n: metrics.coverage?.linked || 0, total: metrics.coverage?.total || 0,
        }) }}</p>
      </section>
      <section class="panel"><h2>{{ t('采集转化漏斗') }}</h2>
        <v-chart class="chart" :option="funnelOption" autoresize />
      </section>
      <section class="panel map-panel"><h2>{{ t('区域地图') }}</h2>
        <div class="map-wrap"><AdminMap /></div>
      </section>
      <section class="panel"><h2>{{ t('每日活动趋势') }}</h2>
        <v-chart v-if="metrics.actions?.length" class="chart" :option="trendOption" autoresize />
        <el-empty v-else :description="t('暂无活动')" />
      </section>
      <section class="panel"><h2>{{ t('经纪人表现') }}</h2>
        <el-table :data="ranked.slice(0, 8)" max-height="310">
          <el-table-column prop="name" :label="t('经纪人')" />
          <el-table-column prop="visits" :label="t('上门拜访')" />
          <el-table-column prop="contacts" :label="t('新联系人')" />
          <el-table-column prop="appraisals" :label="t('预约估价')" />
        </el-table>
      </section>
      <section class="panel"><h2>{{ t('外勤动态') }}</h2>
        <div class="feed">
          <div v-for="event in metrics.events || []" :key="event.id" class="event">
            <b>{{ event.agent }}</b> · {{ event.detail }} · {{ relTime(event.at) }}
          </div>
          <el-empty v-if="!metrics.events?.length" :description="t('暂无活动')" />
        </div>
      </section>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue';
import { ElMessage } from 'element-plus';
import VChart from '../components/chart';
import { api } from '../api';
import { relTime } from '../admin-format';
import { t } from '../i18n';
import AdminMap from './AdminMap.vue';
const period = ref('7');
const fullscreen = ref(false);
const metrics = ref({ kpis: {}, coverage: {}, events: [], trend: [], performance: [], actions: [] });
const labels = [['visits', '上门拜访'], ['captures', '新联系人'], ['owners', '新关联业主'],
  ['appraisals', '预约估价'], ['listings', '新增房源'], ['ofi', '开放参观登记']];
const cards = computed(() => labels.map(([key, label]) => ({ key, label,
  value: metrics.value.kpis?.[key]?.value || 0, previous: metrics.value.kpis?.[key]?.previous || 0 })));
const ranked = computed(() => [...(metrics.value.performance || [])].sort((a, b) => b.visits - a.visits));
const coverage = computed(() => Math.round(1000 * (metrics.value.coverage?.linked || 0) /
  Math.max(1, metrics.value.coverage?.total || 0)) / 10);
const text = { color: '#ccd6de' };
const gaugeOption = computed(() => ({
  series: [{ type: 'gauge', startAngle: 210, endAngle: -30, min: 0, max: 100,
    progress: { show: true, width: 12, itemStyle: { color: '#c9a227' } },
    axisLine: { lineStyle: { width: 12, color: [[1, '#34495c']] } },
    axisTick: { show: false }, splitLine: { show: false }, axisLabel: { show: false },
    detail: { color: '#fff', formatter: '{value}%' }, data: [{ value: coverage.value }] }],
}));
const funnelOption = computed(() => ({
  series: [{ type: 'funnel', sort: 'none', left: 15, right: 60, minSize: '22%',
    label: { color: '#fff', formatter: '{b}: {c}' },
    data: cards.value.slice(0, 5).map(item => ({ name: t(item.label), value: item.value })) }],
}));
const actionNames = { DK: '上门拜访', 'To-do done': '待办完成',
  'To-do reassigned': '任务改派', OFI: '开放参观' };
const trendOption = computed(() => ({
  tooltip: { trigger: 'axis' }, xAxis: { type: 'category', data: metrics.value.trend.map(item => item.day),
    axisLabel: text }, yAxis: { type: 'value', minInterval: 1, axisLabel: text },
  legend: { type: 'scroll', top: 0, textStyle: text }, grid: { top: 42, bottom: 32 },
  series: (metrics.value.actions || []).map(action => ({ name: t(actionNames[action] || action), type: 'line', smooth: true,
    data: metrics.value.trend.map(item => item.actions?.[action] || 0) })), 
}));
async function load() {
  try { metrics.value = await api(`admin/metrics?period=${period.value}`); }
  catch (error) { ElMessage.error(error.message); }
}
watch(period, load);
let timer;
onMounted(() => { load(); timer = setInterval(load, 60000); });
onUnmounted(() => { clearInterval(timer); if (document.fullscreenElement) document.exitFullscreen(); });
function toggle() {
  if (document.fullscreenElement) document.exitFullscreen();
  else document.querySelector('.cockpit').requestFullscreen();
  fullscreen.value = !fullscreen.value;
}
</script>

<style scoped>
.cockpit { min-height: 100%; padding: 24px; background: #0b1a2b; color: #f0f3f6; overflow: auto; }
.heading, .controls { display: flex; justify-content: space-between; align-items: center; gap: 14px; }
.heading h1 { margin: 0 0 5px; font-size: 26px; }
.heading span, .kpi span, .kpi small { color: #a7b8c9; }
.kpis { display: grid; grid-template-columns: repeat(6, 1fr); gap: 12px; margin: 20px 0; }
.panel { background: #142b42; border: 1px solid #2b455d; border-radius: 10px; padding: 16px; min-width: 0; }
.kpi { display: flex; flex-direction: column; gap: 8px; }
.kpi b { font-size: 30px; color: #fff; }
.grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; }
.grid h2 { font-size: 16px; margin: 0 0 12px; }
.chart { height: 300px; }
.gauge p { text-align: center; color: #a7b8c9; }
.map-panel { grid-column: span 2; }
.map-wrap { height: 400px; overflow: hidden; border-radius: 6px; }
.map-wrap :deep(.mv) { height: 400px; }
.map-wrap :deep(.mv > .panel) { display: none; }
.map-wrap :deep(.leaflet-tile-pane) { filter: brightness(.55) saturate(.7) hue-rotate(180deg); }
.map-wrap :deep(.legend) { color: #263747; }
.feed { height: 305px; overflow: auto; }
.event { border-bottom: 1px solid #2b455d; padding: 9px 0; font-size: 13px; }
@media (max-width: 1100px) { .kpis { grid-template-columns: repeat(3, 1fr); } }
</style>
