<template>
  <div class="page">
    <div class="page-head">
      <div><h1>{{ t('工作台') }}</h1><div class="sub">{{ t('外勤采集概览') }}</div></div>
      <div class="sp" />
      <label>{{ t('周期') }}
        <el-select v-model="period" style="width: 115px">
          <el-option v-for="item in periods" :key="item.id" :value="item.id" :label="t(item.label)" />
        </el-select>
      </label>
      <label v-if="identity?.role === 'admin'">{{ t('团队') }}
        <el-select v-model="team" style="width: 145px">
          <el-option value="" :label="t('全部')" />
          <el-option v-for="item in teams" :key="item.id" :value="item.id" :label="item.name" />
        </el-select>
      </label>
      <label>{{ t('经纪人') }}
        <el-select v-model="agent" style="width: 145px">
          <el-option value="" :label="t('全部')" />
          <el-option v-for="item in agents" :key="item.id" :value="item.id" :label="item.name" />
        </el-select>
      </label>
    </div>
    <div class="kpis">
      <KpiCard v-for="item in cards" :key="item.key" :label="t(item.label)" :value="item.value"
        :d="item.delta" :spark="trend.map(day => day.visits)" :icon="item.icon" :color="item.color"
        :foot="t('较上期 {n}', { n: item.previous })" />
    </div>
    <div class="panels">
      <div class="card">
        <div class="card-h"><h3>{{ t('每日活动趋势') }}</h3></div>
        <div class="card-b"><v-chart class="chart" :option="chartOption" autoresize /></div>
      </div>
      <div class="card">
        <div class="card-h"><h3>{{ t('外勤动态') }}</h3><div class="sp" />{{ t('每分钟更新') }}</div>
        <div class="card-b">
          <div v-for="event in events" :key="event.id" class="event">
            <Avatar :name="event.agent" :size="32" />
            <div><b>{{ event.agent }}</b> · {{ event.detail }}<div class="small muted">{{ relTime(event.at) }}</div></div>
          </div>
          <el-empty v-if="!events.length" :description="t('暂无活动')" />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue';
import VChart from 'vue-echarts';
import { api, identity } from '../api';
import { relTime } from '../admin-format';
import { t } from '../i18n';
import KpiCard from '../components/KpiCard.vue';
import Avatar from '../components/Avatar.vue';

const periods = [{ id: 'today', label: '今天' }, { id: '7', label: '7 天' },
  { id: '30', label: '30 天' }, { id: '90', label: '90 天' }];
const period = ref('7');
const team = ref('');
const agent = ref('');
const teams = ref([]);
const agents = ref([]);
const metrics = ref({ kpis: {}, trend: [], events: [] });
const trend = computed(() => metrics.value.trend || []);
const events = computed(() => metrics.value.events || []);
const definitions = [
  ['visits', '上门拜访', 'House', '#0e7490'],
  ['captures', '新联系人', 'User', '#2563eb'],
  ['appraisals', '预约估价', 'Calendar', '#b8860b'],
  ['ofi', '开放参观登记', 'Key', '#16a34a'],
];
const cards = computed(() => definitions.map(([key, label, icon, color]) => {
  const value = metrics.value.kpis?.[key] || { value: 0, previous: 0 };
  return { key, label, icon, color, ...value, delta: value.value - value.previous };
}));
const chartOption = computed(() => ({
  tooltip: { trigger: 'axis' },
  xAxis: { type: 'category', data: trend.value.map(day => day.day) },
  yAxis: { type: 'value', minInterval: 1 },
  series: [{ type: 'line', data: trend.value.map(day => day.visits), smooth: true,
    areaStyle: { opacity: 0.08 }, lineStyle: { color: '#0e7490' } }],
}));
async function load() {
  try {
    metrics.value = await api(`admin/metrics?${new URLSearchParams({ period: period.value,
      team: team.value, agent: agent.value })}`);
  } catch (error) { console.error(error); }
}
watch([period, team, agent], load);
let timer;
onMounted(async () => {
  try {
    teams.value = await api('admin/teams');
    if (identity.value?.role === 'admin') agents.value = await api('admin/users');
  } catch (error) { console.error(error); }
  await load();
  timer = setInterval(load, 60000);
});
onUnmounted(() => clearInterval(timer));
</script>

<style scoped>
.page-head { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.page-head label { display: flex; align-items: center; gap: 6px; font-size: 13px; }
.panels { display: grid; grid-template-columns: 3fr 2fr; gap: 16px; margin-top: 16px; }
.chart { height: 310px; }
.event { display: flex; align-items: center; gap: 12px; padding: 9px 0; border-bottom: 1px solid var(--line); }
@media (max-width: 950px) { .panels { grid-template-columns: 1fr; } }
</style>
