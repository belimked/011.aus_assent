<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h1>{{ t("房产列表") }}</h1>
        <div class="sub">{{ t("与地图共用数据 · 第三方字段不可导出") }}</div>
      </div>
      <div class="sp" />
      <el-radio-group v-model="mode">
        <el-radio-button value="table"
          ><el-icon><List /></el-icon> {{ t("表格") }}</el-radio-button
        >
        <el-radio-button value="card"
          ><el-icon><Grid /></el-icon> {{ t("卡片") }}</el-radio-button
        >
      </el-radio-group>
    </div>
    <div class="card bar">
      <el-input
        v-model="filters.q"
        :placeholder="t('地址或地块编号')"
        :prefix-icon="Search"
        clearable
        class="search"
        @keyup.enter="load(true)"
        @clear="load(true)"
      />
      <label class="filter-label">
        {{ t("区域") }}
        <el-select v-model="filters.suburb" class="filter" @change="load(true)">
          <el-option :label="t('全部')" value="all" />
          <el-option
            v-for="suburb in suburbs"
            :key="suburb"
            :label="suburb"
            :value="suburb"
          />
        </el-select>
      </label>
      <label class="filter-label">
        {{ t("登记状态") }}
        <el-select
          v-model="filters.registered"
          class="filter"
          @change="load(true)"
        >
          <el-option :label="t('全部')" value="all" />
          <el-option :label="t('已登记')" value="true" />
          <el-option :label="t('未登记')" value="false" />
        </el-select>
      </label>
      <label class="filter-label">
        {{ t("产权信息") }}
        <el-select
          v-model="filters.hasOwner"
          class="filter"
          @change="load(true)"
        >
          <el-option :label="t('全部')" value="all" />
          <el-option :label="t('有产权信息')" value="true" />
          <el-option :label="t('无产权信息')" value="false" />
        </el-select>
      </label>
      <label class="filter-label">
        {{ t("持有年限") }}
        <el-select
          v-model="filters.minYearsHeld"
          class="filter"
          @change="load(true)"
        >
          <el-option :label="t('全部')" value="all" />
          <el-option
            v-for="years in [5, 10, 15, 20]"
            :key="years"
            :label="t('持有 ≥ {n} 年', { n: years })"
            :value="years"
          />
        </el-select>
      </label>
      <div class="grow" />
      <el-button :icon="Search" @click="load(true)">{{ t("筛选") }}</el-button>
    </div>
    <div v-if="mode === 'table'" class="card">
      <el-table
        :data="items"
        row-class-name="clickable"
        height="calc(100vh - 310px)"
        @row-click="selected = $event"
      >
        <el-table-column :label="t('地址')" min-width="195">
          <template #default="{ row }">
            <div class="row">
              <span class="dot" :style="{ background: color(row) }" />
              <div>
                <b>{{ row.address }}</b>
                <div class="small muted">{{ row.suburb }}</div>
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="lotplan" :label="t('地块')" min-width="120" />
        <el-table-column :label="t('状态')" width="122">
          <template #default="{ row }"
            ><StatusPill :status="parcelStatus(row)" short
          /></template>
        </el-table-column>
        <el-table-column
          prop="ownerNames"
          :label="t('第三方业主')"
          min-width="150"
          show-overflow-tooltip
        />
        <el-table-column :label="t('最近成交')" width="140">
          <template #default="{ row }">{{
            fmtDate(row.lastSaleDate)
          }}</template>
        </el-table-column>
        <el-table-column :label="t('上次成交价')" width="125" align="right">
          <template #default="{ row }">{{ money(row.lastSalePrice) }}</template>
        </el-table-column>
        <el-table-column :label="t('持有')" width="105">
          <template #default="{ row }">{{
            row.yearsHeld == null ? "—" : t("{n} 年", { n: row.yearsHeld })
          }}</template>
        </el-table-column>
        <el-table-column :label="t('异地业主')" width="100">
          <template #default="{ row }"
            ><StatusPill :status="row.absenteeOwner ? 'yes' : 'no'" short
          /></template>
        </el-table-column>
        <el-table-column :label="t('覆盖经纪人')" min-width="135">
          <template #default="{ row }">{{
            row.coveringAgents?.map((agent) => agent.name).join("、") || "—"
          }}</template>
        </el-table-column>
      </el-table>
    </div>
    <div v-else class="cards">
      <div
        v-for="parcel in items"
        :key="parcel.id"
        class="pc"
        @click="selected = parcel"
      >
        <div class="img">
          <StatusPill :status="parcelStatus(parcel)" class="pill" />
          <div class="lot serif">{{ parcel.lotplan }}</div>
        </div>
        <div class="pb">
          <div class="serif address">{{ parcel.address }}</div>
          <div class="small muted">{{ parcel.suburb }} QLD</div>
          <div class="foot small">
            {{ parcel.ownerNames || t("尚无产权信息") }} ·
            {{ fmtDate(parcel.lastSaleDate) }}
          </div>
        </div>
      </div>
    </div>
    <div class="pg">
      <span class="small muted">{{
        t("第 {n} 页 · 本页 {total} 处", { n: pageNumber, total: items.length })
      }}</span>
      <div class="row">
        <el-button :disabled="!cursors.length" @click="previous">{{
          t("上一页")
        }}</el-button>
        <el-button :disabled="items.length < 50" @click="next">{{
          t("下一页")
        }}</el-button>
      </div>
    </div>
    <AdminParcelDrawer :parcel="selected" @close="selected = null" />
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { Search } from "@element-plus/icons-vue";
import { ElMessage } from "element-plus";
import { api } from "../api";
import { fmtDate, money, parcelStatus } from "../admin-format";
import { t } from "../i18n";
import AdminParcelDrawer from "../components/AdminParcelDrawer.vue";
import StatusPill from "../components/StatusPill.vue";

const route = useRoute();
const mode = ref("table");
const items = ref([]);
const stats = ref([]);
const selected = ref(null);
const cursors = ref([]);
const cursor = ref("");
const filters = reactive({
  q: "",
  suburb: "all",
  registered: "all",
  hasOwner: "all",
  minYearsHeld: "all",
});
const suburbs = computed(() =>
  stats.value.map((row) => row.suburb).filter(Boolean),
);
const pageNumber = computed(() => cursors.value.length + 1);
const color = (row) =>
  ({ owned: "#16a34a", registered: "#c9a227", unregistered: "#9ca3af" })[
    parcelStatus(row)
  ];

async function load(reset = false) {
  if (reset) {
    cursor.value = "";
    cursors.value = [];
  }
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(filters)) {
    if (value !== "" && value !== "all") query.set(key, value);
  }
  if (cursor.value) query.set("cursor", cursor.value);
  try {
    items.value = await api(`admin/properties?${query}`);
  } catch (error) {
    ElMessage.error(error.message);
  }
}
function next() {
  cursors.value.push(cursor.value);
  cursor.value = items.value.at(-1).id;
  load();
}
function previous() {
  cursor.value = cursors.value.pop();
  load();
}
watch(
  () => route.query.q,
  (value) => {
    filters.q = value || "";
    load(true);
  },
);
onMounted(async () => {
  filters.q = route.query.q || "";
  stats.value = await api("admin/parcel-stats");
  load(true);
});
</script>

<style scoped>
.bar {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  align-items: end;
  padding: 12px 14px;
  margin-bottom: 12px;
}
.search {
  width: 220px;
}
.filter-label {
  display: grid;
  gap: 4px;
  color: var(--mut);
  font-size: 12px;
}
.filter {
  width: 138px;
}
:deep(.clickable) {
  cursor: pointer;
}
.pg {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
}
.cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 16px;
}
.pc {
  background: #fff;
  border-radius: 14px;
  overflow: hidden;
  border: 1px solid var(--line);
  cursor: pointer;
}
.pc:hover {
  border-color: var(--gold);
  box-shadow: 0 8px 20px rgba(15, 42, 68, 0.08);
}
.img {
  height: 130px;
  background: linear-gradient(135deg, #517392, var(--navy));
  position: relative;
}
.pill {
  position: absolute;
  top: 12px;
  left: 12px;
}
.lot {
  position: absolute;
  bottom: 10px;
  left: 14px;
  font-size: 20px;
  color: #fff;
}
.pb {
  padding: 12px 14px;
}
.address {
  font-size: 18px;
}
.foot {
  margin-top: 10px;
  padding-top: 8px;
  border-top: 1px solid var(--line);
  color: var(--mut);
}
</style>
