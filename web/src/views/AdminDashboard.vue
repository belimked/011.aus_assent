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
        <el-select v-model="team" :empty-values="[null, undefined]" style="width: 145px">
          <el-option value="" :label="t('全部')" />
          <el-option v-for="item in teams" :key="item.id" :value="item.id" :label="item.name" />
        </el-select>
      </label>
      <label>{{ t('经纪人') }}
        <el-select v-model="agent" :empty-values="[null, undefined]" style="width: 145px">
          <el-option value="" :label="t('全部')" />
          <el-option v-for="item in agents" :key="item.id" :value="item.id" :label="item.name" />
        </el-select>
      </label>
    </div>
    <div class="kpis">
      <KpiCard v-for="item in cards" :key="item.key" :label="t(item.label)" :value="item.value"
        :d="item.delta" :spark="item.spark" :icon="item.icon" :color="item.color"
        :foot="t('较上期 {n}', { n: item.previous })" />
      <KpiCard :label="t('区域覆盖率')" :value="coveragePercent" unit="%" icon="PieChart"
        color="#16a34a" :foot="t('已关联 {n} / {total} 处房产', {
          n: metrics.coverage?.linked || 0, total: metrics.coverage?.total || 0,
        })" to="/map" />
    </div>
    <div class="panels">
      <div class="card">
        <div class="card-h"><h3>{{ t('采集转化漏斗') }}</h3></div>
        <div class="card-b funnel">
          <div v-for="stage in funnel" :key="stage.key" class="stage">
            <span>{{ t(stage.label) }}</span>
            <div class="track"><i :style="{ width: stage.width + '%' }" /></div>
            <b>{{ stage.value }}</b>
          </div>
          <p class="small muted">{{ t('各阶段独立计数，非同一批房产的转化率') }}</p>
        </div>
      </div>
      <div class="card">
        <div class="card-h"><h3>{{ t('区域覆盖率') }}</h3></div>
        <div class="card-b coverage">
          <b>{{ coveragePercent }}%</b>
          <p>{{ t('已关联 {n} / {total} 处房产', {
            n: metrics.coverage?.linked || 0, total: metrics.coverage?.total || 0,
          }) }}</p>
          <el-progress :percentage="coveragePercent" :show-text="false" color="#16a34a" />
        </div>
      </div>
      <div class="card">
        <div class="card-h"><h3>{{ t('今日日程') }}</h3><div class="sp" />
          <router-link to="/tasks">{{ t('全部 →') }}</router-link></div>
        <div class="card-b">
          <router-link v-for="item in metrics.agenda || []" :key="item.id" class="agenda" to="/tasks">
            <b>{{ item.due }}</b><span>{{ item.text }}</span><small>{{ item.agent }}</small>
          </router-link>
          <el-empty v-if="!metrics.agenda?.length" :description="t('暂无待办')" />
        </div>
      </div>
      <div v-if="identity?.role === 'admin'" class="card">
        <div class="card-h"><h3>{{ t('潜在卖家机会') }}</h3></div>
        <div class="card-b">
          <router-link v-for="item in metrics.opportunities || []" :key="item.id"
            class="agenda" :to="`/map?parcel=${encodeURIComponent(item.id)}`">
            <b>{{ item.score }}</b><span>{{ item.address }} · {{ item.suburb }}</span>
          </router-link>
          <el-empty v-if="!metrics.opportunities?.length" :description="t('暂无机会')" />
        </div>
      </div>
      <div class="card performance">
        <div class="card-h"><h3>{{ t('经纪人表现') }}</h3><div class="sp" />
          <label>{{ t('排序') }}
            <el-select v-model="sortBy" style="width: 130px">
              <el-option v-for="field in performanceFields" :key="field.key" :value="field.key"
                :label="t(field.label)" />
            </el-select>
          </label>
        </div>
        <el-table :data="ranked" max-height="360">
          <el-table-column prop="name" :label="t('经纪人')" min-width="140" />
          <el-table-column v-for="field in performanceFields" :key="field.key" :prop="field.key"
            :label="t(field.label)" align="right" min-width="100" sortable />
        </el-table>
      </div>
      <div class="card">
        <div class="card-h"><h3>{{ t('最佳上门时段') }}</h3></div>
        <div class="card-b">
          <v-chart v-if="metrics.visitCount >= 20" class="chart" :option="heatOption" autoresize />
          <el-empty v-else :description="t('数据不足')" />
        </div>
      </div>
      <div class="card">
        <div class="card-h"><h3>{{ t('每日活动趋势') }}</h3></div>
        <div class="card-b">
          <v-chart v-if="metrics.actions?.length" class="chart" :option="chartOption" autoresize />
          <el-empty v-else :description="t('暂无活动')" />
        </div>
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
import VChart from '../components/chart';
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
const metrics = ref({ kpis: {}, trend: [], events: [], actions: [] });
const trend = computed(() => metrics.value.trend || []);
const events = computed(() => metrics.value.events || []);
const sortBy = ref('visits');
const performanceFields = [
  { key: 'visits', label: '上门拜访' }, { key: 'contacts', label: '新联系人' },
  { key: 'appraisals', label: '预约估价' }, { key: 'linked', label: '新关联业主' },
  { key: 'listings', label: '新增房源' }, { key: 'done', label: '已完成待办' },
];
const ranked = computed(() => [...(metrics.value.performance || [])]
  .sort((a, b) => b[sortBy.value] - a[sortBy.value]));
const funnel = computed(() => {
  const stages = [
    ['visits', '上门拜访'], ['captures', '新联系人'], ['owners', '新关联业主'],
    ['appraisals', '预约估价'], ['listings', '新增房源'],
  ].map(([key, label]) => ({ key, label, value: metrics.value.kpis?.[key]?.value || 0 }));
  const max = Math.max(1, ...stages.map(stage => stage.value));
  return stages.map(stage => ({ ...stage, width: Math.round(stage.value / max * 100) }));
});
const coveragePercent = computed(() => Math.round(1000 * (metrics.value.coverage?.linked || 0) /
  Math.max(1, metrics.value.coverage?.total || 0)) / 10);
const heatOption = computed(() => ({
  tooltip: { position: 'top' },
  grid: { left: 42, right: 16, top: 10, bottom: 35 },
  xAxis: { type: 'category', data: Array.from({ length: 24 }, (_, h) => `${h}:00`), splitArea: { show: true } },
  yAxis: { type: 'category', data: ['周日', '周一', '周二', '周三', '周四', '周五', '周六'].map(t),
    splitArea: { show: true } },
  visualMap: { min: 0, max: Math.max(1, ...(metrics.value.heatmap || []).map(point => point[2])),
    calculable: true, orient: 'horizontal', left: 'center', bottom: -6 },
  series: [{ type: 'heatmap', data: metrics.value.heatmap || [] }],
}));
const definitions = [
  ['visits', '上门拜访', 'House', '#0e7490'],
  ['captures', '新联系人', 'User', '#2563eb'],
  ['appraisals', '预约估价', 'Calendar', '#b8860b'],
  ['ofi', '开放参观登记', 'Key', '#16a34a'],
  ['owners', '新关联业主', 'Link', '#9a7023'],
  ['listings', '新增房源', 'OfficeBuilding', '#48627c'],
];
const cards = computed(() => definitions.map(([key, label, icon, color]) => {
  const value = metrics.value.kpis?.[key] || { value: 0, previous: 0 };
  return { key, label, icon, color, ...value,
    delta: value.previous ? Math.round(100 * (value.value - value.previous) / value.previous) : null }; 
}));
const actionNames = { DK: '上门拜访', 'To-do done': '待办完成',
  'To-do reassigned': '任务改派', OFI: '开放参观' };
const chartOption = computed(() => ({
  tooltip: { trigger: 'axis' },
  xAxis: { type: 'category', data: trend.value.map(day => day.day) },
  yAxis: { type: 'value', minInterval: 1 },
  legend: { type: 'scroll', top: 0 }, grid: { top: 42, bottom: 32 },
  series: (metrics.value.actions || []).map(action => ({ name: t(actionNames[action] || action), type: 'line',
    data: trend.value.map(day => day.actions?.[action] || 0), smooth: true,
    areaStyle: { opacity: 0.06 } })), 
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
    agents.value = await api('admin/users');
  } catch (error) { console.error(error); }
  await load();
  timer = setInterval(load, 60000);
});
onUnmounted(() => clearInterval(timer));
</script>

<style scoped>
.page-head { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.kpis { display: grid; grid-template-columns: repeat(auto-fit, minmax(185px, 1fr)); gap: 12px; margin: 16px 0; }
.page-head label { display: flex; align-items: center; gap: 6px; font-size: 13px; }
.panels { display: grid; grid-template-columns: 3fr 2fr; gap: 16px; margin-top: 16px; }
.chart { height: 310px; }
.funnel { display: grid; gap: 12px; }
.stage { display: grid; grid-template-columns: 100px 1fr 35px; gap: 12px; align-items: center; }
.track { height: 20px; background: #ecf1f4; border-radius: 4px; }
.track i { display: block; height: 100%; background: #0e7490; border-radius: 4px; }
.coverage b { font-size: 44px; color: #16a34a; }
.agenda { display: flex; gap: 12px; padding: 10px 0; border-bottom: 1px solid var(--line);
  text-decoration: none; color: inherit; }
.agenda b { min-width: 55px; }
.agenda span { flex: 1; }
.performance { grid-column: span 1; }
.event { display: flex; align-items: center; gap: 12px; padding: 9px 0; border-bottom: 1px solid var(--line); }
@media (max-width: 950px) { .panels { grid-template-columns: 1fr; } }
</style>
