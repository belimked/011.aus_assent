<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h1>{{ t("工作台") }}</h1>
        <div class="sub">{{ t("账户、审核、房产与市场数据概览") }}</div>
      </div>
    </div>
    <div class="kpis">
      <KpiCard
        v-for="metric in primary"
        :key="metric.label"
        :label="t(metric.label)"
        :value="metric.value"
        :icon="metric.icon"
        :color="metric.color"
        :foot="t(metric.foot)"
        :to="metric.to"
      />
    </div>
    <div class="sections">
      <div class="card">
        <div class="card-h">
          <h3>{{ t("各区地块覆盖") }}</h3>
        </div>
        <div class="card-b">
          <div v-for="suburb in stats" :key="suburb.suburb" class="suburb">
            <div class="row">
              <b class="grow">{{ suburb.suburb }}</b>
              <span class="serif pct">{{ suburb.total }}</span>
            </div>
            <div class="stack">
              <i
                :style="{
                  width: percent(suburb.withOwnership, suburb.total),
                  background: '#16a34a',
                }"
              />
              <i
                :style="{
                  width: percent(
                    suburb.registered - suburb.withOwnership,
                    suburb.total,
                  ),
                  background: '#c9a227',
                }"
              />
              <i
                :style="{
                  width: percent(
                    suburb.total - suburb.registered,
                    suburb.total,
                  ),
                  background: '#9ca3af',
                }"
              />
            </div>
            <div class="small muted">
              {{
                t("地块 {total} · 已登记 {registered} · 有产权 {owned}", {
                  total: suburb.total,
                  registered: suburb.registered,
                  owned: suburb.withOwnership,
                })
              }}
            </div>
          </div>
        </div>
      </div>
      <div class="card">
        <div class="card-h">
          <h3>{{ t("最近市场导入") }}</h3>
          <div class="sp" />
          <router-link to="/market">{{ t("全部 →") }}</router-link>
        </div>
        <el-table :data="imports.slice(0, 6)">
          <el-table-column :label="t('日期')" width="130">
            <template #default="{ row }">{{ fmtDate(row.createdAt) }}</template>
          </el-table-column>
          <el-table-column
            prop="filename"
            :label="t('文件')"
            show-overflow-tooltip
          />
          <el-table-column
            prop="success"
            :label="t('成功')"
            width="75"
            align="right"
          />
        </el-table>
      </div>
    </div>
    <div class="card details">
      <div class="card-h">
        <h3>{{ t("记录与 Rex 投递") }}</h3>
      </div>
      <div class="card-b detail-grid">
        <div v-for="item in secondary" :key="item.key">
          <div class="small muted">{{ t(item.label) }}</div>
          <b class="serif count">{{ item.value }}</b>
        </div>
        <div v-for="(count, status) in overview?.rex || {}" :key="status">
          <div class="small muted">{{ t(rexLabels[status] || status) }}</div>
          <b class="serif count">{{ count }}</b>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from "vue";
import { api } from "../api";
import { fmtDate } from "../admin-format";
import { t } from "../i18n";
import KpiCard from "../components/KpiCard.vue";

const overview = ref(null);
const stats = ref([]);
const imports = ref([]);
const rexLabels = {
  waiting_auth: "等待授权",
  pending: "待投递",
  sent: "已发送",
  failed: "失败",
  skipped: "已跳过",
};
const primary = computed(() => {
  const data = overview.value;
  if (!data) return [];
  return [
    {
      label: "房产",
      value: data.counts.properties,
      icon: "OfficeBuilding",
      color: "#0f2a44",
      foot: "所有地块",
      to: "/properties",
    },
    {
      label: "已登记地块",
      value: stats.value.reduce((n, s) => n + s.registered, 0),
      icon: "MapLocation",
      color: "#16a34a",
      foot: "已登记房产",
      to: "/map",
    },
    {
      label: "账户",
      value: data.users,
      icon: "UserFilled",
      color: "#2563eb",
      foot: "团队账户",
      to: "/team",
    },
    {
      label: "待审核",
      value: data.pendingApprovals,
      icon: "Stamp",
      color: "#b8860b",
      foot: "重新分配与合并",
      to: "/review",
    },
    {
      label: "24 小时写入",
      value: data.writes24h,
      icon: "EditPen",
      color: "#0e7490",
      foot: "审计日志记录",
      to: "/audit",
    },
  ];
});
const extraKinds = [
  ["contacts", "联系人"],
  ["captures", "采集记录"],
  ["enquiries", "咨询"],
  ["leads", "线索"],
  ["listings", "房源"],
  ["sales", "成交"],
  ["ofi_attendees", "开放参观登记"],
  ["market_items", "市场记录"],
];
const secondary = computed(() =>
  extraKinds.map(([key, label]) => ({
    key,
    label,
    value: overview.value?.counts?.[key] || 0,
  })),
);
const percent = (part, total) =>
  `${(Math.max(0, part) / Math.max(1, total)) * 100}%`;

onMounted(async () => {
  [overview.value, stats.value, imports.value] = await Promise.all([
    api("admin/overview"),
    api("admin/parcel-stats"),
    api("admin/market/imports"),
  ]);
});
</script>

<style scoped>
.kpis {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 14px;
  margin-bottom: 16px;
}
.sections {
  display: grid;
  grid-template-columns: 1fr 1.2fr;
  gap: 16px;
}
.suburb {
  padding: 10px 0;
  border-bottom: 1px solid var(--line);
}
.suburb:last-child {
  border-bottom: 0;
}
.pct {
  font-size: 20px;
}
.stack {
  display: flex;
  height: 7px;
  overflow: hidden;
  border-radius: 8px;
  background: #e7e9ee;
  margin: 8px 0;
}
.stack i {
  display: block;
}
.details {
  margin-top: 16px;
}
.detail-grid {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 16px;
}
.count {
  display: block;
  font-size: 23px;
  margin-top: 4px;
}
</style>
